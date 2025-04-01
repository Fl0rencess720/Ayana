package biz

import (
	"context"
	"io"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

type SeminarRepo interface {
	CreateTopic(ctx context.Context, phone string, topic *Topic) error
	DeleteTopic(ctx context.Context, topicUID string) error
	GetTopic(ctx context.Context, topicUID string) (Topic, error)
	GetTopicsMetadata(ctx context.Context, phone string) ([]Topic, error)
}

type SeminarUsecase struct {
	repo SeminarRepo
	log  *log.Helper

	topicCache *TopicCache
}

func NewSeminarUsecase(repo SeminarRepo, topicCache *TopicCache, logger log.Logger) *SeminarUsecase {
	return &SeminarUsecase{repo: repo, topicCache: topicCache, log: log.NewHelper(logger)}
}

func (uc *SeminarUsecase) CreateTopic(ctx context.Context, phone string, topic *Topic) error {
	if err := uc.repo.CreateTopic(ctx, phone, topic); err != nil {
		return err
	}
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
	return topic, nil
}

func (uc *SeminarUsecase) GetTopicsMetadata(ctx context.Context, phone string) ([]Topic, error) {
	topics, err := uc.repo.GetTopicsMetadata(ctx, phone)
	if err != nil {
		return nil, err
	}
	return topics, nil
}

func (uc *SeminarUsecase) StartTopic(topicID string, stream grpc.ServerStreamingServer[v1.StartTopicReply]) error {
	cm, err := deepseek.NewChatModel(context.Background(), &deepseek.ChatModelConfig{
		APIKey: viper.GetString("deepseek.apiKey"),
		Model:  "deepseek-reasoner",
	})
	if err != nil {
		return err
	}
	var messages []*schema.Message

	messages = append(messages, &schema.Message{
		Role:    schema.User,
		Content: "帮我写一篇引人入胜的故事",
	})

	ctx := context.Background()
	aiStream, err := cm.Stream(ctx, messages)
	if err != nil {
		return err
	}

	for {
		resp, err := aiStream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if reasoning, ok := deepseek.GetReasoningContent(resp); ok {
			stream.Send(&v1.StartTopicReply{
				Content: &v1.StartTopicReply_Reasoning{Reasoning: reasoning},
			})
		}

		if len(resp.Content) > 0 {
			stream.Send(&v1.StartTopicReply{
				Content: &v1.StartTopicReply_Text{Text: resp.Content},
			})
		}
	}
}
