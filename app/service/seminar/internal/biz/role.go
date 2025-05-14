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
	"github.com/cloudwego/eino/schema"
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

func (rs *RoleScheduler) Call(messages []*schema.Message, signalChan chan StateSignal, tokenChan chan *TokenMessage) (*schema.Message, StateSignal, error) {
	role := rs.current
	ctx, mainCancel := context.WithCancel(context.Background())
	defer mainCancel()

	// 创建一个用于暂停检测的context
	ctxFromPausing, pauseCancel := context.WithCancel(ctx)
	defer pauseCancel()

	// 监听暂停信号
	go func() {
		if signal, ok := <-signalChan; ok {
			if signal == Pause {
				pauseCancel() // 触发暂停
				return
			}
			if signal == Normal {
				return
			}
		}
	}()
	// 初始化聊天模型
	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey: role.ApiKey,
		Model:  role.ModelName,
	})
	if err != nil {
		return nil, Error, err
	}
	// 创建AI流
	aiStream, err := cm.Stream(ctx, messages)
	if err != nil {
		return nil, Error, err
	}
	defer aiStream.Close()

	// 创建token缓冲区
	tokenBuffer := NewTokenBuffer(10, 50*time.Millisecond)

	// 创建发送函数
	batchSender := func(ctx context.Context, tokens []*TokenMessage) error {
		// 假设rs.brepo有一个批量发送方法
		return rs.brepo.SendTokensToKafkaBatch(ctx, rs.topicUID, tokens)
	}

	// 启动异步处理
	tokenBuffer.Start(ctx, batchSender)
	defer tokenBuffer.Stop()

	// 创建响应消息
	message := &schema.Message{Role: schema.User}
	resultChan := make(chan struct {
		*schema.Message
		StateSignal
		error
	}, 1)

	// 处理AI流的goroutine
	go func() {
		defer close(resultChan)
		isFirstToken := true

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
					// 确保所有消息都被发送
					tokenBuffer.Flush()
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
			// 处理reasoning内容
			if reasoning, ok := deepseek.GetReasoningContent(resp); ok && reasoning != "" {
				message.Content += reasoning
				token := &TokenMessage{
					TopicUID:    rs.topicUID,
					RoleUID:     role.Uid,
					ContentType: "reasoning",
					Content:     reasoning,
					IsFirst:     isFirstToken,
				}

				// 添加到缓冲区，如果需要立即刷新则刷新
				if tokenBuffer.Add(token) {
					tokenBuffer.Flush()
				}
			}

			// 处理主要内容
			if len(resp.Content) > 0 {
				message.Content += resp.Content
				token := &TokenMessage{
					TopicUID:    rs.topicUID,
					RoleUID:     role.Uid,
					ContentType: "text",
					Content:     resp.Content,
					IsFirst:     isFirstToken,
				}

				// 添加到缓冲区，如果需要立即刷新则刷新
				if tokenBuffer.Add(token) {
					tokenBuffer.Flush()
				}
			}

			isFirstToken = false
		}
	}()

	// 等待结果或暂停信号
	select {
	case <-ctxFromPausing.Done():
		return nil, Pause, nil
	case result := <-resultChan:
		// 如果操作正常完成，发送正常信号
		if result.StateSignal == Normal {
			signalChan <- Normal
		}
		return result.Message, result.StateSignal, result.error
	}
}
