package service

import (
	"context"

	v1 "github.com/Fl0rencess720/Ayana/api/gateway/user/v1"
	"github.com/Fl0rencess720/Ayana/app/gateway/interface/internal/biz"
)

type UserService struct {
	v1.UnimplementedUserServer

	uc *biz.UserUsecase
}

func NewUserService(uc *biz.UserUsecase) *UserService {
	return &UserService{uc: uc}
}

func (s *UserService) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginReply, error) {
	reply, err := s.uc.Login(ctx, req)
	if err != nil {
		return nil, err
	}
	reply.AccessToken = "Bearer " + reply.AccessToken
	reply.RefreshToken = "Bearer " + reply.RefreshToken
	return reply, nil
}
func (s *UserService) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.RegisterReply, error) {
	reply, err := s.uc.Register(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (s *UserService) SetProfile(ctx context.Context, req *v1.SetProfileRequest) (*v1.SetProfileReply, error) {
	reply, err := s.uc.SetProfile(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (s *UserService) GetProfile(ctx context.Context, req *v1.GetProfileRequest) (*v1.GetProfileReply, error) {
	reply, err := s.uc.GetProfile(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (s *UserService) RefreshToken(ctx context.Context, req *v1.RefreshTokenRequest) (*v1.RefreshTokenReply, error) {
	accessToken, err := s.uc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &v1.RefreshTokenReply{AccessToken: "Bearer " + accessToken}, nil
}
