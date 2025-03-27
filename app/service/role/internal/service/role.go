package service

import (
	"context"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/role/v1"
	"github.com/Fl0rencess720/Wittgenstein/app/service/role/internal/biz"
)

type RoleService struct {
	v1.UnimplementedRoleManagerServer

	uc *biz.RoleUsecase
}

func NewRoleService(uc *biz.RoleUsecase) *RoleService {
	return &RoleService{uc: uc}
}

func (s *RoleService) CreateRole(ctx context.Context, req *v1.CreateRoleRequest) (*v1.CreateRoleReply, error) {
	uid, err := s.uc.CreateRole(ctx, req.Phone, biz.Role{
		Uid:         req.Role.Uid,
		RoleName:    req.Role.Name,
		Description: req.Role.Description,
		Avatar:      req.Role.Avatar,
		ApiPath:     req.Role.ApiPath,
		ApiKey:      req.Role.ApiKey,
		ModelName:   req.Role.Model.Name,
		Provider:    req.Role.Model.Provider,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CreateRoleReply{
		Uid: uid,
	}, nil
}
func (s *RoleService) DeleteRole(ctx context.Context, req *v1.DeleteRoleRequest) (*v1.DeleteRoleReply, error) {
	if err := s.uc.DeleteRole(ctx, req.Phone, req.Uid); err != nil {
		return nil, err
	}
	return &v1.DeleteRoleReply{Message: "success"}, nil
}
func (s *RoleService) GetAvailableModels(ctx context.Context, req *v1.GetAvailableModelsRequest) (*v1.GetAvailableModelsReply, error) {

	return nil, nil
}
func (s *RoleService) GetRoles(ctx context.Context, req *v1.GetRolesRequest) (*v1.GetRolesReply, error) {
	roles, err := s.uc.GetRoles(ctx, req.Phone)
	if err != nil {
		return nil, err
	}
	reply := &v1.GetRolesReply{}
	for _, role := range roles {
		reply.Roles = append(reply.Roles, &v1.Role{
			Uid:         role.Uid,
			Name:        role.RoleName,
			Description: role.Description,
			ApiPath:     role.ApiPath,
			ApiKey:      role.ApiKey,
			Model: &v1.Model{
				Name:     role.ModelName,
				Provider: role.Provider,
			},
		})
	}
	return reply, nil
}
