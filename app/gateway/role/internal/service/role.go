package service

import (
	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/role/v1"
	"github.com/Fl0rencess720/Wittgenstein/app/gateway/role/internal/biz"
)

type RoleService struct {
	v1.UnimplementedRoleManagerServer

	uc *biz.RoleUsecase
}

func NewUserService(uc *biz.RoleUsecase) *RoleService {
	return &RoleService{uc: uc}
}
