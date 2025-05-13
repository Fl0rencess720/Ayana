package data

import (
	"context"
	"fmt"

	"github.com/Fl0rencess720/Ayana/app/gateway/interface/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
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

// false 说明未上锁，true 说明上锁
func (r *seminarRepo) GetTopicLockStatus(ctx context.Context, topicUID string) (bool, error) {

	_, err := r.data.redisClient.Get(ctx, topicUID).Result()
	if err == redis.Nil {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to check lock status: %v", err)
	}
	return true, nil
}
