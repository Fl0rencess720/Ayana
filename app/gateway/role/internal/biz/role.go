package biz

import (
	roleV1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/role/v1"
	"github.com/go-kratos/kratos/v2/log"
)

// UserRepo is a User repo.
type RoleRepo interface {
}

// UserUsecase is a User usecase.
type RoleUsecase struct {
	repo       RoleRepo
	log        *log.Helper
	userClient roleV1.RoleManagerClient
}

// NewUserUsecase new a User usecase.
func NewRoleUsecase(repo RoleRepo, logger log.Logger, userClient roleV1.RoleManagerClient) *RoleUsecase {
	return &RoleUsecase{repo: repo, log: log.NewHelper(logger), userClient: userClient}
}
