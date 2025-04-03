package biz

import (
	"context"
	"io"
	"sync"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type Role struct {
	gorm.Model
	Phone       string `gorm:"type:varchar(50);primaryKey"`
	Uid         string `gorm:"type:varchar(50)"`
	RoleName    string `gorm:"type:varchar(50)"`
	Description string `gorm:"type:text"`
	Avatar      string `gorm:"type:varchar(50)"`
	ApiPath     string `gorm:"type:varchar(50)"`
	ApiKey      string `gorm:"type:varchar(50)"`
	ModelName   string `gorm:"type:varchar(50)"`
	Provider    string `gorm:"type:varchar(50)"`
}

type RoleCache struct {
	sync.RWMutex
	Roles map[string][]*Role
}

type RoleScheduler struct {
	roles   []*Role
	current int
}

func NewRoleCache() *RoleCache {
	return &RoleCache{
		Roles: make(map[string][]*Role),
	}
}

func (rc *RoleCache) GetRoles(topicID string) []*Role {
	rc.RLock()
	defer rc.RUnlock()
	return rc.Roles[topicID]
}

func (rc *RoleCache) SetRoles(topicID string, roles []*Role) {
	rc.Lock()
	defer rc.Unlock()
	rc.Roles[topicID] = roles
}

func (rc *RoleCache) DeleteRoles(topicID string) {
	rc.Lock()
	defer rc.Unlock()
	delete(rc.Roles, topicID)
}

func (rs *RoleScheduler) NextRole() *Role {
	rs.current = (rs.current + 1) % len(rs.roles)
	return rs.roles[rs.current]
}

func (role *Role) Call(messages []*schema.Message, stream grpc.ServerStreamingServer[v1.StartTopicReply]) (*schema.Message, error) {
	cm, err := deepseek.NewChatModel(context.Background(), &deepseek.ChatModelConfig{
		APIKey: role.ApiKey,
		Model:  role.ModelName,
	})
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if messages == nil {
	}
	aiStream, err := cm.Stream(ctx, messages)
	if err != nil {
		return nil, err
	}
	message := &schema.Message{Role: schema.User}
	for {
		resp, err := aiStream.Recv()
		if err == io.EOF {
			return message, nil
		}
		if err != nil {
			return nil, err
		}

		if reasoning, ok := deepseek.GetReasoningContent(resp); ok {
			message.Content += reasoning
			stream.Send(&v1.StartTopicReply{
				Content: &v1.StartTopicReply_Reasoning{Reasoning: reasoning},
			})
		}

		if len(resp.Content) > 0 {
			message.Content += resp.Content
			stream.Send(&v1.StartTopicReply{
				Content: &v1.StartTopicReply_Text{Text: resp.Content},
			})
		}
	}
}
