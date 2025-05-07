package service

import (
	"context"

	v1 "github.com/Fl0rencess720/Ayana/api/gateway/role/v1"
	"github.com/Fl0rencess720/Ayana/app/service/role/internal/biz"
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

func (s *RoleService) GetRolesAndModeratorByUIDs(ctx context.Context, req *v1.GetRolesAndModeratorByUIDsRequest) (*v1.GetRolesAndModeratorByUIDsReply, error) {
	moderator, roles, err := s.uc.GetRolesAndModeratorByUIDs(ctx, req.Phone, req.Moderator, req.Uids)
	if err != nil {
		return nil, err
	}
	reply := &v1.GetRolesAndModeratorByUIDsReply{}
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
	reply.Moderator = &v1.Role{
		Uid:         moderator.Uid,
		Name:        moderator.RoleName,
		Description: moderator.Description,
		ApiPath:     moderator.ApiPath,
		ApiKey:      moderator.ApiKey,
		Model: &v1.Model{
			Name:     moderator.ModelName,
			Provider: moderator.Provider,
		},
	}
	return reply, nil
}
func (s *RoleService) SetRole(ctx context.Context, req *v1.SetRoleRequest) (*v1.SetRoleReply, error) {

	if err := s.uc.SetRole(ctx, req.Phone, req.Uid, biz.Role{RoleName: req.Role.Name,
		Description: req.Role.Description,
		Avatar:      req.Role.Avatar,
		ApiPath:     req.Role.ApiPath,
		ApiKey:      req.Role.ApiKey,
		ModelName:   req.Role.Model.Name,
		Provider:    req.Role.Model.Provider,
	}); err != nil {
		return nil, err
	}
	return &v1.SetRoleReply{Message: "success"}, nil
}
