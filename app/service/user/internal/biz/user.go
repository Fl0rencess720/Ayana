package biz

import (
	"context"
	"errors"

	"github.com/Fl0rencess720/Wittgenstein/pkgs/jwtc"
	"github.com/Fl0rencess720/Wittgenstein/pkgs/utils"
	"github.com/go-kratos/kratos/v2/log"
)

type Profile struct {
	Name   string
	Avatar string
}

// UserRepo is a User repo.
type UserRepo interface {
	ExistUser(ctx context.Context, phone string) (bool, error)
	CreatUser(ctx context.Context, phone string, password string) error
	SetProfile(ctx context.Context, phone string, profile Profile) error
	GetProfile(ctx context.Context, phone string) (Profile, error)
	VerifyPassword(ctx context.Context, phone string, password string) error
	ProfileToRedis(ctx context.Context, phone string, profile Profile) error
}

// UserUsecase is a User usecase.
type UserUsecase struct {
	repo UserRepo
	log  *log.Helper
}

// NewUserUsecase new a User usecase.
func NewUserUsecase(repo UserRepo, logger log.Logger) *UserUsecase {
	return &UserUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *UserUsecase) Register(ctx context.Context, phone, password string) error {
	exist, err := uc.repo.ExistUser(ctx, phone)
	if err != nil {
		return err
	}
	if exist {
		return errors.New("phone already exists")
	}
	hash, err := utils.GenBcryptHash(password)
	if err != nil {
		return err
	}
	if err := uc.repo.CreatUser(ctx, phone, hash); err != nil {
		return err
	}
	return nil
}

func (uc *UserUsecase) Login(ctx context.Context, phone, password string) (string, string, error) {
	exist, err := uc.repo.ExistUser(ctx, phone)
	if err != nil {
		return "", "", err
	}
	if !exist {
		return "", "", errors.New("phone not exists")
	}
	hash, err := utils.GenBcryptHash(password)
	if err != nil {
		return "", "", err
	}
	if err := uc.repo.VerifyPassword(ctx, phone, hash); err != nil {
		return "", "", err
	}
	accessToken, refreshToken, err := jwtc.GenToken(phone)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, err
}

func (uc *UserUsecase) SetProfile(ctx context.Context, phone string, profile Profile) error {
	if err := uc.repo.SetProfile(ctx, phone, profile); err != nil {
		return err
	}
	if err := uc.repo.ProfileToRedis(ctx, phone, profile); err != nil {
		return err
	}
	return nil
}

func (uc *UserUsecase) GetProfile(ctx context.Context, phone string) (Profile, error) {
	profile, err := uc.repo.GetProfile(ctx, phone)
	if err != nil {
		return Profile{}, err
	}
	return profile, nil
}
