package data

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Fl0rencess720/Ayana/app/service/seminar/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/segmentio/kafka-go"
)

type broadcastRepo struct {
	data *Data
	log  *log.Helper
}

func NewBroadcastRepo(data *Data, logger log.Logger) biz.BroadcastRepo {
	return &broadcastRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *broadcastRepo) SendTokenToKafka(ctx context.Context, topic string, message *biz.TokenMessage) error {
	writer := r.data.kafkaClient.kafkaWriter
	if writer == nil {
		return errors.New("kafka writer is nil")
	}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if err := writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(topic),
		Value: data,
	}); err != nil {
		return err
	}
	return nil
}

func (r *broadcastRepo) SendTokensToKafkaBatch(ctx context.Context, topicUID string, tokens []*biz.TokenMessage) error {

	writer := r.data.kafkaClient.kafkaWriter

	kafkaMessages := make([]kafka.Message, 0, len(tokens))
	for _, token := range tokens {
		data, err := json.Marshal(token)
		if err != nil {
			return err
		}

		kafkaMessages = append(kafkaMessages, kafka.Message{
			Key:   []byte(topicUID),
			Value: data,
		})
	}

	return writer.WriteMessages(ctx, kafkaMessages...)
}
