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
	RegisterConnChannel(ctx context.Context, topic string, connChan chan *TokenMessage) error
	UngisterConnChannel(ctx context.Context, topic string, connChan chan *TokenMessage) error
	ReadTopic(ctx context.Context, topic string) error
	GetMessageCache(topicUID string) []*TokenMessage
	LockMutex()
	UnlockMutex()
}
