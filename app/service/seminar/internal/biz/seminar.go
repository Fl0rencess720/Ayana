package biz

import (
	"context"
	"fmt"
	"time"

	roleV1 "github.com/Fl0rencess720/Ayana/api/gateway/role/v1"
	"github.com/Fl0rencess720/Ayana/pkgs/utils"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type SeminarRepo interface {
	CreateTopic(ctx context.Context, phone string, topic *Topic) error
	DeleteTopic(ctx context.Context, topicUID string) error
	GetTopic(ctx context.Context, topicUID string) (*Topic, error)
	GetTopicsMetadata(ctx context.Context, phone string) ([]Topic, error)
	SaveSpeech(ctx context.Context, speech *Speech) error
	SaveSpeechToRedis(ctx context.Context, speech *Speech) error
	LockTopic(ctx context.Context, topicUID, lockerUID string) error
	UnlockTopic(topicUID, lockerUID string) error
	AddMCPServerToMysql(ctx context.Context, server *MCPServer) error
	GetMCPServersFromMysql(ctx context.Context, phone string) ([]MCPServer, error)
}

type SeminarUsecase struct {
	repo  SeminarRepo
	brepo BroadcastRepo
	log   *log.Helper

	roleClient roleV1.RoleManagerClient
	topicCache *TopicCache
	roleCache  *RoleCache
}

type MCPServer struct {
	ID            uint   `gorm:"primaryKey"`
	UID           string `gorm:"unique;index"`
	Name          string
	URL           string
	RequestHeader string
	Status        uint
	Phone         string `gorm:"index"`
}

type RoleType uint8

const (
	MODERATOR RoleType = iota
	PARTICIPANT
	UNKNOWN
)

func NewSeminarUsecase(repo SeminarRepo, brepo BroadcastRepo, topicCache *TopicCache,
	roleCache *RoleCache, roleClient roleV1.RoleManagerClient, logger log.Logger) *SeminarUsecase {
	s := &SeminarUsecase{repo: repo, brepo: brepo, topicCache: topicCache, roleCache: roleCache,
		roleClient: roleClient, log: log.NewHelper(logger)}
	go func() {
		if err := s.brepo.ReadPauseSignalFromKafka(context.Background()); err != nil {
			zap.L().Error("read pause signal from kafka failed", zap.Error(err))
		}
	}()
	return s
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

func (uc *SeminarUsecase) StartTopic(ctx context.Context, phone, topicUID string) error {
	// 分布式锁的value
	lockerUID := uuid.New().String()
	if err := uc.repo.LockTopic(ctx, topicUID, lockerUID); err != nil {
		zap.L().Error("lock topic failed", zap.Error(err))
	}
	defer uc.repo.UnlockTopic(topicUID, lockerUID)

	// 获取主题详情
	topic, err := uc.topicCache.GetTopic(topicUID)
	if err != nil {
		return err
	}
	if topic == nil {
		topic, err = uc.repo.GetTopic(context.Background(), topicUID)
		if err != nil {
			return err
		}
		topic.signalChan = make(chan StateSignal, 1)
		uc.topicCache.SetTopic(topic)
	}

	// 添加暂停信号通道
	if err := uc.brepo.AddTopicPauseChannel(ctx, topicUID, topic.signalChan); err != nil {
		zap.L().Error("add topic pause channel failed", zap.Error(err))
	}
	defer uc.brepo.DeleteTopicPauseContext(ctx, topicUID)

	// 获取该主题的所有角色
	rolesReply, err := uc.roleClient.GetModeratorAndParticipantsByUIDs(context.Background(), &roleV1.GetModeratorAndParticipantsByUIDsRequest{Phone: topic.Phone, Moderator: topic.Moderator, Uids: topic.Participants})
	if err != nil {
		return err
	}
	// 加载主持人
	moderator := &Role{
		Uid:         rolesReply.Moderator.Uid,
		RoleName:    rolesReply.Moderator.Name,
		Description: rolesReply.Moderator.Description,
		Avatar:      rolesReply.Moderator.Avatar,
		ApiPath:     rolesReply.Moderator.ApiPath,
		ApiKey:      rolesReply.Moderator.ApiKey,
		ModelName:   rolesReply.Moderator.Model.Name,
		RoleType:    MODERATOR,
	}
	// 加载参与者
	participants := []*Role{}
	for _, r := range rolesReply.Participants {
		participants = append(participants, &Role{
			Uid:         r.Uid,
			RoleName:    r.Name,
			Description: r.Description,
			Avatar:      r.Avatar,
			ApiPath:     r.ApiPath,
			ApiKey:      r.ApiKey,
			ModelName:   r.Model.Name,
			RoleType:    PARTICIPANT,
		})
	}

	//  将加载的所有角色添加到角色缓存中
	uc.roleCache.SetRoles(topicUID, append(participants, moderator))

	// 初始化角色调度器
	roleScheduler, err := NewRoleScheduler(topic, moderator, participants, uc.brepo)
	if err != nil {
		return err
	}

	previousMessages := []*schema.Message{{Role: schema.User, Content: "@研讨会管理员:研讨会的主题是---" + topic.Content + "。请主持人做好准备！"}}
	for i, speech := range topic.Speeches {
		content := buildMessageContent(speech)
		if i%2 == 0 {
			previousMessages = append(previousMessages, &schema.Message{
				Role:    schema.User,
				Content: content,
			})
		} else {
			previousMessages = append(previousMessages, &schema.Message{
				Role:    schema.Assistant,
				Content: content,
			})
		}
	}
	if len(topic.Speeches) > 0 {
		roleScheduler.NextRole(topic.Speeches[len(topic.Speeches)-1].Content)
	} else {
		roleScheduler.NextRole("")
	}

	messages, err := roleScheduler.state.buildMessages(roleScheduler, previousMessages)
	if err != nil {
		return err
	}

	tokenChan := make(chan *TokenMessage, 50)

	mcpservers, err := uc.repo.GetMCPServersFromMysql(ctx, phone)
	if err != nil {
		zap.L().Error("get mcp servers from mysql failed", zap.Error(err))
	}

	for {
		message, signal, err := roleScheduler.Call(messages, mcpservers, topic.signalChan, tokenChan)
		if err != nil {
			zap.L().Error("角色调用失败", zap.Error(err))
			return err
		}
		if signal == Pause {
			uc.topicCache.SetTopic(topic)
			break
		}
		newSpeech := Speech{
			Content:  message.Content,
			RoleUID:  roleScheduler.current.Uid,
			RoleName: roleScheduler.current.RoleName,
			TopicUID: topic.UID,
			Time:     time.Now(),
		}
		topic.Speeches = append(topic.Speeches, newSpeech)
		if err := uc.repo.SaveSpeech(context.Background(), &newSpeech); err != nil {
			return err
		}
		if err := roleScheduler.NextRole(message.Content); err != nil {
			return err
		}
		message.Content = buildMessageContent(newSpeech)
		messages, err = roleScheduler.BuildMessages(append(messages, message)[1:])
		if err != nil {
			return err
		}
	}
	return nil
}

func (uc *SeminarUsecase) StopTopic(ctx context.Context, topicID string) error {
	topic, err := uc.topicCache.GetTopic(topicID)
	if err != nil {
		return err
	}
	if err := uc.brepo.SendPauseSignalToKafka(ctx, topic.UID); err != nil {
		return err
	}
	return nil
}

func (uc *SeminarUsecase) AddMCPServer(ctx context.Context, phone, name, url, requestHeader string) error {
	uid, err := utils.GetSnowflakeID(0)
	if err != nil {
		return err
	}
	if err := uc.repo.AddMCPServerToMysql(ctx, &MCPServer{UID: uid, Name: name, URL: url, RequestHeader: requestHeader, Phone: phone}); err != nil {
		return err
	}
	return nil
}

func (uc *SeminarUsecase) GetMCPServers(ctx context.Context, phone string) ([]MCPServer, error) {
	servers, err := uc.repo.GetMCPServersFromMysql(ctx, phone)
	if err != nil {
		return nil, err
	}
	return servers, nil
}

func buildMessageContent(speech Speech) string {
	return fmt.Sprintf("@%s:%s", speech.RoleName, speech.Content)
}
func findNextRoleNameFromMessage(msg string) (string, error) {
	cm, err := deepseek.NewChatModel(context.Background(), &deepseek.ChatModelConfig{
		APIKey: viper.GetString("DEEPSEEK_API_KEY"),
		Model:  "deepseek-chat",
	})
	if err != nil {
		return "", err
	}
	output, err := cm.Generate(context.Background(), []*schema.Message{{
		Role:    schema.System,
		Content: "你将接收一个字符串，你需要从字符串中找出下一个角色的名字，大多数情况下角色名字会在@字符后，除了角色的名字以外不要输出任何其他内容。",
	}, {Role: schema.User, Content: msg}},
	)
	if err != nil {
		return "", err
	}
	return output.Content, nil
}
