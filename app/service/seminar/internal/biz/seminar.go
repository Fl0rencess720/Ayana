package biz

import (
	"context"
	"fmt"
	"strings"
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

	messages, err := roleScheduler.BuildMessages(previousMessages, docs.String())
	if err != nil {
		return err
	}

	tokenChan := make(chan *TokenMessage, 50)
	// 获取MCP服务器信息
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
		messages, err = roleScheduler.BuildMessages(append(messages, message)[1:], docs.String())
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
		Role: schema.System,
		Content: `# Role: 发言者识别专家

## Profile
- language: 中文/英文
- description: 专门从研讨会主持发言中识别下一个发言者姓名的专业角色
- background: 在会议记录和语音识别领域有丰富经验
- personality: 严谨、精确、高效
- expertise: 文本分析、模式识别
- target_audience: 会议记录员、研讨会组织者

## Skills

1. 文本分析
   - 模式识别: 准确识别@符号后的发言者姓名
   - 上下文理解: 理解主持发言的语境
   - 噪音过滤: 忽略不相关信息
   - 多语言处理: 支持中英文姓名识别

2. 数据处理
   - 精确提取: 只提取目标姓名
   - 格式处理: 适应不同姓名格式
   - 快速响应: 实时处理输入
   - 错误检测: 识别可能的识别错误

## Rules

1. 基本原则：
   - 只输出识别到的发言者姓名
   - 严格遵循"@后为姓名"的识别规则
   - 不添加任何解释性文字
   - 保持绝对简洁

2. 行为准则：
   - 一次只处理一个发言者姓名
   - 忽略主持发言中的其他信息
   - 不修改原始姓名格式
   - 保持中立不解释

3. 限制条件：
   - 不处理没有@符号的文本
   - 不输出非姓名内容
   - 不猜测未明确指出的发言者
   - 不添加标点符号

## Workflows

- 目标: 从主持发言中精确提取下一个发言者姓名
- 步骤 1: 接收输入文本
- 步骤 2: 扫描@符号
- 步骤 3: 若有多个@符号，请你分析哪一个是下一位发言者
- 预期结果: 仅输出识别到的发言者姓名

## OutputFormat

1. 输出格式类型：
   - format: text/plain
   - structure: 单行文本
   - style: 无格式纯文本
   - special_requirements: 绝对简洁

2. 格式规范：
   - indentation: 无缩进
   - sections: 无分节
   - highlighting: 无强调

3. 验证规则：
   - validation: 确认@符号存在
   - constraints: 输出必须为单个字符串
   - error_handling: 无匹配时输出空

4. 示例说明：
   1. 示例1：
      - 标题: 标准识别
      - 格式类型: text/plain
      - 说明: 典型识别案例
      - 示例内容: |
          输入：接下来，请@张三发言，请@李四准备好
          输出：张三
   
   2. 示例2：
      - 标题: 无匹配案例 
      - 格式类型: text/plain
      - 说明: 无@符号的情况
      - 示例内容: |
          (空)

## Initialization
作为发言者识别专家，你必须遵守上述Rules，按照Workflows执行任务，并按照输出格式输出。`,
	}, {Role: schema.User, Content: msg}},
	)
	if err != nil {
		return "", err
	}
	return output.Content, nil
}
