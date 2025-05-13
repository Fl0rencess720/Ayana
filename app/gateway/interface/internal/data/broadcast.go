package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Fl0rencess720/Ayana/app/gateway/interface/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type broadcastRepo struct {
	data *Data
	log  *log.Helper
}

func NewBroadcastRepo(data *Data, logger log.Logger) biz.BroadcastRepo {
	return &broadcastRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *broadcastRepo) RegisterConnChannel(ctx context.Context, topic string, connChan chan *biz.TokenMessage) error {
	r.data.mu.Lock()
	r.data.clientConnMap[topic] = append(r.data.clientConnMap[topic], &clientConn{tokenMessageChan: connChan, isclosed: false})
	r.log.Info("RegisterConnChannel", zap.String("topic", topic))
	r.data.mu.Unlock()
	return nil
}

func (r *broadcastRepo) UngisterConnChannel(ctx context.Context, topic string, connChan chan *biz.TokenMessage) error {
	r.log.Info("UngisterConnChannel", zap.String("topic", topic))
	for i, v := range r.data.clientConnMap[topic] {
		if v.tokenMessageChan == connChan {
			r.data.clientConnMap[topic][i].isclosed = true
			return nil
		}
	}
	return nil
}

// ReadTopic 从 Kafka 主题读取消息并处理
func (r *broadcastRepo) ReadTopic(ctx context.Context, topic string) error {
	readers := r.data.kafkaClient.readers
	errChan := make(chan error, 1)

	for _, reader := range readers {
		go func(reader *kafka.Reader) {
			defer func() {
				if closeErr := reader.Close(); closeErr != nil {
					zap.L().Error("failed to close kafka reader", zap.Error(closeErr))
				}
			}()

			for {
				select {
				case <-ctx.Done():
					zap.L().Info("kafka reader context cancelled", zap.Error(ctx.Err()))
					return // 上下文取消，协程退出
				default:
					// 继续尝试读取消息
				}

				// ReadMessage 是一个阻塞调用，直到有消息可用或上下文取消
				msg, err := reader.ReadMessage(ctx)
				if err != nil {
					// 处理读取错误
					switch {
					case errors.Is(err, context.Canceled):
						zap.L().Info("kafka reader cancelled during ReadMessage", zap.Error(err))
						// context.Canceled 错误表示 ReadMessage 是因为 ctx.Done() 而返回
						// 通常不需要通过 errChan 再次通知，因为外部 select 也会检测 ctx.Done()
						return
					case errors.Is(err, io.EOF):
						zap.L().Info("kafka reader connection closed (EOF)", zap.Error(err))
						// 连接关闭，通知外部并退出
						select {
						case errChan <- fmt.Errorf("kafka connection closed: %w", err):
						default:
							// 如果 errChan 已满，避免阻塞
							zap.L().Error("errChan buffer full, couldn't send EOF error")
						}
						return // EOF 错误，协程退出
					default:
						// 其他读取错误，记录日志并继续尝试读取
						zap.L().Error("kafka read error", zap.Error(err))
						// 可以考虑在这里加入短暂的退避时间，避免错误循环
						// time.Sleep(time.Second)
						continue // 继续循环，尝试下一次读取
					}
				}

				// 消息读取成功，处理消息
				var tokenMsg biz.TokenMessage
				if err := json.Unmarshal(msg.Value, &tokenMsg); err != nil {
					zap.L().Error("failed to unmarshal message", zap.Error(err), zap.String("message_value", string(msg.Value)))
					// 反序列化错误，记录日志，跳过此消息
					// 注意：如果需要提交 offset，这里可能需要处理死信队列或跳过提交
					continue
				}

				r.data.mu.Lock()
				if len(r.data.messageCache[tokenMsg.TopicUID]) != 0 {
					if tokenMsg.RoleUID != r.data.messageCache[tokenMsg.TopicUID][len(r.data.messageCache[tokenMsg.TopicUID])-1].RoleUID {
						// 清空切片
						r.data.messageCache[tokenMsg.TopicUID] = r.data.messageCache[tokenMsg.TopicUID][0:0]
					}
				}
				// 追加到切片
				r.data.messageCache[tokenMsg.TopicUID] = append(r.data.messageCache[tokenMsg.TopicUID], &tokenMsg)
				r.data.mu.Unlock()

				var connsCopy []*clientConn
				r.data.mu.Lock()
				conns, ok := r.data.clientConnMap[tokenMsg.TopicUID]
				if ok && len(conns) > 0 {
					connsCopy = make([]*clientConn, len(conns))
					copy(connsCopy, conns)
				}
				r.data.mu.Unlock()

				if len(connsCopy) > 0 {
					for _, clientConn := range connsCopy {

						// 在发送消息到通道时，也要检查上下文，避免阻塞协程退出
						select {
						case clientConn.tokenMessageChan <- &tokenMsg:
							// 消息成功发送
						case <-ctx.Done():
							// 如果在尝试发送时上下文取消，记录并退出当前消息的处理循环
							// 注意：这只会退出内层 for 循环，外层读消息循环会由 ReadMessage 处的 context.Canceled 处理
							zap.L().Info("context cancelled while sending message to client conn", zap.Error(ctx.Err()))
							return // 协程退出，因为上下文被取消了
							// 可以考虑添加一个 default 或超时，以防 tokenMessageChan 长期阻塞
							// default:
							// 	zap.L().Warn("client conn tokenMessageChan is full, dropping message")
						}
					}
				}

				// TODO: 在成功处理消息并发送给客户端后，您可能需要手动提交 offset。
				//      kafka-go Reader 如果配置了 GroupID 且 AutoCommitInterval > 0，
				//      会定期自动提交。如果您需要更精确的提交（例如每处理一条消息或一个批次后提交），
				//      您需要在消息处理逻辑成功后调用 reader.CommitMessages(ctx, msg)。
				//      如果您依赖自动提交，则无需手动调用。
				//      如果您创建了多个 Reader for the same GroupID/Topic，并且依赖自动提交，
				//      其行为可能会变得复杂和难以预测。
				/*
					if commitErr := reader.CommitMessages(ctx, msg); commitErr != nil {
					    zap.L().Error("failed to commit message offset", zap.Error(commitErr))
						// 根据需要处理提交失败，可能需要重试或记录
					}
				*/
			}
		}(reader) // 将 reader 作为参数传递给匿名函数，避免闭包问题
	}

	// ReadTopic 函数等待错误发生或上下文取消
	select {
	case err := <-errChan:
		// 收到一个 Reader 协程通过 errChan 发送的错误
		return err
	case <-ctx.Done():
		// ReadTopic 的上下文被取消
		zap.L().Info("ReadTopic context cancelled", zap.Error(ctx.Err()))
		// 上下文取消后，所有 reader 协程会通过 select <-ctx.Done() 退出
		// 这里返回上下文的错误
		return ctx.Err()
	}
}

func (r *broadcastRepo) IndexTopicLastMessageToRedis(ctx context.Context, topic string, offset int) error {
	// 将某主题最后的发言的起始offset存入redis
	return r.data.redisClient.Set(ctx, fmt.Sprintf("AyanaTopic:%s", topic), offset, 0).Err()
}

// 多网关、多grpc服务情况
// 点击创建主题按钮，发送创建主题请求，主题存入mysql
// 点击开始按钮，发送开始讨论请求，负载均衡到了某个网关，网关生成唯一的topicID
// 网关向某个grpc服务发送开始讨论请求，grpc服务将大模型响应内容推入kafka
// 网关监听此topicID对应的kafka topic，将获取的结果使用sse响应给用户

// 若用户新开了一个窗口，点击某个主题，给某个网关发送GetTopic请求
// 网关先向grpc服务发送GetTopic请求，获取过往历史消息，再监听kafka获取是否有新的消息
// 网关将获取的结果使用sse响应给用户
