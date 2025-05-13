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
					return
				default:
				}

				msg, err := reader.ReadMessage(ctx)
				if err != nil {
					switch {
					case errors.Is(err, context.Canceled):
						zap.L().Info("kafka reader cancelled during ReadMessage", zap.Error(err))
						return
					case errors.Is(err, io.EOF):
						zap.L().Info("kafka reader connection closed (EOF)", zap.Error(err))
						select {
						case errChan <- fmt.Errorf("kafka connection closed: %w", err):
						default:
							zap.L().Error("errChan buffer full, couldn't send EOF error")
						}
						return
					default:
						zap.L().Error("kafka read error", zap.Error(err))
						continue
					}
				}

				var tokenMsg biz.TokenMessage
				if err := json.Unmarshal(msg.Value, &tokenMsg); err != nil {
					zap.L().Error("failed to unmarshal message", zap.Error(err), zap.String("message_value", string(msg.Value)))
					continue
				}

				r.data.mu.Lock()
				if len(r.data.messageCache[tokenMsg.TopicUID]) != 0 {
					if tokenMsg.RoleUID != r.data.messageCache[tokenMsg.TopicUID][len(r.data.messageCache[tokenMsg.TopicUID])-1].RoleUID {

						r.data.messageCache[tokenMsg.TopicUID] = r.data.messageCache[tokenMsg.TopicUID][0:0]
					}
				}
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
						if clientConn.isclosed {
							continue
						}
						select {
						case clientConn.tokenMessageChan <- &tokenMsg:

						case <-ctx.Done():

							zap.L().Info("context cancelled while sending message to client conn", zap.Error(ctx.Err()))
							return
						}
					}
				}

			}
		}(reader)
	}

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		zap.L().Info("ReadTopic context cancelled", zap.Error(ctx.Err()))
		return ctx.Err()
	}
}

func (r *broadcastRepo) IndexTopicLastMessageToRedis(ctx context.Context, topic string, offset int) error {
	return r.data.redisClient.Set(ctx, fmt.Sprintf("AyanaTopic:%s", topic), offset, 0).Err()
}
