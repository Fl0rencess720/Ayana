package service

import (
	"context"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
	"github.com/Fl0rencess720/Wittgenstein/app/gateway/interface/internal/biz"
)

type SeminarService struct {
	v1.UnimplementedSeminarServer

	uc *biz.SeminarUsecase
}

func NewSeminarService(uc *biz.SeminarUsecase) *SeminarService {
	return &SeminarService{uc: uc}
}

func (s *SeminarService) CreateTopic(ctx context.Context, req *v1.CreateTopicRequest) (*v1.CreateTopicReply, error) {
	reply, err := s.uc.CreateTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func (s *SeminarService) DeleteTopic(ctx context.Context, req *v1.DeleteTopicRequest) (*v1.DeleteTopicReply, error) {
	reply, err := s.uc.DeleteTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func (s *SeminarService) GetTopic(ctx context.Context, req *v1.GetTopicRequest) (*v1.GetTopicReply, error) {
	reply, err := s.uc.GetTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func (s *SeminarService) GetTopicsMetadata(ctx context.Context, req *v1.GetTopicsMetadataRequest) (*v1.GetTopicsMetadataReply, error) {
	return nil, nil
}
func (s *SeminarService) StartTopic(ctx context.Context, req *v1.StartTopicRequest) (*v1.StartTopicReply, error) {
	return nil, nil
}
