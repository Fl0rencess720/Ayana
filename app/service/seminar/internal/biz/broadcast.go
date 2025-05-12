package biz

import (
	"context"
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
	AddKafkaWriter(topic string) error
	SendMessageToKafka(ctx context.Context, topic string, message *TokenMessage) error
}
