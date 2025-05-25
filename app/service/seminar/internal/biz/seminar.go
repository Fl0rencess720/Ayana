package biz

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	roleV1 "github.com/Fl0rencess720/Ayana/api/gateway/role/v1"
	"github.com/Fl0rencess720/Ayana/pkgs/utils"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type SeminarRepo interface {
	CreateTopic(ctx context.Context, phone string, document []string, topic *Topic) error
	DeleteTopic(ctx context.Context, topicUID string) error
	GetTopic(ctx context.Context, topicUID string) (*Topic, error)
	GetTopicsMetadata(ctx context.Context, phone string) ([]Topic, error)
	SaveSpeech(ctx context.Context, speech *Speech) error
	SaveSpeechToRedis(ctx context.Context, speech *Speech) error
	LockTopic(ctx context.Context, topicUID, lockerUID string) error
	UnlockTopic(topicUID, lockerUID string) error
	AddMCPServerToMysql(ctx context.Context, server *MCPServer) error
	GetMCPServersFromMysql(ctx context.Context, phone string) ([]MCPServer, error)
	DeleteMCPServerFromMysql(ctx context.Context, phone, uid string) error
	EnableMCPServerInMysql(ctx context.Context, phone, uid string) error
	DisableMCPServerInMysql(ctx context.Context, phone, uid string) error
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
	Status        int32
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

func (uc *SeminarUsecase) CreateTopic(ctx context.Context, phone string, documents []string, topic *Topic) error {
	if err := uc.repo.CreateTopic(ctx, phone, documents, topic); err != nil {
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
		fmt.Printf("err: %v\n", err)
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

	// 检索相关文档段落
	var docs strings.Builder
	for _, document := range topic.Documents {
		documents, err := globalRAGUsecase.repo.RetrieveDocuments(ctx, topic.Content, document.UID)
		if err != nil {
			zap.L().Error("retrieve documents failed", zap.Error(err))
		}
		for _, doc := range documents {
			docs.WriteString(doc.Content)
			docs.WriteString("\n\n")
		}
	}

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
	for _, speech := range topic.Speeches {
		content := buildMessageContent(speech)
		previousMessages = append(previousMessages, &schema.Message{
			Role:    schema.Assistant,
			Content: content,
		})
	}
	if len(topic.Speeches) > 0 {
		roleScheduler.NextRole(topic.Speeches[len(topic.Speeches)-1].Content)
	} else {
		roleScheduler.NextRole("")
	}
	// 获取MCP服务器信息
	mcpservers, err := uc.repo.GetMCPServersFromMysql(ctx, phone)
	if err != nil {
		zap.L().Error("get mcp servers from mysql failed", zap.Error(err))
	}
	mcpBaseTools, mcpToolsInfo, err := getHealthyMCPServers(ctx, mcpservers)
	if err != nil {
		zap.L().Error("Error getting healthy MCP servers", zap.Error(err))
	}
	tokenChan := make(chan *TokenMessage, 50)

	tokenBuffer := NewTokenBuffer(10, 50*time.Millisecond)

	batchSender := func(sendCtx context.Context, tokens []*TokenMessage) error {
		return roleScheduler.brepo.SendTokensToKafkaBatch(sendCtx, roleScheduler.topic.UID, tokens)
	}

	newCtx := context.Background()

	tokenBuffer.Start(newCtx, batchSender)
	defer tokenBuffer.Stop()

	roleScheduler.msgs = previousMessages
	roleScheduler.mcpTools = mcpBaseTools
	roleScheduler.mcpToolsInfo = mcpToolsInfo
	roleScheduler.docs = docs.String()
	roleScheduler.tokenChan = tokenChan
	roleScheduler.TokenBuffer = tokenBuffer

	done := make(chan struct{})
	defer close(done)

	runner, err := uc.BuildGraph(ctx, roleScheduler, topic.signalChan)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return err
	}
	resultChan := make(chan error, 1)
	go func() {
		_, err := runner.Stream(newCtx, []*schema.Message{})
		resultChan <- err
	}()

	err = <-resultChan
	if err != nil {
		return err
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

func (uc *SeminarUsecase) CheckMCPServerHealth(ctx context.Context, url string) (int32, error) {
	health, err := checkMCPServerHealthByPing(MCPServer{URL: url})
	if err != nil {
		return 0, err
	}
	return health, nil
}

func (uc *SeminarUsecase) DeleteMCPServer(ctx context.Context, phone string, uid string) error {
	err := uc.repo.DeleteMCPServerFromMysql(ctx, phone, uid)
	if err != nil {
		return err
	}
	return nil
}

func (uc *SeminarUsecase) EnableMCPServer(ctx context.Context, phone, uid, url string) (int32, error) {
	status, err := uc.CheckMCPServerHealth(ctx, url)
	if err != nil {
		return 0, err
	}
	if err = uc.repo.EnableMCPServerInMysql(ctx, phone, uid); err != nil {
		return status, err
	}
	return status, nil
}

func (uc *SeminarUsecase) DisableMCPServer(ctx context.Context, phone string, uid string) error {
	err := uc.repo.DisableMCPServerInMysql(ctx, phone, uid)
	if err != nil {
		return err
	}
	return nil
}

func (uc *SeminarUsecase) BuildGraph(ctx context.Context, roleScheduler *RoleScheduler, signalChan <-chan StateSignal) (compose.Runnable[[]*schema.Message, *schema.Message], error) {
	// 为每个角色创建独立的模型实例
	moderatorModel, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey: roleScheduler.moderator.ApiKey,
		Model:  roleScheduler.moderator.ModelName,
	})
	if err != nil {
		return nil, err
	}

	participantModel, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey: roleScheduler.current.ApiKey,
		Model:  roleScheduler.current.ModelName,
	})
	if err != nil {
		return nil, err
	}

	moderatorModel.BindTools(roleScheduler.mcpToolsInfo)
	participantModel.BindTools(roleScheduler.mcpToolsInfo)

	g := compose.NewGraph[[]*schema.Message, *schema.Message](
		compose.WithGenLocalState(func(ctx context.Context) *RoleScheduler {
			return roleScheduler
		}))

	_ = g.AddPassthroughNode("decision")

	// 添加主持人节点
	_ = g.AddChatModelNode("moderator", moderatorModel,
		compose.WithStatePreHandler(
			func(ctx context.Context, input []*schema.Message, state *RoleScheduler) ([]*schema.Message, error) {
				// 确保当前角色是主持人
				if state.current.RoleType != MODERATOR {
					state.current = state.moderator
				}

				messages, err := state.BuildMessages(append(state.msgs, input...), roleScheduler.docs)
				if err != nil {
					return nil, err
				}

				return messages, nil
			}),
		compose.WithNodeName("moderator"))

	// 添加参与者节点
	_ = g.AddChatModelNode("participant", participantModel,
		compose.WithStatePreHandler(
			func(ctx context.Context, input []*schema.Message, state *RoleScheduler) ([]*schema.Message, error) {

				messages, err := state.BuildMessages(append(state.msgs, input...), roleScheduler.docs)
				if err != nil {
					return nil, err
				}

				return messages, nil
			}),
		compose.WithNodeName("participant"))

	// 添加转换节点
	_ = g.AddLambdaNode("moderatorToParticipant", compose.ToList[*schema.Message]())
	_ = g.AddLambdaNode("participantToModerator", compose.ToList[*schema.Message]())

	// 构建图的连接
	_ = g.AddEdge(compose.START, "decision")

	// 根据初始状态决定下一个节点
	_ = g.AddBranch("decision", compose.NewGraphBranch(func(ctx context.Context, in []*schema.Message) (string, error) {
		var next string
		err := compose.ProcessState(ctx, func(ctx context.Context, state *RoleScheduler) error {
			if state.current.RoleType == PARTICIPANT {
				next = "participant"
			} else {
				next = "moderator"
			}
			return nil
		})
		if err != nil {
			return "", err
		}
		return next, nil
	}, map[string]bool{"moderator": true, "participant": true}))

	// 连接转换节点
	_ = g.AddEdge("moderatorToParticipant", "participant")
	_ = g.AddEdge("participantToModerator", "moderator")

	// 主持人输出后的分支
	_ = g.AddBranch("moderator", compose.NewStreamGraphBranch(
		func(ctx context.Context, input *schema.StreamReader[*schema.Message]) (string, error) {
			// 收集完整输出
			state, err := compose.GetState[*RoleScheduler](ctx)
			if err != nil {
				return "", err
			}

			message := &schema.Message{Role: schema.Assistant}
			for {
				// 在每次循环开始时检查暂停信号
				select {
				case signal, ok := <-signalChan:
					if ok && signal == Pause {
						return "", compose.InterruptAndRerun
					}
				default:
					// 没有暂停信号，继续正常处理
				}

				resp, err := input.Recv()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					fmt.Printf("err: %v\n", err)
					return "", err
				}

				state.TokenBuffer.Add(&TokenMessage{
					Content: resp.Content,
					RoleUID: state.current.Uid,
				})

				// 处理 reasoning 内容
				if reasoning, ok := deepseek.GetReasoningContent(resp); ok && reasoning != "" {
					message.Content += reasoning
					token := &TokenMessage{
						TopicUID:    state.topic.UID,
						RoleUID:     state.current.Uid,
						ContentType: "reasoning",
						Content:     reasoning,
					}

					if state.TokenBuffer.Add(token) {
						state.TokenBuffer.Flush()
					}
				}

				// 处理主要内容
				if len(resp.Content) > 0 {
					message.Content += resp.Content
					token := &TokenMessage{
						TopicUID:    state.topic.UID,
						RoleUID:     state.current.Uid,
						ContentType: "text",
						Content:     resp.Content,
					}

					if state.TokenBuffer.Add(token) {
						state.TokenBuffer.Flush()
					}
				}
			}

			speech := Speech{
				TopicUID: roleScheduler.topic.UID,
				RoleUID:  state.current.Uid,
				Content:  message.Content,
				RoleName: state.current.RoleName,
				Time:     time.Now(),
			}
			if err = uc.repo.SaveSpeech(ctx, &speech); err != nil {
				return "", err
			}
			roleScheduler.topic.Speeches = append(roleScheduler.topic.Speeches, speech)
			uc.topicCache.SetTopic(roleScheduler.topic)

			// 更新状态
			var next string
			err = compose.ProcessState(ctx, func(ctx context.Context, state *RoleScheduler) error {
				// 添加主持人的回复到消息历史
				state.msgs = append(state.msgs, &schema.Message{
					Role:    schema.Assistant,
					Content: message.Content,
				})

				// 切换到参与者
				err := state.NextRole(message.Content)
				if err != nil {
					return err
				}

				next = "moderatorToParticipant"
				return nil
			})

			return next, err
		},
		map[string]bool{compose.END: true, "moderatorToParticipant": true}))

	// 参与者输出后的分支
	_ = g.AddBranch("participant", compose.NewStreamGraphBranch(
		func(ctx context.Context, input *schema.StreamReader[*schema.Message]) (string, error) {
			// 收集完整输出
			state, err := compose.GetState[*RoleScheduler](ctx)
			if err != nil {
				return "", err
			}

			message := &schema.Message{Role: schema.Assistant}
			for {
				select {
				case signal, ok := <-signalChan:
					if ok && signal == Pause {

						return "", compose.InterruptAndRerun
					}
				default:
					// 没有暂停信号，继续正常处理
				}
				resp, err := input.Recv()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					return "", err
				}
				state.TokenBuffer.Add(&TokenMessage{
					Content: resp.Content,
					RoleUID: state.current.Uid,
				})
				// 处理 reasoning 内容
				if reasoning, ok := deepseek.GetReasoningContent(resp); ok && reasoning != "" {
					message.Content += reasoning
					token := &TokenMessage{
						TopicUID:    state.topic.UID,
						RoleUID:     state.current.Uid,
						ContentType: "reasoning",
						Content:     reasoning,
					}

					if state.TokenBuffer.Add(token) {
						state.TokenBuffer.Flush()
					}
				}

				// 处理主要内容
				if len(resp.Content) > 0 {
					message.Content += resp.Content
					token := &TokenMessage{
						TopicUID:    state.topic.UID,
						RoleUID:     state.current.Uid,
						ContentType: "text",
						Content:     resp.Content,
					}

					if state.TokenBuffer.Add(token) {
						state.TokenBuffer.Flush()
					}
				}

			}
			speech := Speech{
				TopicUID: roleScheduler.topic.UID,
				RoleUID:  state.current.Uid,
				Content:  message.Content,
				RoleName: state.current.RoleName,
				Time:     time.Now(),
			}
			if err = uc.repo.SaveSpeech(ctx, &speech); err != nil {
				return "", err
			}
			roleScheduler.topic.Speeches = append(roleScheduler.topic.Speeches, speech)
			uc.topicCache.SetTopic(roleScheduler.topic)
			// 更新状态
			var next string
			err = compose.ProcessState(ctx, func(ctx context.Context, state *RoleScheduler) error {
				// 添加参与者的回复到消息历史
				state.msgs = append(state.msgs, &schema.Message{
					Role:    schema.Assistant,
					Content: message.Content,
				})

				// 切换到主持人
				err := state.NextRole(message.Content)
				if err != nil {
					return err
				}

				// 检查是否应该结束对话

				next = "participantToModerator"

				return nil
			})

			return next, err
		},
		map[string]bool{compose.END: true, "participantToModerator": true}))

	runner, err := g.Compile(ctx, compose.WithMaxRunSteps(100))
	if err != nil {
		return nil, err
	}
	return runner, nil
}
