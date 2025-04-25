package biz

import (
	"context"
	"fmt"
	"time"

	roleV1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/role/v1"
	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
)

type SeminarRepo interface {
	CreateTopic(ctx context.Context, phone string, topic *Topic) error
	DeleteTopic(ctx context.Context, topicUID string) error
	GetTopic(ctx context.Context, topicUID string) (*Topic, error)
	GetTopicsMetadata(ctx context.Context, phone string) ([]Topic, error)
	SaveSpeech(ctx context.Context, speech *Speech) error
	SaveSpeechToRedis(ctx context.Context, speech *Speech) error
}

type SeminarUsecase struct {
	repo SeminarRepo
	log  *log.Helper

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

	uc.roleCache.SetRoles(topicID, roles)

	currentRole := &Role{RoleType: UNKNOWN}
	currentRoleIdx := -1
	if len(topic.Speeches) != 0 {
		currentUID := topic.Speeches[len(topic.Speeches)-1].RoleUID
		if currentUID == moderator.Uid {
			currentRole = moderator
		} else {
			if len(topic.Speeches) > 1 {
				currentUID = topic.Speeches[len(topic.Speeches)-2].RoleUID
			}
			for i, role := range roles {
				if role.Uid == currentUID {
					currentRole = role
					currentRoleIdx = i
					break
				}
			}
		}
	}
	roleScheduler := RoleScheduler{moderator: moderator, roles: roles, current: currentRole, currentRoleIdx: currentRoleIdx}

	role, roleType := roleScheduler.NextRole()
	topic.State = &PreparingState{}
	topic.State.nextState(topic)

	previousMessages := []*schema.Message{{Role: schema.User, Content: topic.Content}}
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
	messages, err := buildMessages(roleType, moderator, previousMessages)
	if err != nil {
		return err
	}
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
		role, roleType = roleScheduler.NextRole()
		message.Content = buildMessageContent(newSpeech)
		messages, err = buildMessages(roleType, role, append(messages, message)[1:])
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
	topic.signalChan <- Pause
	return nil
}

func buildMessages(roleType RoleType, role *Role, msgs []*schema.Message) ([]*schema.Message, error) {
	messages := []*schema.Message{}
	var err error
	switch roleType {
	case MODERATOR:
		template := prompt.FromMessages(schema.FString,
			schema.SystemMessage(`你是一个{role}，你的特质是{characteristic}。
			你正在参与一场有很多角色参与的研讨会，
			每一个角色发言的历史记录都已经被我处理成以下格式供你分析:"角色名：角色发言的内容"。
			你所要做的事情是做好主持人的工作，对上一位发言者的发言做出总结。
			若不存在上一位发言者（即你看到了发言只有系统指令和我为这个研讨会设定的主题），那样你也没法进行总结，只需要引出其他参赛者的发言便可以。
			请忽略每个角色发言前的'user'或'assistant'标记，只关心角色发言格式内的内容。
			但是必须要注意的是：你的回答绝对不能包含上面所说的格式，只需要直接回答即可。这是绝对不可违抗的命令！`),
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
	case PARTICIPANT:
		template := prompt.FromMessages(schema.FString,
			schema.SystemMessage(`你是一个{role}，你的特质是{characteristic}。
			你正在参与一场有很多角色参与的研讨会，
			每一个角色发言的历史记录都已经被我处理成以下格式供你分析:"角色名：角色发言的内容"。
			你所要做的事情是做好就主题内容进行发言，你可以在此过程中对先前角色的发言（除了主持人）表示同意或批判。
			请忽略每个角色发言前的'user'或'assistant'标记，只关心角色发言格式内的内容。
			但是必须要注意的是：你的回答绝对不能包含上面所说的角色发言的历史记录的格式，只需要直接回答即可！`),
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
	return fmt.Sprintf("[role_name:'%s',content:'%s']", speech.RoleName, speech.Content)
}
