// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// _________                        .__  __
// \_   ___ \  ____   _____   _____ |__|/  |_
// /    \  \/ /  _ \ /     \ /     \|  \   __\
// \     \___(  <_> )  Y Y  \  Y Y  \  ||  |
//  \______  /\____/|__|_|  /__|_|  /__||__|
//         \/             \/      \/
// ____   ____           .__  _____.__               __  .__
// \   \ /   /___________|__|/ ____\__| ____ _____ _/  |_|__| ____   ____
//  \   Y   // __ \_  __ \  \   __\|  |/ ___\\__  \\   __\  |/  _ \ /    \
//   \     /\  ___/|  | \/  ||  |  |  \  \___ / __ \|  | |  (  <_> )   |  \
//    \___/  \___  >__|  |__||__|  |__|\___  >____  /__| |__|\____/|___|  /
//               \/                        \/     \/                    \/
//
// This file contains commit verification functions for refs passed across in hooks

func verifyCommits(oldCommitID, newCommitID string, repo *git.Repository, env []string) error {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		log.Error("Unable to create os.Pipe for %s", repo.Path)
		return err
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	// This is safe as force pushes are already forbidden
	err = git.NewCommand("rev-list", oldCommitID+"..."+newCommitID).
		RunInDirTimeoutEnvFullPipelineFunc(env, -1, repo.Path,
			stdoutWriter, nil, nil,
			func(ctx context.Context, cancel context.CancelFunc) error {
				_ = stdoutWriter.Close()
				err := readAndVerifyCommitsFromShaReader(stdoutReader, repo, env)
				if err != nil {
					log.Error("%v", err)
					cancel()
				}
				_ = stdoutReader.Close()
				return err
			})
	if err != nil && !isErrUnverifiedCommit(err) {
		log.Error("Unable to check commits from %s to %s in %s: %v", oldCommitID, newCommitID, repo.Path, err)
	}
	return err
}

func readAndVerifyCommitsFromShaReader(input io.ReadCloser, repo *git.Repository, env []string) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		err := readAndVerifyCommit(line, repo, env)
		if err != nil {
			log.Error("%v", err)
			return err
		}
	}
	return scanner.Err()
}

func readAndVerifyCommit(sha string, repo *git.Repository, env []string) error {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		log.Error("Unable to create pipe for %s: %v", repo.Path, err)
		return err
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()
	hash := git.MustIDFromString(sha)

	return git.NewCommand("cat-file", "commit", sha).
		RunInDirTimeoutEnvFullPipelineFunc(env, -1, repo.Path,
			stdoutWriter, nil, nil,
			func(ctx context.Context, cancel context.CancelFunc) error {
				_ = stdoutWriter.Close()
				commit, err := git.CommitFromReader(repo, hash, stdoutReader)
				if err != nil {
					return err
				}
				verification := models.ParseCommitWithSignature(commit)
				if !verification.Verified {
					cancel()
					return &errUnverifiedCommit{
						commit.ID.String(),
					}
				}
				return nil
			})
}

type errUnverifiedCommit struct {
	sha string
}

func (e *errUnverifiedCommit) Error() string {
	return fmt.Sprintf("Unverified commit: %s", e.sha)
}

func isErrUnverifiedCommit(err error) bool {
	_, ok := err.(*errUnverifiedCommit)
	return ok
}
