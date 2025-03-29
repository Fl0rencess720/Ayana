package biz

import (
	"context"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
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

	return &v1.CreateTopicReply{}, nil
}
