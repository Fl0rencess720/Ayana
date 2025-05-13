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
	IsFirst     bool   `json:"is_first"`
}

type BroadcastRepo interface {
	SendTokenToKafka(ctx context.Context, topic string, message *TokenMessage) error
	SendTokensToKafkaBatch(ctx context.Context, topicUID string, tokens []*TokenMessage) error
	AddTopicPauseChannel(ctx context.Context, topicUID string, pauseChan chan StateSignal) error
	DeleteTopicPauseContext(ctx context.Context, topicUID string) error
	SendPauseSignalToKafka(ctx context.Context, topic string) error
	ReadPauseSignalFromKafka(ctx context.Context) error
}
