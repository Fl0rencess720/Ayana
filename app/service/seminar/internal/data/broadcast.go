package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

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

func (r *broadcastRepo) AddTopicPauseChannel(ctx context.Context, topicUID string, pauseChan chan biz.StateSignal) error {
	r.data.mu.Lock()
	r.data.topicPauseContextMap[topicUID] = pauseChan
	r.data.mu.Unlock()
	return nil
}

func (r *broadcastRepo) DeleteTopicPauseContext(ctx context.Context, topicUID string) error {
	r.data.mu.Lock()
	delete(r.data.topicPauseContextMap, topicUID)
	r.data.mu.Unlock()
	return nil
}

func (r *broadcastRepo) SendPauseSignalToKafka(ctx context.Context, topic string) error {
	writer := r.data.kafkaClient.pauseSignalWriter
	if writer == nil {
		return errors.New("kafka writer is nil")
	}
	return writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(topic),
		Value: []byte(topic),
	})
}

func (r *broadcastRepo) ReadPauseSignalFromKafka(ctx context.Context) error {
	reader := r.data.kafkaClient.pauseSignalReader
	if reader == nil {
		return errors.New("kafka reader is nil")
	}
	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			switch {
			case errors.Is(err, context.Canceled):
				return nil
			case errors.Is(err, io.EOF):
				return fmt.Errorf("kafka connection closed: %w", err)
			default:
				return fmt.Errorf("kafka read error: %w", err)
			}
		}

		if c, ok := r.data.topicPauseContextMap[string(msg.Key)]; ok {
			c <- biz.Pause
		}
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
