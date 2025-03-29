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

func (s *SeminarService) CreateTopic(context.Context, *v1.CreateTopicRequest) (*v1.CreateTopicReply, error) {
	return nil, nil
}
func (s *SeminarService) DeleteTopic(context.Context, *v1.DeleteTopicRequest) (*v1.DeleteTopicReply, error) {
	return nil, nil
}
func (s *SeminarService) GetTopic(context.Context, *v1.GetTopicRequest) (*v1.GetTopicReply, error) {
	return nil, nil
}
func (s *SeminarService) GetTopicsMetadata(context.Context, *v1.GetTopicsMetadataRequest) (*v1.GetTopicsMetadataReply, error) {
	return nil, nil
}
func (s *SeminarService) StartTopic(context.Context, *v1.StartTopicRequest) (*v1.StartTopicReply, error) {
	return nil, nil
}
