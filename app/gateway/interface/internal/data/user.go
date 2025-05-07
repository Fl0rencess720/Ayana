package data

import (
	"context"

	"github.com/Fl0rencess720/Ayana/app/gateway/interface/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
)

type userRepo struct {
	data *Data
	log  *log.Helper
}

func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *userRepo) GetUser(ctx context.Context, phone string) error {
	return nil
}
func (r *userRepo) CreatUser(ctx context.Context, phone string, password string) error {
	return nil
}
func (r *userRepo) SetUser(ctx context.Context, phone string, profile biz.Profile) error {
	return nil
}
func (r *userRepo) VerifyPassword(ctx context.Context, phone string, password string) error {
	return nil
}
