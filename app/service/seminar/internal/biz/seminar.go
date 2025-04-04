package biz

import (
	"context"
	"time"

	roleV1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/role/v1"
	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
)

type SeminarRepo interface {
	CreateTopic(ctx context.Context, phone string, topic *Topic) error
	DeleteTopic(ctx context.Context, topicUID string) error
	GetTopic(ctx context.Context, topicUID string) (*Topic, error)
	GetTopicsMetadata(ctx context.Context, phone string) ([]Topic, error)
}

type SeminarUsecase struct {
	repo SeminarRepo
	log  *log.Helper

	roleClient roleV1.RoleManagerClient
	topicCache *TopicCache
	roleCache  *RoleCache
}

func NewSeminarUsecase(repo SeminarRepo, topicCache *TopicCache, roleCache *RoleCache, roleClient roleV1.RoleManagerClient, logger log.Logger) *SeminarUsecase {
	return &SeminarUsecase{repo: repo, topicCache: topicCache, roleCache: roleCache, roleClient: roleClient, log: log.NewHelper(logger)}
}

func (uc *SeminarUsecase) CreateTopic(ctx context.Context, phone string, topic *Topic) error {
	if err := uc.repo.CreateTopic(ctx, phone, topic); err != nil {
		return err
	}
	uc.topicCache.SetTopic(topic)
	return nil
}

func (uc *SeminarUsecase) DeleteTopic(ctx context.Context, topicUID string) error {
	if err := uc.repo.DeleteTopic(ctx, topicUID); err != nil {
		return err
	}
	return nil
}

func (uc *SeminarUsecase) GetTopic(ctx context.Context, topicUID string) (Topic, error) {
	topic, err := uc.repo.GetTopic(ctx, topicUID)
	if err != nil {
		return Topic{}, err
	}
	return *topic, nil
}

func (uc *SeminarUsecase) GetTopicsMetadata(ctx context.Context, phone string) ([]Topic, error) {
	topics, err := uc.repo.GetTopicsMetadata(ctx, phone)
	if err != nil {
		return nil, err
	}
	return topics, nil
}

func (uc *SeminarUsecase) StartTopic(topicID string, stream grpc.ServerStreamingServer[v1.StreamOutputReply]) error {
	topic, err := uc.topicCache.GetTopic(topicID)
	if err != nil {
		return err
	}
	if topic == nil {
		topic, err = uc.repo.GetTopic(context.Background(), topicID)
		if err != nil {
			return err
		}
		topic.signalChan = make(chan StateSignal, 1)
		uc.topicCache.SetTopic(topic)
	}
	rolesReply, err := uc.roleClient.GetRolesByUIDs(context.Background(), &roleV1.GetRolesByUIDsRequest{Phone: topic.Phone, Uids: topic.Participants})
	if err != nil {
		return err
	}
	roles := []*Role{}
	for _, r := range rolesReply.Roles {
		roles = append(roles, &Role{
			Uid:         r.Uid,
			RoleName:    r.Name,
			Description: r.Description,
			Avatar:      r.Avatar,
			ApiPath:     r.ApiPath,
			ApiKey:      r.ApiKey,
			ModelName:   r.Model.Name,
		})
	}
	uc.roleCache.SetRoles(topicID, roles)
	roleScheduler := RoleScheduler{roles: roles}
	role := roleScheduler.NextRole()
	topic.State = &PreparingState{}
	topic.State.nextState(topic)

	messages := []*schema.Message{}
	messages = append(messages, &schema.Message{
		Role:    schema.User,
		Content: topic.Content,
	})
	for {
		message, signal, err := role.Call(messages, stream, topic.signalChan)
		if err != nil {
			return err
		}
		if signal == Pause {
			topic.State.nextState(topic)
			uc.topicCache.SetTopic(topic)
			break
		}
		messages = append(messages, message)
		topic.Speeches = append(topic.Speeches, Speech{
			Content:  message.Content,
			RoleUID:  role.Uid,
			TopicUID: topic.UID,
			Time:     time.Now(),
		})
		role = roleScheduler.NextRole()
	}
	return nil
}

func (uc *SeminarUsecase) StopTopic(ctx context.Context, topicID string) error {
	topic, err := uc.topicCache.GetTopic(topicID)
	if err != nil {
		return err
	}
	topic.signalChan <- Pause
	return nil
}
