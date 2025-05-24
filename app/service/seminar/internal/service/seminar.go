package service

import (
	"context"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"

	v1 "github.com/Fl0rencess720/Ayana/api/gateway/seminar/v1"
	"github.com/Fl0rencess720/Ayana/app/service/seminar/internal/biz"
)

type SeminarService struct {
	v1.UnimplementedSeminarServer

	uc  *biz.SeminarUsecase
	ruc *biz.RAGUsecase
}

func NewSeminarService(uc *biz.SeminarUsecase, ruc *biz.RAGUsecase) *SeminarService {
	return &SeminarService{uc: uc, ruc: ruc}
}

func (s *SeminarService) CreateTopic(ctx context.Context, req *v1.CreateTopicRequest) (*v1.CreateTopicReply, error) {
	topic, err := biz.NewTopic(req.Content, req.Moderator, req.Participants)
	if err != nil {
		return nil, err
	}
	s.uc.CreateTopic(ctx, req.Phone, req.Documents, topic)
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
	for _, document := range topic.Documents {
		reply.Topic.Documents = append(reply.Topic.Documents, &v1.Document{
			Filename:  document.Filename,
			TotalSize: strconv.Itoa(int(document.TotalSize)),
			Uid:       document.UID,
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
		documents := make([]*v1.Document, 0)
		for _, document := range topic.Documents {
			documents = append(documents, &v1.Document{
				Filename:  document.Filename,
				TotalSize: strconv.Itoa(int(document.TotalSize)),
				Uid:       document.UID,
			})
		}
		reply.Topics = append(reply.Topics, &v1.TopicMetadata{
			Uid:          topic.UID,
			Participants: topic.Participants,
			Moderator:    topic.Moderator,
			Content:      topic.Content,
			Documents:    documents,
		})
	}
	return reply, nil
}
func (s *SeminarService) StartTopic(ctx context.Context, req *v1.StartTopicRequest) (*v1.StartTopicReply, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("panic: %v", r)
		}
	}()
	if err := s.uc.StartTopic(ctx, req.Phone, req.TopicId); err != nil {
		return nil, err
	}
	return &v1.StartTopicReply{
		Message: "success",
	}, nil
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

func (s *SeminarService) GetDocuments(ctx context.Context, req *v1.GetDocumentsRequest) (*v1.GetDocumentsReply, error) {
	documents, err := s.ruc.GetDocuments(ctx, req.Phone)
	if err != nil {
		return nil, err
	}
	reply := &v1.GetDocumentsReply{}
	for _, document := range documents {
		reply.Documents = append(reply.Documents, &v1.Document{
			Filename:  document.Filename,
			TotalSize: strconv.Itoa(int(document.TotalSize)),
			Uid:       document.UID,
		})
	}
	return reply, nil
}

func (s *SeminarService) AddMCPServer(ctx context.Context, req *v1.AddMCPServerReqeust) (*v1.AddMCPServerReply, error) {
	if err := s.uc.AddMCPServer(ctx, req.Phone, req.Name, req.Url, req.RequestHeader); err != nil {
		return nil, err
	}
	return &v1.AddMCPServerReply{Message: "success"}, nil
}

func (s *SeminarService) GetMCPServers(ctx context.Context, req *v1.GetMCPServersRequest) (*v1.GetMCPServersReply, error) {
	servers, err := s.uc.GetMCPServers(ctx, req.Phone)
	if err != nil {
		return nil, err
	}
	reply := &v1.GetMCPServersReply{}
	for _, server := range servers {
		reply.Servers = append(reply.Servers, &v1.MCPServer{
			Name:          server.Name,
			RequestHeader: server.RequestHeader,
			Url:           server.URL,
			Uid:           server.UID,
			Status:        server.Status,
		})
	}
	return reply, nil
}

func (s *SeminarService) CheckMCPServerHealth(ctx context.Context, req *v1.CheckMCPServerHealthReqeust) (*v1.CheckMCPServerHealthReply, error) {
	health, err := s.uc.CheckMCPServerHealth(ctx, req.Url)
	if err != nil {
		return nil, err
	}

	return &v1.CheckMCPServerHealthReply{Health: health}, nil
}

func (s *SeminarService) DeleteMCPServer(ctx context.Context, req *v1.DeleteMCPServerRequest) (*v1.DeleteMCPServerReply, error) {
	err := s.uc.DeleteMCPServer(ctx, req.Phone, req.Uid)
	if err != nil {
		return nil, err
	}

	return &v1.DeleteMCPServerReply{Message: "success"}, nil
}

func (s *SeminarService) EnableMCPServer(ctx context.Context, req *v1.EnableMCPServerRequest) (*v1.EnableMCPServerReply, error) {
	status, err := s.uc.EnableMCPServer(ctx, req.Phone, req.Uid, req.Url)
	if err != nil {
		return nil, err
	}

	return &v1.EnableMCPServerReply{Status: status}, nil
}

func (s *SeminarService) DisableMCPServer(ctx context.Context, req *v1.DisableMCPServerRequest) (*v1.DisableMCPServerReply, error) {
	err := s.uc.DisableMCPServer(ctx, req.Phone, req.Uid)
	if err != nil {
		return nil, err
	}
	return &v1.DisableMCPServerReply{Message: "success"}, nil
}
