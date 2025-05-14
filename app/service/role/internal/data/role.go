package data

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Fl0rencess720/Ayana/app/service/role/internal/biz"
	"github.com/Fl0rencess720/Ayana/pkgs/utils"
	"github.com/go-kratos/kratos/v2/log"
)

type roleRepo struct {
	data *Data
	log  *log.Helper
}

func NewRoleRepo(data *Data, logger log.Logger) biz.RoleRepo {
	return &roleRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *roleRepo) CreateRole(ctx context.Context, phone string, role biz.Role) (string, error) {
	uid, err := utils.GetSnowflakeID(0)
	if err != nil {
		return "", err
	}
	role.Uid = uid
	role.Phone = phone
	if err := r.data.mysqlClient.Create(&role).Error; err != nil {
		return "", err
	}
	return uid, nil
}

func (r *roleRepo) GetRoles(ctx context.Context, phone string) ([]biz.Role, error) {
	roles := []biz.Role{}
	if err := r.data.mysqlClient.Where("phone = ?", phone).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *roleRepo) GetModeratorAndParticipantsByUIDs(ctx context.Context, phone string, moderatorUID string, participantsUIDs []string) (biz.Role, []biz.Role, error) {
	participantsResult := []biz.Role{}
	moderator := biz.Role{}
	if err := r.data.mysqlClient.Debug().Where("uid IN ?", participantsUIDs).Find(&participantsResult).Error; err != nil {
		return biz.Role{}, nil, err
	}

	if err := r.data.mysqlClient.Debug().Where("uid = ?", moderatorUID).Find(&moderator).Error; err != nil {
		return biz.Role{}, nil, err
	}
	return moderator, participantsResult, nil
}

func (r *roleRepo) DeleteRole(ctx context.Context, uid string) error {
	if err := r.data.mysqlClient.Delete(&biz.Role{}, "uid = ?", uid).Error; err != nil {
		return err
	}
	return nil
}

func (r *roleRepo) RolesToRedis(ctx context.Context, phone string, roles []biz.Role) error {
	serializedRole, err := json.Marshal(roles)
	if err != nil {
		return err
	}
	return r.data.redisClient.Set(ctx, "roles:"+phone, serializedRole, 3*24*time.Hour).Err()
}

func (r *roleRepo) GetRolesFromRedis(ctx context.Context, phone string) ([]biz.Role, error) {
	rolesByte, err := r.data.redisClient.Get(ctx, "roles:"+phone).Bytes()
	if err != nil {
		return nil, err
	}
	roles := []biz.Role{}
	if err := json.Unmarshal(rolesByte, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}
func (r *roleRepo) SetRole(ctx context.Context, phone, uid string, role biz.Role) error {
	return r.data.mysqlClient.Model(&biz.Role{}).Where("uid = ?", uid).Updates(&role).Error
}
