package biz

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
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
	topicUID     string
	moderator    *Role
	participants []*Role
	current      *Role
	roleMap      map[string]*Role
	roleNames    []string
	state        RoleState
	brepo        BroadcastRepo
}

func NewRoleCache() *RoleCache {
	return &RoleCache{
		Roles: make(map[string][]*Role),
	}
}

func NewRoleScheduler(topic *Topic, moderator *Role, participants []*Role, brepo BroadcastRepo) (*RoleScheduler, error) {

	// 构建 参与者名->参与者实例 的映射
	// key为name，value为Role
	roleMap := make(map[string]*Role)
	for _, p := range participants {
		roleMap[p.RoleName] = p
	}
	// 构建参与者名切片，用于后续选取下一个参与者
	roleNames := []string{}
	for _, p := range participants {
		roleNames = append(roleNames, p.RoleName)
	}

	lastRole := &Role{RoleType: UNKNOWN}
	var state RoleState
	if len(topic.Speeches) != 0 {
		lastRoleUID := topic.Speeches[len(topic.Speeches)-1].RoleUID
		lastRoleName := topic.Speeches[len(topic.Speeches)-1].RoleName
		if lastRoleUID == moderator.Uid {
			lastRole = moderator
			state = ModeratorState{}
		} else {
			lastRole = roleMap[lastRoleName]
			state = ParticipantState{}
		}
	} else {
		state = UnknownState{}
	}

	return &RoleScheduler{
		topicUID:     topic.UID,
		moderator:    moderator,
		participants: participants,
		roleMap:      roleMap,
		roleNames:    roleNames,
		state:        state,
		current:      lastRole,
		brepo:        brepo,
	}, nil
}

type TokenBuffer struct {
	messages    []*TokenMessage
	mu          sync.Mutex
	batchSize   int
	flushTicker *time.Ticker
	sendChan    chan []*TokenMessage
	done        chan struct{}
	closeOnce   sync.Once
}

func NewTokenBuffer(batchSize int, flushInterval time.Duration) *TokenBuffer {
	tb := &TokenBuffer{
		messages:    make([]*TokenMessage, 0, batchSize*2),
		batchSize:   batchSize,
		flushTicker: time.NewTicker(flushInterval),
		sendChan:    make(chan []*TokenMessage, 10),
		done:        make(chan struct{}),
	}

	return tb
}

func (tb *TokenBuffer) Add(token *TokenMessage) bool {
	tb.mu.Lock()
	tb.messages = append(tb.messages, token)
	needFlush := len(tb.messages) >= tb.batchSize
	tb.mu.Unlock()

	return needFlush
}

func (tb *TokenBuffer) Flush() {
	tb.mu.Lock()
	if len(tb.messages) == 0 {
		tb.mu.Unlock()
		return
	}

	toSend := make([]*TokenMessage, len(tb.messages))
	copy(toSend, tb.messages)
	tb.messages = tb.messages[:0]
	tb.mu.Unlock()

	select {
	case tb.sendChan <- toSend:
	case <-tb.done:
	}
}

func (tb *TokenBuffer) safeCloseSendChan() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	select {
	case <-tb.done:
	default:
		close(tb.sendChan)
	}
}

func (tb *TokenBuffer) safeCloseDone() {
	tb.closeOnce.Do(func() {
		close(tb.done)
	})
}

func (tb *TokenBuffer) Start(ctx context.Context, sender func(context.Context, []*TokenMessage) error) {
	go func() {
		defer tb.flushTicker.Stop()

		for {
			select {
			case <-tb.flushTicker.C:
				tb.Flush()
			case batch, ok := <-tb.sendChan:
				if !ok {
					// sendChan已关闭，退出循环
					return
				}

				// 异步发送到Kafka，如果出错也继续处理
				go func(b []*TokenMessage) {
					if err := sender(ctx, b); err != nil {
						log.Printf("Error sending batch to Kafka: %v", err)
					}
				}(batch)
			case <-tb.done:
				// 关闭时发送所有剩余消息
				tb.Flush()

				// 安全关闭发送通道
				tb.safeCloseSendChan()

				// 处理通道中剩余的批次
				for batch := range tb.sendChan {
					if err := sender(ctx, batch); err != nil {
						log.Printf("Error sending final batch to Kafka: %v", err)
					}
				}
				return
			case <-ctx.Done():
				// 仅触发done信号，不直接关闭通道
				tb.safeCloseDone()
			}
		}
	}()
}

// Stop 停止后台处理
func (tb *TokenBuffer) Stop() {
	// 安全地关闭done通道
	tb.safeCloseDone()
}

// BatchSendTokensToKafka 发送一批token到Kafka的函数类型
type BatchSendTokensToKafka func(context.Context, []*TokenMessage) error

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

// 获取当前状态
func (rs *RoleScheduler) getState() RoleState {
	return rs.state
}

// 设置新状态
func (rs *RoleScheduler) setState(state RoleState) {
	rs.state = state
}
func (rs *RoleScheduler) NextRole(msgContent string) error {
	role, err := rs.state.nextRole(rs, msgContent)
	if err != nil {
		return err
	}
	rs.current = role
	return nil
}

func (rs *RoleScheduler) BuildMessages(msgs []*schema.Message) ([]*schema.Message, error) {
	return rs.state.buildMessages(rs, msgs)
}

// Call 负责初始化资源并处理整个对话流程
func (rs *RoleScheduler) Call(messages []*schema.Message, mcpservers []MCPServer, signalChan chan StateSignal, tokenChan chan *TokenMessage) (*schema.Message, StateSignal, error) {
	role := rs.current

	ctx := context.Background()

	pauseCtx, pauseCancel := context.WithCancel(ctx)

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case signal, ok := <-signalChan:
			if ok && signal == Pause {
				pauseCancel() // 只取消暂停检测的 context
			}
		case <-done:
			return
		}
	}()

	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey: role.ApiKey,
		Model:  role.ModelName,
	})
	if err != nil {
		return nil, Error, err
	}

	mcpTools, mcpToolsInfo, err := getHealthyMCPServers(ctx, mcpservers)
	if err != nil {
		zap.L().Error("Error getting healthy MCP servers", zap.Error(err))
	}

	if len(mcpToolsInfo) > 0 {
		if err := cm.BindTools(mcpToolsInfo); err != nil {
			zap.L().Error("Error binding tools to chat model", zap.Error(err))
		}
	}

	tokenBuffer := NewTokenBuffer(10, 50*time.Millisecond)

	batchSender := func(sendCtx context.Context, tokens []*TokenMessage) error {
		return rs.brepo.SendTokensToKafkaBatch(sendCtx, rs.topicUID, tokens)
	}

	tokenBuffer.Start(ctx, batchSender)
	defer tokenBuffer.Stop()

	resultChan := make(chan struct {
		*schema.Message
		StateSignal
		error
	}, 1)

	go func() {
		msg, signal, err := rs.processConversation(
			ctx, pauseCtx, done, cm, mcpTools,
			messages, tokenBuffer,
		)
		resultChan <- struct {
			*schema.Message
			StateSignal
			error
		}{msg, signal, err}
	}()

	select {
	case <-pauseCtx.Done():
		return nil, Pause, nil
	case result := <-resultChan:
		if result.StateSignal == Normal {
			select {
			case signalChan <- Normal:
			default:
			}
		}
		return result.Message, result.StateSignal, result.error
	}
}

func getHealthyMCPServers(ctx context.Context, mcpServers []MCPServer) ([]tool.BaseTool, []*schema.ToolInfo, error) {
	tools := []tool.BaseTool{}
	toolsInfo := []*schema.ToolInfo{}
	for _, mcpServer := range mcpServers {
		if mcpServer.Status == 0 {
			continue
		}
		cli, err := client.NewSSEMCPClient(mcpServer.URL)
		if err != nil {
			zap.L().Error("failed to create MCP client", zap.Error(err))
			continue
		}

		if err = cli.Start(ctx); err != nil {
			zap.L().Error("failed to start MCP client", zap.Error(err))
			continue
		}

		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{
			Name:    "eino-llm-app",
			Version: "1.0.0",
		}

		_, err = cli.Initialize(ctx, initRequest)
		if err != nil {
			fmt.Printf("err: %v\n", err)
			zap.L().Error("failed to initialize MCP client", zap.Error(err))
			continue
		}

		tools, err := mcpp.GetTools(ctx, &mcpp.Config{
			Cli: cli,
		})
		if err != nil {
			fmt.Printf("err: %v\n", err)
			zap.L().Error("failed to get tools", zap.Error(err))
		}
		for _, tool := range tools {
			t, err := tool.Info(ctx)
			if err != nil {
				zap.L().Error("failed to get tool info", zap.Error(err))
				continue
			}
			tools = append(tools, tool)
			toolsInfo = append(toolsInfo, t)
		}
	}
	return tools, toolsInfo, nil
}

func invoke(ctx context.Context, toolCalls []schema.ToolCall, mcpTools []tool.BaseTool) []*schema.Message {
	toolsReply := []*schema.Message{}
	for _, toolCall := range toolCalls {
		var selectedTool tool.InvokableTool
		for _, t := range mcpTools {
			info, _ := t.Info(ctx)
			if info.Name == toolCall.Function.Name {
				if invokable, ok := t.(tool.InvokableTool); ok {
					selectedTool = invokable
					break
				}
			}
		}

		if selectedTool != nil {
			fmt.Println("正在调用工具：", toolCall.Function.Name)
			result, err := selectedTool.InvokableRun(ctx, toolCall.Function.Arguments)
			if err != nil {
				zap.L().Error("工具执行失败", zap.Error(err))
				continue
			}
			toolsReply = append(toolsReply, schema.ToolMessage(result, toolCall.ID))
		}
	}
	return toolsReply
}

// processConversation 处理与AI模型的对话流程，包括工具调用
func (rs *RoleScheduler) processConversation(
	ctx context.Context, // 主上下文
	pauseCtx context.Context, // 暂停监听上下文
	done chan struct{}, // 主函数结束信号
	cm *deepseek.ChatModel,
	mcpTools []tool.BaseTool,
	messages []*schema.Message,
	tokenBuffer *TokenBuffer,
) (*schema.Message, StateSignal, error) {
	// 准备结果消息
	message := &schema.Message{Role: schema.User}

	// 开始处理对话
	currentMessages := messages

	for {
		// 每个循环创建一个可取消的上下文，用于控制单次流处理
		streamCtx, streamCancel := context.WithCancel(ctx)

		// 创建监听器以响应暂停
		go func() {
			select {
			case <-pauseCtx.Done():
				streamCancel() // 暂停时取消当前流处理
			case <-done:
				return // 主函数结束，退出该 goroutine
			}
		}()

		aiStream, err := cm.Stream(streamCtx, currentMessages)
		if err != nil {
			streamCancel()
			if pauseCtx.Err() != nil {
				return nil, Pause, nil
			} else {
				return nil, Error, err
			}
		}

		var toolCallMessages []*schema.Message
		var hasTool bool

		// 处理当前流
		hasToolCall := false
		streamErr := func() error {
			defer aiStream.Close()
			defer streamCancel()

			for {
				resp, err := aiStream.Recv()
				if err != nil {
					// 检查是否是由于暂停引起的取消
					if pauseCtx.Err() != nil {
						return pauseCtx.Err()
					}

					// 正常结束
					if errors.Is(err, io.EOF) {
						// 如果有工具调用，准备下一轮
						if hasTool && len(toolCallMessages) > 0 {
							hasToolCall = true
							return nil // 返回nil表示正常结束，hasToolCall标记有工具调用
						}

						// 没有工具调用，完成整个对话
						tokenBuffer.Flush()
						return nil // 正常结束，没有工具调用
					}

					// 其他错误
					return fmt.Errorf("stream error: %w", err)
				}

				// 处理工具调用
				if len(resp.ToolCalls) > 0 {
					hasTool = true
					toolCallMessages = append(toolCallMessages, resp)
					continue
				}

				// 处理 reasoning 内容
				if reasoning, ok := deepseek.GetReasoningContent(resp); ok && reasoning != "" {
					message.Content += reasoning
					token := &TokenMessage{
						TopicUID:    rs.topicUID,
						RoleUID:     rs.current.Uid,
						ContentType: "reasoning",
						Content:     reasoning,
					}

					if tokenBuffer.Add(token) {
						tokenBuffer.Flush()
					}
				}

				// 处理主要内容
				if len(resp.Content) > 0 {
					message.Content += resp.Content
					token := &TokenMessage{
						TopicUID:    rs.topicUID,
						RoleUID:     rs.current.Uid,
						ContentType: "text",
						Content:     resp.Content,
					}

					if tokenBuffer.Add(token) {
						tokenBuffer.Flush()
					}
				}
			}
		}()

		// 检查流处理结果
		if streamErr != nil {
			// 如果发生暂停
			if pauseCtx.Err() != nil {
				return nil, Pause, nil
			}
			// 其他错误
			return nil, Error, streamErr
		}

		// 如果收到暂停信号，退出
		if pauseCtx.Err() != nil {
			return nil, Pause, nil
		}

		// 处理工具调用
		if hasToolCall && len(toolCallMessages) > 0 {
			toolMsg, err := schema.ConcatMessages(toolCallMessages)
			if err != nil {
				zap.L().Error("failed to concat messages", zap.Error(err))
				return nil, Error, err
			}

			// 执行工具调用
			toolsReply := invoke(ctx, toolMsg.ToolCalls, mcpTools)

			// 更新消息并继续会话
			currentMessages = append(currentMessages, toolMsg)
			currentMessages = append(currentMessages, toolsReply...)
			continue // 继续外层循环，进行下一次 Stream
		}

		// 如果没有工具调用，结束对话
		tokenBuffer.Flush()
		return message, Normal, nil
	}
}
