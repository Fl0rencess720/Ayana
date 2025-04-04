package biz

import (
	"context"

	roleV1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/role/v1"
	"github.com/Fl0rencess720/Wittgenstein/pkgs/utils"
	"github.com/go-kratos/kratos/v2/log"
)

type RoleRepo interface {
	GetAvailableModelsFromRedis(ctx context.Context) ([]Model, error)
	GetRolesFromRedis(ctx context.Context, phone string) ([]Role, error)
}

type Role struct {
	Uid         string
	Description string
	Avatar      string
	ApiPath     string
	ApiKey      string
	RoleName    string
	Model
}

type Model struct {
	ModelName string
	Provider  string
}

type RoleUsecase struct {
	repo       RoleRepo
	log        *log.Helper
	roleClient roleV1.RoleManagerClient
}

func NewRoleUsecase(repo RoleRepo, logger log.Logger, roleClient roleV1.RoleManagerClient) *RoleUsecase {
	return &RoleUsecase{repo: repo, log: log.NewHelper(logger), roleClient: roleClient}
}

func (uc *RoleUsecase) CallRole() {
}

func (uc *RoleUsecase) CreateRole(ctx context.Context, req *roleV1.CreateRoleRequest) (*roleV1.CreateRoleReply, error) {
	return uc.roleClient.CreateRole(ctx, req)
}

func (uc *RoleUsecase) GetRoles(ctx context.Context, req *roleV1.GetRolesRequest) (*roleV1.GetRolesReply, error) {
	phone := utils.GetPhoneFromContext(ctx)
	roles, err := uc.repo.GetRolesFromRedis(ctx, phone)
	if err == nil {
		rs := make([]*roleV1.Role, 0, len(roles))
		for _, r := range roles {
			rs = append(rs, &roleV1.Role{
				Name:        r.RoleName,
				Uid:         r.Uid,
				Description: r.Description,
				Avatar:      r.Avatar,
				ApiPath:     r.ApiPath,
				ApiKey:      r.ApiKey,
				Model:       &roleV1.Model{Provider: r.Model.Provider, Name: r.Model.ModelName},
			})
		}
		return &roleV1.GetRolesReply{
			Roles: rs,
		}, nil
	}
	req.Phone = phone
	return uc.roleClient.GetRoles(ctx, req)
}

func (uc *RoleUsecase) DeleteRole(ctx context.Context, req *roleV1.DeleteRoleRequest) (*roleV1.DeleteRoleReply, error) {
	phone := utils.GetPhoneFromContext(ctx)
	req.Phone = phone
	return uc.roleClient.DeleteRole(ctx, req)

}

func (uc *RoleUsecase) GetAvailableModels(ctx context.Context, req *roleV1.GetAvailableModelsRequest) (*roleV1.GetAvailableModelsReply, error) {
	models, err := uc.repo.GetAvailableModelsFromRedis(ctx)
	if err == nil {
		ms := make([]*roleV1.Model, 0, len(models))
		for _, m := range models {
			ms = append(ms, &roleV1.Model{
				Name:     m.ModelName,
				Provider: m.Provider,
			})
		}
		return &roleV1.GetAvailableModelsReply{
			Models: ms,
		}, nil
	}
	return uc.roleClient.GetAvailableModels(ctx, req)
}
func (uc *RoleUsecase) SetRole(ctx context.Context, req *roleV1.SetRoleRequest) (*roleV1.SetRoleReply, error) {
	reply, err := uc.roleClient.SetRole(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
