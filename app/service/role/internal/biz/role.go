package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type RoleRepo interface {
	CreateRole(ctx context.Context, phone string, role Role) (string, error)
	GetRoles(ctx context.Context, phone string) ([]Role, error)
	DeleteRole(ctx context.Context, uid string) error
	RolesToRedis(ctx context.Context, phone string, roles []Role) error
}

type AIModel struct {
	gorm.Model
	ModelName string `gorm:"type:varchar(50)"`
	Provider  string `gorm:"type:varchar(50)"`
}

type Role struct {
	gorm.Model
	Phone       string `gorm:"type:varchar(50);primaryKey"`
	Uid         string `gorm:"type:varchar(50)"`
	RoleName    string `gorm:"type:varchar(50)"`
	Description string `gorm:"type:text"`
	Avatar      string `gorm:"type:varchar(50)"`
	ApiPath     string `gorm:"type:varchar(50)"`
	ApiKey      string `gorm:"type:varchar(50)"`
	ModelName   string `gorm:"type:varchar(50)"`
	Provider    string `gorm:"type:varchar(50)"`
}

type RoleUsecase struct {
	repo RoleRepo
	log  *log.Helper
}

func NewRoleUsecase(repo RoleRepo, logger log.Logger) *RoleUsecase {
	return &RoleUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *RoleUsecase) CreateRole(ctx context.Context, phone string, role Role) (string, error) {
	uid, err := uc.repo.CreateRole(ctx, phone, role)
	if err != nil {
		return "", err
	}
	roles, err := uc.repo.GetRoles(ctx, phone)
	if err != nil {
		return "", err
	}
	if err = uc.repo.RolesToRedis(ctx, phone, roles); err != nil {
		uc.log.Error(err)
	}
	return uid, nil
}

func (uc *RoleUsecase) GetRoles(ctx context.Context, phone string) ([]Role, error) {
	roles, err := uc.repo.GetRoles(ctx, phone)
	if err != nil {
		return nil, err
	}
	if err = uc.repo.RolesToRedis(ctx, phone, roles); err != nil {
		uc.log.Error(err)
	}
	return roles, nil
}

func (uc *RoleUsecase) DeleteRole(ctx context.Context, phone, uid string) error {
	if err := uc.repo.DeleteRole(ctx, uid); err != nil {
		return err
	}
	roles, err := uc.repo.GetRoles(ctx, phone)
	if err != nil {
		uc.log.Error(err)
		return nil
	}
	if err = uc.repo.RolesToRedis(ctx, phone, roles); err != nil {
		uc.log.Error(err)
	}
	return nil
}
