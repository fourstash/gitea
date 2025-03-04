// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path/filepath"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	repo_service "code.gitea.io/gitea/services/repository"
)

// AdoptOrDeleteRepository adopts or deletes a repository
func AdoptOrDeleteRepository(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsRepos"] = true
	allowAdopt := ctx.IsUserSiteAdmin() || setting.Repository.AllowAdoptionOfUnadoptedRepositories
	ctx.Data["allowAdopt"] = allowAdopt
	allowDelete := ctx.IsUserSiteAdmin() || setting.Repository.AllowDeleteOfUnadoptedRepositories
	ctx.Data["allowDelete"] = allowDelete

	dir := ctx.FormString("id")
	action := ctx.FormString("action")

	ctxUser := ctx.User
	root := user_model.UserPath(ctxUser.LowerName)

	// check not a repo
	has, err := models.IsRepositoryExist(ctxUser, dir)
	if err != nil {
		ctx.ServerError("IsRepositoryExist", err)
		return
	}

	isDir, err := util.IsDir(filepath.Join(root, dir+".git"))
	if err != nil {
		ctx.ServerError("IsDir", err)
		return
	}
	if has || !isDir {
		// Fallthrough to failure mode
	} else if action == "adopt" && allowAdopt {
		if _, err := repo_service.AdoptRepository(ctxUser, ctxUser, models.CreateRepoOptions{
			Name:      dir,
			IsPrivate: true,
		}); err != nil {
			ctx.ServerError("repository.AdoptRepository", err)
			return
		}
		ctx.Flash.Success(ctx.Tr("repo.adopt_preexisting_success", dir))
	} else if action == "delete" && allowDelete {
		if err := repo_service.DeleteUnadoptedRepository(ctxUser, ctxUser, dir); err != nil {
			ctx.ServerError("repository.AdoptRepository", err)
			return
		}
		ctx.Flash.Success(ctx.Tr("repo.delete_preexisting_success", dir))
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings/repos")
}
