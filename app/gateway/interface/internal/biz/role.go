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
	Api_path    string
	Api_key     string
	Model       Model
	Name        string
}

type Model struct {
	Name     string
	Provider string
}

type RoleUsecase struct {
	repo       RoleRepo
	log        *log.Helper
	userClient roleV1.RoleManagerClient
}

func NewRoleUsecase(repo RoleRepo, logger log.Logger, userClient roleV1.RoleManagerClient) *RoleUsecase {
	return &RoleUsecase{repo: repo, log: log.NewHelper(logger), userClient: userClient}
}

func (uc *RoleUsecase) CallRole() {
}

func (uc *RoleUsecase) CreateRole(ctx context.Context, req *roleV1.CreateRoleRequest) (*roleV1.CreateRoleReply, error) {
	return uc.userClient.CreateRole(ctx, req)
}

func (uc *RoleUsecase) GetRoles(ctx context.Context, req *roleV1.GetRolesRequest) (*roleV1.GetRolesReply, error) {
	phone := utils.GetPhoneFromContext(ctx)
	roles, err := uc.repo.GetRolesFromRedis(ctx, phone)
	if err == nil {
		rs := make([]*roleV1.Role, 0, len(roles))
		for _, r := range roles {
			rs = append(rs, &roleV1.Role{
				Name:        r.Name,
				Uid:         r.Uid,
				Description: r.Description,
				Avatar:      r.Avatar,
				ApiPath:     r.Api_key,
				ApiKey:      r.Api_key,
				Model:       r.Model.Name,
			})
		}
		return &roleV1.GetRolesReply{
			Roles: rs,
		}, nil
	}
	req.Phone = phone
	return uc.userClient.GetRoles(ctx, req)
}

func (uc *RoleUsecase) DeleteRole(ctx context.Context, req *roleV1.DeleteRoleRequest) (*roleV1.DeleteRoleReply, error) {
	return uc.userClient.DeleteRole(ctx, req)

}

func (uc *RoleUsecase) GetAvailableModels(ctx context.Context, req *roleV1.GetAvailableModelsRequest) (*roleV1.GetAvailableModelsReply, error) {
	models, err := uc.repo.GetAvailableModelsFromRedis(ctx)
	if err == nil {
		ms := make([]*roleV1.Model, 0, len(models))
		for _, m := range models {
			ms = append(ms, &roleV1.Model{
				Name:     m.Name,
				Provider: m.Provider,
			})
		}
		return &roleV1.GetAvailableModelsReply{
			Models: ms,
		}, nil
	}
	return uc.userClient.GetAvailableModels(ctx, req)
}
