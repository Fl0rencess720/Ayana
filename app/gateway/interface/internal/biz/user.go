package biz

import (
	"context"
	"errors"
	"strings"

	userV1 "github.com/Fl0rencess720/Ayana/api/gateway/user/v1"
	"github.com/Fl0rencess720/Ayana/pkgs/jwtc"
	"github.com/Fl0rencess720/Ayana/pkgs/utils"
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
	repo       UserRepo
	log        *log.Helper
	userClient userV1.UserClient
}

// NewUserUsecase new a User usecase.
func NewUserUsecase(repo UserRepo, logger log.Logger, userClient userV1.UserClient) *UserUsecase {
	return &UserUsecase{repo: repo, log: log.NewHelper(logger), userClient: userClient}
}

func (uc *UserUsecase) Register(ctx context.Context, req *userV1.RegisterRequest) (*userV1.RegisterReply, error) {
	reply, err := uc.userClient.Register(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (uc *UserUsecase) Login(ctx context.Context, req *userV1.LoginRequest) (*userV1.LoginReply, error) {
	reply, err := uc.userClient.Login(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (uc *UserUsecase) SetProfile(ctx context.Context, req *userV1.SetProfileRequest) (*userV1.SetProfileReply, error) {
	req.Phone = utils.GetPhoneFromContext(ctx)
	reply, err := uc.userClient.SetProfile(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (uc *UserUsecase) GetProfile(ctx context.Context, req *userV1.GetProfileRequest) (*userV1.GetProfileReply, error) {
	req.Phone = utils.GetPhoneFromContext(ctx)
	reply, err := uc.userClient.GetProfile(ctx, req)
	if err != nil {
		return reply, err
	}
	return reply, nil
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
