package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
	"github.com/Fl0rencess720/Wittgenstein/app/service/seminar/internal/biz"
	"google.golang.org/grpc"
)

type SeminarService struct {
	v1.UnimplementedSeminarServer

	uc  *biz.SeminarUsecase
	ruc *biz.RAGUsecase
}

func NewSeminarService(uc *biz.SeminarUsecase) *SeminarService {
	return &SeminarService{uc: uc}
}

func (s *SeminarService) CreateTopic(ctx context.Context, req *v1.CreateTopicRequest) (*v1.CreateTopicReply, error) {
	topic, err := biz.NewTopic(req.Content, req.Moderator, req.Participants)
	if err != nil {
		return nil, err
	}
	s.uc.CreateTopic(ctx, req.Phone, topic)
	return &v1.CreateTopicReply{Uid: topic.UID}, nil
}
func (s *SeminarService) DeleteTopic(ctx context.Context, req *v1.DeleteTopicRequest) (*v1.DeleteTopicReply, error) {
	if err := s.uc.DeleteTopic(ctx, req.Uid); err != nil {
		return nil, err
	}
	return &v1.DeleteTopicReply{Message: "success"}, nil
}
func (s *SeminarService) GetTopic(ctx context.Context, req *v1.GetTopicRequest) (*v1.GetTopicReply, error) {
	topic, err := s.uc.GetTopic(ctx, req.Uid)
	if err != nil {
		return nil, err
	}
	reply := &v1.GetTopicReply{Topic: &v1.Topic{
		Uid:          topic.UID,
		Content:      topic.Content,
		Participants: topic.Participants,
		Title:        topic.Title,
		TitleImage:   topic.TitleImage,
	}}
	for _, speech := range topic.Speeches {
		reply.Topic.Speeches = append(reply.Topic.Speeches, &v1.Speech{
			Uid:     speech.UID,
			RoleUid: speech.RoleUID,
			Content: speech.Content,
		})
	}
	return reply, nil
}
func (s *SeminarService) GetTopicsMetadata(ctx context.Context, req *v1.GetTopicsMetadataRequest) (*v1.GetTopicsMetadataReply, error) {
	topics, err := s.uc.GetTopicsMetadata(ctx, req.Phone)
	if err != nil {
		return nil, err
	}
	reply := &v1.GetTopicsMetadataReply{}
	for _, topic := range topics {
		reply.Topics = append(reply.Topics, &v1.TopicMetadata{
			Uid:          topic.UID,
			Participants: topic.Participants,
			Content:      topic.Content,
		})
	}
	return reply, nil
}
func (s *SeminarService) StartTopic(req *v1.StartTopicRequest, stream grpc.ServerStreamingServer[v1.StreamOutputReply]) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("panic: %v", r)
		}
	}()
	if err := s.uc.StartTopic(req.TopicId, stream); err != nil {
		return err
	}
	return nil
}

func (s *SeminarService) StopTopic(ctx context.Context, req *v1.StopTopicRequest) (*v1.StopTopicReply, error) {
	if err := s.uc.StopTopic(ctx, req.TopicId); err != nil {
		return nil, err
	}
	return &v1.StopTopicReply{Message: "success"}, nil
}

func (s *SeminarService) UploadDocument(stream v1.Seminar_UploadDocumentServer) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("panic: %v", r)
		}
	}()

	if err := s.ruc.UploadDocument(stream); err != nil {
		return err
	}
	return stream.SendAndClose(&v1.UploadDocumentReply{
		Message: "success",
	})
}
