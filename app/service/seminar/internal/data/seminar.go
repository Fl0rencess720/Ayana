package data

import (
	"context"

	"github.com/Fl0rencess720/Wittgenstein/app/service/seminar/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
)

type seminarRepo struct {
	data *Data
	log  *log.Helper
}

func NewSeminarRepo(data *Data, logger log.Logger) biz.SeminarRepo {
	return &seminarRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *seminarRepo) CreateTopic(ctx context.Context, phone string, topic *biz.Topic) error {
	topic.Phone = phone
	if err := r.data.mysqlClient.Create(topic).Error; err != nil {
		return err
	}
	return nil
}

func (r *seminarRepo) DeleteTopic(ctx context.Context, topicUID string) error {
	if err := r.data.mysqlClient.Model(&biz.Topic{}).Where("uid = ?", topicUID).Unscoped().Delete(&biz.Topic{}).Error; err != nil {
		return err
	}
	return nil
}

func (r *seminarRepo) GetTopic(ctx context.Context, uid string) (biz.Topic, error) {
	var topic biz.Topic
	if err := r.data.mysqlClient.Model(&biz.Topic{}).Preload("Speeches").Where("uid = ?", uid).First(&topic).Error; err != nil {
		return biz.Topic{}, err
	}
	return topic, nil
}
