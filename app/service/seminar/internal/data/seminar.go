package data

import (
	"context"
	"fmt"

	"github.com/Fl0rencess720/Ayana/app/service/seminar/internal/biz"
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

func (r *seminarRepo) GetTopic(ctx context.Context, uid string) (*biz.Topic, error) {
	var topic *biz.Topic
	if err := r.data.mysqlClient.Model(topic).Preload("Speeches").Preload("Documents").Where("uid = ?", uid).First(&topic).Error; err != nil {
		return nil, err
	}

	return topic, nil
}

func (r *seminarRepo) GetTopicsMetadata(ctx context.Context, phone string) ([]biz.Topic, error) {
	var topics []biz.Topic
	if err := r.data.mysqlClient.Preload("Documents").Model(&biz.Topic{}).Where("phone = ?", phone).Find(&topics).Error; err != nil {
		return nil, err
	}
	return topics, nil
}
func (r *seminarRepo) SaveSpeech(ctx context.Context, speech *biz.Speech) error {
	if err := r.data.mysqlClient.Create(speech).Error; err != nil {
		return err
	}
	return nil
}

func (r *seminarRepo) SaveSpeechToRedis(ctx context.Context, speech *biz.Speech) error {
	if err := r.data.redisClient.Set(ctx, speech.UID, speech, 0).Err(); err != nil {
		return err
	}
	return nil
}

func (r *seminarRepo) LockTopic(ctx context.Context, topicUID string, lockerUID string) error {
	if err := r.data.redisClient.SetNX(ctx, topicUID, lockerUID, 0).Err(); err != nil {
		return err
	}
	return nil
}

func (r *seminarRepo) UnlockTopic(topicUID string, lockerUID string) error {
	ctx := context.Background()
	res, err := r.data.redisClient.Del(ctx, topicUID).Result()
	if err != nil {
		return fmt.Errorf("unlock failed: %v", err)
	}
	if res != 1 {
		return fmt.Errorf("unlock failed: key does not exist")
	}
	return nil
}
