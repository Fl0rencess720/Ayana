package service

import (
	"context"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/role/v1"
	"github.com/Fl0rencess720/Wittgenstein/app/gateway/interface/internal/biz"
	"github.com/Fl0rencess720/Wittgenstein/pkgs/utils"
)

type RoleService struct {
	v1.UnimplementedRoleManagerServer

	uc *biz.RoleUsecase
}

func NewRoleService(uc *biz.RoleUsecase) *RoleService {
	return &RoleService{uc: uc}
}

func (s *RoleService) CreateRole(ctx context.Context, req *v1.CreateRoleRequest) (*v1.CreateRoleReply, error) {
	reply, err := s.uc.CreateRole(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func (s *RoleService) DeleteRole(ctx context.Context, req *v1.DeleteRoleRequest) (*v1.DeleteRoleReply, error) {
	reply, err := s.uc.DeleteRole(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func (s *RoleService) GetAvailableModels(ctx context.Context, req *v1.GetAvailableModelsRequest) (*v1.GetAvailableModelsReply, error) {
	reply, err := s.uc.GetAvailableModels(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func (s *RoleService) GetRoles(ctx context.Context, req *v1.GetRolesRequest) (*v1.GetRolesReply, error) {
	reply, err := s.uc.GetRoles(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func (s *RoleService) SetRole(ctx context.Context, req *v1.SetRoleRequest) (*v1.SetRoleReply, error) {
	req.Phone = utils.GetPhoneFromContext(ctx)
	reply, err := s.uc.SetRole(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
