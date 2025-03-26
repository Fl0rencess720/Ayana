package data

import (
	"context"
	"encoding/json"

	"github.com/Fl0rencess720/Wittgenstein/app/gateway/interface/internal/biz"
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

func (r *roleRepo) GetAvailableModelsFromRedis(ctx context.Context) ([]biz.Model, error) {
	data, err := r.data.redisClient.Get(context.Background(), "models").Bytes()
	if err != nil {
		return nil, err
	}
	var models []biz.Model
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, err
	}
	return models, nil
}
func (r *roleRepo) GetRolesFromRedis(ctx context.Context, phone string) ([]biz.Role, error) {
	data, err := r.data.redisClient.Get(context.Background(), "roles:"+phone).Bytes()
	if err != nil {
		return nil, err
	}
	var roles []biz.Role
	if err := json.Unmarshal(data, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}
