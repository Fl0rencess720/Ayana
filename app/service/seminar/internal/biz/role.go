package biz

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"

	v1 "github.com/Fl0rencess720/Ayana/api/gateway/seminar/v1"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type Role struct {
	gorm.Model
	Phone       string   `gorm:"type:varchar(50);primaryKey"`
	Uid         string   `gorm:"type:varchar(50)"`
	RoleName    string   `gorm:"type:varchar(50)"`
	RoleType    RoleType `gorm:"type:enum(0, 1)"`
	Description string   `gorm:"type:text"`
	Avatar      string   `gorm:"type:varchar(50)"`
	ApiPath     string   `gorm:"type:varchar(50)"`
	ApiKey      string   `gorm:"type:varchar(50)"`
	ModelName   string   `gorm:"type:varchar(50)"`
	Provider    string   `gorm:"type:varchar(50)"`
}

type RoleCache struct {
	sync.RWMutex
	Roles map[string][]*Role
}

type RoleScheduler struct {
	moderator      *Role
	roles          []*Role
	current        *Role
	currentRoleIdx int
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

func (rs *RoleScheduler) NextRole() (*Role, RoleType) {
	if rs.current.RoleType != MODERATOR {
		rs.currentRoleIdx = -1
		rs.current = rs.moderator
		return rs.moderator, MODERATOR
	}

	if rs.currentRoleIdx == -1 {
		if len(rs.roles) == 0 {
			return rs.moderator, MODERATOR
		}
		randomIndex := rand.Intn(len(rs.roles))
		rs.current = rs.roles[randomIndex]
		rs.currentRoleIdx = randomIndex
		return rs.current, PARTICIPANT
	}

	pre := rs.roles[:rs.currentRoleIdx]
	tail := []*Role{}
	if rs.currentRoleIdx+1 < len(rs.roles) {
		tail = rs.roles[rs.currentRoleIdx+1:]
	}
	selectingRoles := append(pre, tail...)

	if len(selectingRoles) == 0 {
		rs.currentRoleIdx = -1
		rs.current = rs.moderator
		return rs.moderator, MODERATOR
	}

	randomIndex := rand.Intn(len(selectingRoles))
	originalIdx := rs.currentRoleIdx
	if randomIndex >= originalIdx {
		randomIndex++
	}
	rs.current = selectingRoles[randomIndex]
	rs.currentRoleIdx = randomIndex
	return rs.current, PARTICIPANT
}

func (role *Role) Call(messages []*schema.Message, stream grpc.ServerStreamingServer[v1.StreamOutputReply], signalChan chan StateSignal) (*schema.Message, StateSignal, error) {
	ctx := context.Background()
	ctxFromPausing, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if signal, ok := <-signalChan; ok {
			if signal == Pause {
				cancel()
				return
			}
			if signal == Normal {
				return
			}
		}
	}()

	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey: role.ApiKey,
		Model:  role.ModelName,
	})
	if err != nil {
		return nil, Error, err
	}

	aiStream, err := cm.Stream(ctx, messages)
	if err != nil {
		return nil, Error, err
	}
	defer aiStream.Close()

	message := &schema.Message{Role: schema.User}
	resultChan := make(chan struct {
		*schema.Message
		StateSignal
		error
	}, 1)
	go func() {
		defer close(resultChan)
		for {
			resp, err := aiStream.Recv()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					resultChan <- struct {
						*schema.Message
						StateSignal
						error
					}{nil, Pause, nil}
					return
				}
				if errors.Is(err, io.EOF) {
					resultChan <- struct {
						*schema.Message
						StateSignal
						error
					}{message, Normal, nil}
					return
				}
				resultChan <- struct {
					*schema.Message
					StateSignal
					error
				}{nil, Error, fmt.Errorf("stream error: %w", err)}
				return
			}
			if reasoning, ok := deepseek.GetReasoningContent(resp); ok {
				message.Content += reasoning
				if sendErr := stream.Send(&v1.StreamOutputReply{
					RoleUID: role.Uid,
					Content: &v1.StreamOutputReply_Reasoning{Reasoning: reasoning},
				}); sendErr != nil {
					resultChan <- struct {
						*schema.Message
						StateSignal
						error
					}{nil, Error, sendErr}
					return
				}
			}
			if len(resp.Content) > 0 {
				message.Content += resp.Content
				if sendErr := stream.Send(&v1.StreamOutputReply{
					RoleUID: role.Uid,
					Content: &v1.StreamOutputReply_Text{Text: resp.Content},
				}); sendErr != nil {
					resultChan <- struct {
						*schema.Message
						StateSignal
						error
					}{nil, Error, sendErr}
					return
				}
			}
		}
	}()
	select {
	case <-ctxFromPausing.Done():
		return nil, Pause, nil
	case result := <-resultChan:
		signalChan <- Normal
		return result.Message, result.StateSignal, result.error
	}
}
