package data

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Fl0rencess720/Ayana/app/service/user/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
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

func (r *userRepo) ExistUser(ctx context.Context, phone string) (bool, error) {
	var existUser User
	result := r.data.mysqlClient.Where("phone = ?", phone).First(&existUser)
	if result.Error == nil {
		return true, nil
	}
	if result.Error == gorm.ErrRecordNotFound {
		return false, nil
	}
	return false, result.Error
}
func (r *userRepo) CreatUser(ctx context.Context, phone string, password string) error {
	if err := r.data.mysqlClient.Create(&User{
		Phone:    phone,
		Password: password,
	}).Error; err != nil {
		return err
	}
	return nil
}
func (r *userRepo) SetProfile(ctx context.Context, phone string, profile biz.Profile) error {
	if err := r.data.mysqlClient.Model(&User{}).Where("phone = ?", phone).Updates(User{
		Name:   profile.Name,
		Avatar: profile.Avatar,
	}).Error; err != nil {
		return err
	}
	return nil
}

func (r *userRepo) GetProfile(ctx context.Context, phone string) (biz.Profile, error) {
	var existUser User
	if err := r.data.mysqlClient.Where("phone = ?", phone).First(&existUser).Error; err != nil {
		return biz.Profile{}, err
	}
	return biz.Profile{
		Avatar: existUser.Avatar,
		Name:   existUser.Name,
	}, nil
}

func (r *userRepo) VerifyPassword(ctx context.Context, phone string, password string) error {
	var existUser User
	if err := r.data.mysqlClient.Where("phone = ?", phone).First(&existUser).Error; err != nil {
		return err
	}
	return nil
}

func (r *userRepo) ProfileToRedis(ctx context.Context, phone string, profile biz.Profile) error {
	serialized, _ := json.Marshal(profile)
	if err := r.data.redisClient.Set(ctx, "profile:"+phone, serialized, 3*24*time.Hour).Err(); err != nil {
		return err
	}
	return nil
}
