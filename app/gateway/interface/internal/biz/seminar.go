package biz

import (
	"context"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
	"github.com/Fl0rencess720/Wittgenstein/pkgs/utils"
	"github.com/go-kratos/kratos/v2/log"
)

type SeminarUsecase struct {
	repo UserRepo
	log  *log.Helper

	seminarClient v1.SeminarClient
}

func NewSeminarUsecase(repo UserRepo, logger log.Logger, seminarClient v1.SeminarClient) *SeminarUsecase {
	return &SeminarUsecase{repo: repo, log: log.NewHelper(logger), seminarClient: seminarClient}
}

func (uc *SeminarUsecase) CreateTopic(ctx context.Context, req *v1.CreateTopicRequest) (*v1.CreateTopicReply, error) {
	req.Phone = utils.GetPhoneFromContext(ctx)
	reply, err := uc.seminarClient.CreateTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (uc *SeminarUsecase) DeleteTopic(ctx context.Context, req *v1.DeleteTopicRequest) (*v1.DeleteTopicReply, error) {
	reply, err := uc.seminarClient.DeleteTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (uc *SeminarUsecase) GetTopic(ctx context.Context, req *v1.GetTopicRequest) (*v1.GetTopicReply, error) {
	reply, err := uc.seminarClient.GetTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
