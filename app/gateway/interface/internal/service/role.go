package service

import (
	"context"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/role/v1"
	"github.com/Fl0rencess720/Wittgenstein/app/gateway/interface/internal/biz"
	"google.golang.org/grpc"
)

type RoleService struct {
	v1.UnimplementedRoleManagerServer

	uc *biz.RoleUsecase
}

func NewUserService(uc *biz.RoleUsecase) *RoleService {
	return &RoleService{uc: uc}
}

func (s *RoleService) CallRole(req *v1.CallRoleRequest, sr grpc.ServerStreamingServer[v1.CallRoleReply]) error {
	return nil
}
func (s *RoleService) CreateRole(ctx context.Context, req *v1.CreateRoleRequest) (*v1.CreateRoleReply, error) {
	return nil, nil
}
func (s *RoleService) DeleteRolec(ctx context.Context, req *v1.DeleteRoleRequest) (*v1.DeleteRoleReply, error) {
	return nil, nil
}
func (s *RoleService) GetAvailableModels(ctx context.Context, req *v1.GetAvailableModelsRequest) (*v1.GetAvailableModelsReply, error) {
	return nil, nil
}
func (s *RoleService) GetRoles(ctx context.Context, req *v1.GetRolesRequest) (*v1.GetRolesReply, error) {
	return nil, nil
}
