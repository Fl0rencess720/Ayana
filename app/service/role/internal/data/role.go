package data

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Fl0rencess720/Wittgenstein/app/service/role/internal/biz"
	"github.com/Fl0rencess720/Wittgenstein/pkgs/utils"
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

func (r *roleRepo) RoleToRedis(ctx context.Context, phone string, roles []biz.Role) error {
	serializedRole, err := json.Marshal(roles)
	if err != nil {
		return err
	}
	return r.data.redisClient.Set(ctx, "roles:"+phone, serializedRole, 3*24*time.Hour).Err()
}
