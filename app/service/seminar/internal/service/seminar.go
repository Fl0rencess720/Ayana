package service

import (
	"context"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
	"github.com/Fl0rencess720/Wittgenstein/app/service/seminar/internal/biz"
)

type SeminarService struct {
	v1.UnimplementedSeminarServer

	uc *biz.SeminarUsecase
}

func NewSeminarService(uc *biz.SeminarUsecase) *SeminarService {
	return &SeminarService{uc: uc}
}

func (s *SeminarService) CreateTopic(ctx context.Context, req *v1.CreateTopicRequest) (*v1.CreateTopicReply, error) {
	topic, err := biz.NewTopic(req.Content, req.Participants)
	if err != nil {
		return nil, err
	}
	s.uc.CreateTopic(ctx, req.Phone, topic)
	return &v1.CreateTopicReply{Uid: topic.UID}, nil
}
func (s *SeminarService) DeleteTopic(ctx context.Context, req *v1.DeleteTopicRequest) (*v1.DeleteTopicReply, error) {
	return nil, nil
}
func (s *SeminarService) GetTopic(ctx context.Context, req *v1.GetTopicRequest) (*v1.GetTopicReply, error) {
	return nil, nil
}
func (s *SeminarService) GetTopicsMetadata(ctx context.Context, req *v1.GetTopicsMetadataRequest) (*v1.GetTopicsMetadataReply, error) {
	return nil, nil
}
func (s *SeminarService) StartTopic(ctx context.Context, req *v1.StartTopicRequest) (*v1.StartTopicReply, error) {
	return nil, nil
}
