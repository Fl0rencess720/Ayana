package data

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Fl0rencess720/Ayana/app/service/seminar/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
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

func (r *broadcastRepo) AddKafkaWriter(topic string) error {
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{viper.GetString("kafka.broker")},
		Topic:   topic,
	})
	if writer == nil {
		return errors.New("kafka reader is nil")
	}
	r.data.kafkaClient.kafkaWriters[topic] = writer
	return nil
}

func (r *broadcastRepo) SendMessageToKafka(ctx context.Context, topic string, message *biz.TokenMessage) error {
	writer := r.data.kafkaClient.kafkaWriters[topic]
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
