package biz

import (
	"context"
	"errors"
	"strings"

	"github.com/Fl0rencess720/Wittgenstein/pkgs/jwtc"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
)

type Profile struct {
	Name   string
	Avatar string
}

// UserRepo is a User repo.
type UserRepo interface {
	GetUser(ctx context.Context, phone string) error
	CreatUser(ctx context.Context, phone string, password string) error
	SetUser(ctx context.Context, phone string, profile Profile) error
	VerifyPassword(ctx context.Context, phone string, password string) error
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
	if err := uc.repo.CreatUser(ctx, phone, password); err != nil {
		return err
	}
	return nil
}

func (uc *UserUsecase) Login(ctx context.Context, phone, password string) error {
	return nil
}

func (uc *UserUsecase) SetProfile(ctx context.Context, profile Profile) error {
	return nil
}

func (uc *UserUsecase) GetProfile(ctx context.Context) (Profile, error) {
	return Profile{}, nil
}

func (uc *UserUsecase) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		tokenString := tr.RequestHeader().Get("Authorization")
		if tokenString == "" {
			return "", errors.New("miss token string")
		}
		parts := strings.Split(tokenString, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return "", errors.New("wrong token format")
		}
		accessToken, err := jwtc.RefreshToken(parts[1], refreshToken)
		if err != nil {
			return "", err
		}
		return accessToken, nil
	}
	return "", errors.New("wrong context")
}
