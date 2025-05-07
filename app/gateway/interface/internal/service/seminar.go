package service

import (
	"context"

	v1 "github.com/Fl0rencess720/Ayana/api/gateway/seminar/v1"
	"github.com/Fl0rencess720/Ayana/app/gateway/interface/internal/biz"
	"github.com/go-kratos/kratos/v2/transport/http"
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
	reply, err := s.uc.GetTopicsMetadata(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func StartTopic(ctx http.Context) error {
	h := ctx.Middleware(func(c context.Context, req interface{}) (interface{}, error) {
		return biz.StartTopic(ctx)
	})
	_, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (s *SeminarService) StopTopic(ctx context.Context, req *v1.StopTopicRequest) (*v1.StopTopicReply, error) {
	reply, err := s.uc.StopTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func ResumeTopic(ctx http.Context) error {
	h := ctx.Middleware(func(c context.Context, req interface{}) (interface{}, error) {
		return biz.StartTopic(ctx)
	})
	_, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func UploadDocument(ctx http.Context) error {
	h := ctx.Middleware(func(c context.Context, req interface{}) (interface{}, error) {
		file, handler, err := ctx.Request().FormFile("file")
		if err != nil {
			return nil, err
		}
		defer file.Close()

		return biz.UploadDocument(c, file, handler)
	})
	_, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return ctx.JSON(200, map[string]interface{}{
		"message": "上传成功",
	})
}
