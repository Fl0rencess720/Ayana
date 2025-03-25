package service

import (
	"context"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/user/v1"
	"github.com/Fl0rencess720/Wittgenstein/app/service/user/internal/biz"
)

type UserService struct {
	v1.UnimplementedUserServer

	uc *biz.UserUsecase
}

func NewUserService(uc *biz.UserUsecase) *UserService {
	return &UserService{uc: uc}
}

func (s *UserService) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginReply, error) {
	if err := s.uc.Login(ctx, req.Phone, req.Password); err != nil {
		return nil, err
	}
	return &v1.LoginReply{}, nil
}
func (s *UserService) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.RegisterReply, error) {
	if err := s.uc.Register(ctx, req.Phone, req.Password); err != nil {
		return nil, err
	}
	return &v1.RegisterReply{}, nil
}

func (s *UserService) SetProfile(ctx context.Context, req *v1.SetProfileRequest) (*v1.SetProfileReply, error) {
	if err := s.uc.SetProfile(ctx, biz.Profile{Name: req.Profile.Name, Avatar: req.Profile.Avatar}); err != nil {
		return nil, err
	}
	return &v1.SetProfileReply{}, nil
}

func (s *UserService) GetProfile(ctx context.Context, req *v1.GetProfileRequest) (*v1.GetProfileReply, error) {
	profile, err := s.uc.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.GetProfileReply{Profile: &v1.Profile{Name: profile.Name, Avatar: profile.Avatar}}, nil
}

func (s *UserService) RefreshToken(ctx context.Context, req *v1.RefreshTokenRequest) (*v1.RefreshTokenReply, error) {
	accessToken, err := s.uc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &v1.RefreshTokenReply{AccessToken: accessToken}, nil
}
