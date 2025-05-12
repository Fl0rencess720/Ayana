package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Fl0rencess720/Ayana/app/gateway/interface/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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

func (r *broadcastRepo) AddKafkaReader(topic string, offset int64) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{viper.GetString("kafka.broker")},
		Topic:       topic,
		StartOffset: offset,
	})
	if reader == nil {
		return errors.New("kafka reader is nil")
	}
	r.data.kafkaClient.kafkaReaders[topic] = reader
	return nil
}

func (r *broadcastRepo) ReadMessagesByOffset(ctx context.Context, topic string, tokenChan chan *biz.TokenMessage) error {
	reader, ok := r.data.kafkaClient.kafkaReaders[topic]
	if !ok {
		return errors.New("kafka reader not found")
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
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

			var tokenMsg biz.TokenMessage
			if err := json.Unmarshal(msg.Value, &tokenMsg); err != nil {
				zap.L().Error("failed to unmarshal message", zap.Error(err))
				continue
			}
			tokenMsg.Position = int(msg.Offset - reader.Config().StartOffset)
			select {
			case tokenChan <- &tokenMsg:
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
				return fmt.Errorf("message channel blocked for 5 seconds")
			}
		}
	}
}

func (r *broadcastRepo) ReadLastestMessage(ctx context.Context, topic string) (kafka.Message, error) {
	reader := r.data.kafkaClient.kafkaReaders[topic]
	message, err := reader.ReadMessage(ctx)
	if err != nil {
		return kafka.Message{}, err
	}
	return message, nil
}

// 多网关、多grpc服务情况
// 点击创建主题按钮，发送创建主题请求，主题存入mysql
// 点击开始按钮，发送开始讨论请求，负载均衡到了某个网关，网关生成唯一的topicID
// 网关向某个grpc服务发送开始讨论请求，grpc服务将大模型响应内容推入kafka
// 网关监听此topicID对应的kafka topic，将获取的结果使用sse响应给用户

// 若用户新开了一个窗口，点击某个主题，给某个网关发送GetTopic请求
// 网关先向grpc服务发送GetTopic请求，获取过往历史消息，再监听kafka获取是否有新的消息
// 网关将获取的结果使用sse响应给用户
