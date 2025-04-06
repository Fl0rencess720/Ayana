package biz

import (
	"context"
	"fmt"
	"io"
	nethttp "net/http"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
	"github.com/Fl0rencess720/Wittgenstein/pkgs/utils"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/http"
)

type SeminarUsecase struct {
	repo UserRepo
	log  *log.Helper

	seminarClient v1.SeminarClient
}

type sseResp struct {
	RoleUID string `json:"role_uid"`
	Content string `json:"content"`
}

var globalSeminarUsecase *SeminarUsecase

func NewSeminarUsecase(repo UserRepo, logger log.Logger, seminarClient v1.SeminarClient) *SeminarUsecase {
	seminarUsecase := &SeminarUsecase{repo: repo, log: log.NewHelper(logger), seminarClient: seminarClient}
	globalSeminarUsecase = seminarUsecase
	return seminarUsecase

}

func (uc *SeminarUsecase) CreateTopic(ctx context.Context, req *v1.CreateTopicRequest) (*v1.CreateTopicReply, error) {
	req.Phone = utils.GetPhoneFromContext(ctx)
	reply, err := uc.seminarClient.CreateTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (uc *SeminarUsecase) DeleteTopic(ctx context.Context, req *v1.DeleteTopicRequest) (*v1.DeleteTopicReply, error) {
	reply, err := uc.seminarClient.DeleteTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (uc *SeminarUsecase) GetTopic(ctx context.Context, req *v1.GetTopicRequest) (*v1.GetTopicReply, error) {
	reply, err := uc.seminarClient.GetTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (uc *SeminarUsecase) GetTopicsMetadata(ctx context.Context, req *v1.GetTopicsMetadataRequest) (*v1.GetTopicsMetadataReply, error) {
	req.Phone = utils.GetPhoneFromContext(ctx)
	reply, err := uc.seminarClient.GetTopicsMetadata(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func StartTopic(ctx http.Context) (interface{}, error) {
	req := v1.StartTopicRequest{}
	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}
	req.TopicId = ctx.Query().Get("topic_id")
	stream, err := globalSeminarUsecase.seminarClient.StartTopic(ctx, &req)
	if err != nil {
		return nil, err
	}
	fmt.Println(1)
	w := ctx.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	ctx.Response().WriteHeader(nethttp.StatusOK)
	flusher, _ := w.(http.Flusher)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			fmt.Fprintf(w, "event: end\ndata: %s\n\n", "end")
			flusher.Flush()
			return nil, nil
		}
		if err != nil {
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
			flusher.Flush()
			return nil, nil
		}

		switch content := resp.Content.(type) {
		case *v1.StreamOutputReply_Reasoning:
			sseRespReasoning := sseResp{
				RoleUID: resp.RoleUID,
				Content: content.Reasoning,
			}
			fmt.Printf("sseRespReasoning: %v\n", sseRespReasoning)
			fmt.Fprintf(w, "event: reasoning\ndata: %v\n\n", sseRespReasoning)
		case *v1.StreamOutputReply_Text:
			sseRespText := sseResp{
				RoleUID: resp.RoleUID,
				Content: content.Text,
			}
			fmt.Printf("sseRespText: %v\n", sseRespText)
			fmt.Fprintf(w, "event: text\ndata: %v\n\n", sseRespText)
		}
		flusher.Flush()
	}

}

func (uc *SeminarUsecase) StopTopic(ctx context.Context, req *v1.StopTopicRequest) (*v1.StopTopicReply, error) {
	reply, err := uc.seminarClient.StopTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
