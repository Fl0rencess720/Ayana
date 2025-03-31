package service

import (
	"context"
	"fmt"
	"io"
	nethttp "net/http"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
	"github.com/Fl0rencess720/Wittgenstein/app/gateway/interface/internal/biz"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/spf13/viper"
)

type SeminarService struct {
	v1.UnimplementedSeminarServer

	uc *biz.SeminarUsecase
}

func NewSeminarService(uc *biz.SeminarUsecase) *SeminarService {
	return &SeminarService{uc: uc}
}

func (s *SeminarService) CreateTopic(ctx context.Context, req *v1.CreateTopicRequest) (*v1.CreateTopicReply, error) {
	reply, err := s.uc.CreateTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func (s *SeminarService) DeleteTopic(ctx context.Context, req *v1.DeleteTopicRequest) (*v1.DeleteTopicReply, error) {
	reply, err := s.uc.DeleteTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func (s *SeminarService) GetTopic(ctx context.Context, req *v1.GetTopicRequest) (*v1.GetTopicReply, error) {
	reply, err := s.uc.GetTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func (s *SeminarService) GetTopicsMetadata(ctx context.Context, req *v1.GetTopicsMetadataRequest) (*v1.GetTopicsMetadataReply, error) {
	reply, err := s.uc.GetTopicsMetadata(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func call(ctx http.Context) (any, error) {
	apiKey := viper.GetString("deepseek.api_key")

	w := ctx.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	cm, err := deepseek.NewChatModel(context.Background(), &deepseek.ChatModelConfig{
		APIKey: apiKey,
		Model:  "deepseek-reasoner",
	})
	if err != nil {
		return nil, err
	}
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "写一段非常吸引人的小短文",
		},
	}
	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := cm.Stream(streamCtx, messages)
	if err != nil {
		return nil, err
	}

	ctx.Response().WriteHeader(nethttp.StatusOK)
	flusher, _ := w.(http.Flusher)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			fmt.Fprintf(w, "data: Message: %s\n\n", "end")
			flusher.Flush()
			return nil, nil
		}
		if err != nil {
			fmt.Fprintf(w, "data: Error: %s\n\n", err.Error())
			flusher.Flush()
			return nil, nil
		}
		if reasoning, ok := deepseek.GetReasoningContent(resp); ok {
			fmt.Fprintf(w, "data: reasoning: %s\n\n", reasoning)
			flusher.Flush()
		}
		if len(resp.Content) > 0 {
			fmt.Fprintf(w, "data: content: %s\n\n", resp.Content)
			flusher.Flush()
		}
	}
}

func StartTopic(ctx http.Context) error {
	h := ctx.Middleware(func(c context.Context, req interface{}) (interface{}, error) {
		return call(ctx)
	})
	_, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}
