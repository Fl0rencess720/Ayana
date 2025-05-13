package biz

import (
	"context"
	"fmt"
	"time"

	roleV1 "github.com/Fl0rencess720/Ayana/api/gateway/role/v1"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/components/prompt"
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
}

type SeminarUsecase struct {
	repo  SeminarRepo
	brepo BroadcastRepo
	log   *log.Helper

	roleClient roleV1.RoleManagerClient
	topicCache *TopicCache
	roleCache  *RoleCache
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

func (uc *SeminarUsecase) StartTopic(ctx context.Context, topicUID string) error {
	lockerUID := uuid.New().String()
	if err := uc.repo.LockTopic(ctx, topicUID, lockerUID); err != nil {
		zap.L().Error("lock topic failed", zap.Error(err))
	}
	defer uc.repo.UnlockTopic(topicUID, lockerUID)

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

	if err := uc.brepo.AddTopicPauseChannel(ctx, topicUID, topic.signalChan); err != nil {
		zap.L().Error("add topic pause channel failed", zap.Error(err))
	}
	defer uc.brepo.DeleteTopicPauseContext(ctx, topicUID)

	rolesReply, err := uc.roleClient.GetRolesAndModeratorByUIDs(context.Background(), &roleV1.GetRolesAndModeratorByUIDsRequest{Phone: topic.Phone, Moderator: topic.Moderator, Uids: topic.Participants})
	if err != nil {
		return err
	}
	// 加载角色
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
			RoleType:    PARTICIPANT,
		})
	}
	// key为name，value为Role
	mroles := make(map[string]*Role)
	for _, r := range roles {
		mroles[r.RoleName] = r
	}
	nroles := []string{}
	for k := range mroles {
		nroles = append(nroles, k)
	}
	uc.roleCache.SetRoles(topicUID, roles)

	// currentRole := &Role{RoleType: UNKNOWN}
	// currentRoleIdx := -1
	role := moderator
	roleType := MODERATOR
	if len(topic.Speeches) != 0 {
		lastRoleUID := topic.Speeches[len(topic.Speeches)-1].RoleUID
		msg := topic.Speeches[len(topic.Speeches)-1].Content
		if lastRoleUID == moderator.Uid {
			nextRoleName, err := findNextRoleNameFromMessage(msg)
			if err != nil {
				return err
			}
			role = mroles[nextRoleName]
			roleType = PARTICIPANT
		}
	}
	// 角色调度器
	roleScheduler := NewRoleScheduler(topicUID, moderator, roles, role, -1, uc.brepo)

	// role, roleType := roleScheduler.NextRole()
	roleScheduler.current = role

	topic.State = &PreparingState{}
	topic.State.nextState(topic)

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
	messages, err := buildMessages(roleType, moderator, nroles, previousMessages)
	if err != nil {
		return err
	}
	tokenChan := make(chan *TokenMessage, 50)

	for {
		message, signal, err := roleScheduler.Call(messages, topic.signalChan, tokenChan)
		if err != nil {
			fmt.Printf("err: %v\n", err)
			zap.L().Error("角色调用失败", zap.Error(err))
			return err
		}
		if signal == Pause {
			topic.State.nextState(topic)
			uc.topicCache.SetTopic(topic)
			break
		}
		newSpeech := Speech{
			Content:  message.Content,
			RoleUID:  role.Uid,
			RoleName: role.RoleName,
			TopicUID: topic.UID,
			Time:     time.Now(),
		}
		topic.Speeches = append(topic.Speeches, newSpeech)
		if err := uc.repo.SaveSpeech(context.Background(), &newSpeech); err != nil {
			return err
		}
		nextRoleName := ""
		if roleType == MODERATOR {
			nextRoleName, err = findNextRoleNameFromMessage(message.Content)
			if err != nil {
				return err
			}
			role = mroles[nextRoleName]
			roleType = PARTICIPANT
		} else {
			// role, roleType = roleScheduler.NextRole()
			role = moderator
			roleType = MODERATOR
		}
		roleScheduler.current = role
		message.Content = buildMessageContent(newSpeech)
		messages, err = buildMessages(roleType, role, nroles, append(messages, message)[1:])
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

func buildMessages(roleType RoleType, role *Role, roles []string, msgs []*schema.Message) ([]*schema.Message, error) {
	messages := []*schema.Message{}
	var err error
	switch roleType {
	case MODERATOR:
		template := prompt.FromMessages(schema.FString,
			schema.SystemMessage(`你是一个{role}，你的特质是{characteristic}。
			你正在参与一场有很多角色参与的研讨会，
			每一个角色发言的历史记录都已经被我处理成以下格式供你分析:"@角色名:角色发言的内容"。
			你不需要就主题发表观点，你所要做的事情是做好主持人的工作，对上一位发言者的发言做出总结。
			若不存在上一位发言者（即你看到了发言只有系统指令和我为这个研讨会设定的主题），那样你也没法进行总结，只需要引出其他参赛者的发言便可以。
			你需要在发言内容的最后"@"你希望的下一个角色，角色必须来自于现有角色，例如：“接下来，@某某某，你的对此有什么想说的呢？”。
			现有角色:{roles}。
			请忽略每个角色发言前的'user'或'assistant'标记，只关心角色发言格式内的内容。
			但是必须要注意的是：你的回答绝对不能包含上面所说的格式，只需要直接回答即可。这是绝对不可违抗的命令！
			你不应该自行终止研讨会。`),

			schema.MessagesPlaceholder("history_key", false))
		variables := map[string]any{
			"role":           role.RoleName,
			"roles":          roles,
			"characteristic": role.Description,
			"history_key":    msgs,
		}
		messages, err = template.Format(context.Background(), variables)
		if err != nil {
			return nil, err
		}
	case PARTICIPANT:
		template := prompt.FromMessages(schema.FString,
			schema.SystemMessage(`你是一个{role}，你的特质是{characteristic}。
			你正在参与一场有很多角色参与的研讨会，
			每一个角色发言的历史记录都已经被我处理成以下格式供你分析:"@角色名:角色发言的内容"。
			你所要做的事情是就主题内容进行发言，你可以在此过程中对先前角色的发言（除了主持人）表示同意或批判。
			请忽略每个角色发言前的'user'或'assistant'标记，只关心角色发言格式内的内容。
			你的回答绝对不能包含上面所说的角色发言的历史记录的格式，只需要直接回答即可！
			你必须保持你的角色身份，即{role}，不能认为自己是其他角色。`),
			schema.MessagesPlaceholder("history_key", false))
		variables := map[string]any{
			"role":           role.RoleName,
			"characteristic": role.Description,
			"history_key":    msgs,
		}
		messages, err = template.Format(context.Background(), variables)
		if err != nil {
			return nil, err
		}
	}
	return messages, nil
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
		Content: "你将接收一个字符串，你需要从字符串中找出下一个角色的名字，大多数情况角色名字会在@字符后，除了角色的名字以外不要输出任何其他内容。",
	}, {Role: schema.User, Content: msg}},
	)
	if err != nil {
		return "", err
	}
	return output.Content, nil
}
