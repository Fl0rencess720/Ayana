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
