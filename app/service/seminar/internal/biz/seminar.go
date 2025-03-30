package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

type SeminarRepo interface {
	CreateTopic(ctx context.Context, phone string, topic *Topic) error
	DeleteTopic(ctx context.Context, topicUID string) error
	GetTopic(ctx context.Context, topicUID string) (Topic, error)
}

type SeminarUsecase struct {
	repo SeminarRepo
	log  *log.Helper
}

func NewSeminarUsecase(repo SeminarRepo, logger log.Logger) *SeminarUsecase {
	return &SeminarUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *SeminarUsecase) CreateTopic(ctx context.Context, phone string, topic *Topic) error {
	if err := uc.repo.CreateTopic(ctx, phone, topic); err != nil {
		return err
	}
	return nil
}

func (uc *SeminarUsecase) DeleteTopic(ctx context.Context, topicUID string) error {
	if err := uc.repo.DeleteTopic(ctx, topicUID); err != nil {
		return err
	}
	return nil
}

func (uc *SeminarUsecase) GetTopic(ctx context.Context, topicUID string) (Topic, error) {
	topic, err := uc.repo.GetTopic(ctx, topicUID)
	if err != nil {
		return Topic{}, err
	}
	return topic, nil
}
