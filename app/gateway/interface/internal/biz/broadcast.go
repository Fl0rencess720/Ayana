package biz

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type TokenMessage struct {
	TopicUID    string `json:"topic_uid"`
	RoleUID     string `json:"role_uid"`
	RoleName    string `json:"role_name"`
	RoleType    string `json:"role_type"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
	Position    int    `json:"position"`
}

type BroadcastRepo interface {
	AddKafkaReader(topic string, offset int64) error
	ReadMessagesByOffset(ctx context.Context, topic string, tokenChan chan *TokenMessage) error
	ReadLastestMessage(ctx context.Context, topic string) (kafka.Message, error)
}
