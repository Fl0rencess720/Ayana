package biz

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	nethttp "net/http"

	v1 "github.com/Fl0rencess720/Ayana/api/gateway/seminar/v1"
	"github.com/Fl0rencess720/Ayana/pkgs/utils"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type SeminarRepo interface {
	GetTopicsMetadataFromRedis(context.Context, string) ([]TopicMetadata, error)
	GetTopicFromRedis(context.Context, string)
}

type SeminarUsecase struct {
	repo  UserRepo
	brepo BroadcastRepo
	log   *log.Helper

	seminarClient v1.SeminarClient
}
type sseResp struct {
	RoleUID string `json:"role_uid"`
	Content string `json:"content"`
}

var globalSeminarUsecase *SeminarUsecase

type TopicMetadata struct {
	Uid          string   `json:"uid"`
	Content      string   `json:"content"`
	Participants []string `json:"participants"`
}

type Speech struct {
	Uid     string
	RoleUid string
	Content string
}

type Topic struct {
	Uid          string
	Participants []string
	Speeches     []Speech
	Title        string
	TitleImage   string
	Content      string
}

func NewSeminarUsecase(repo UserRepo, brepo BroadcastRepo, logger log.Logger, seminarClient v1.SeminarClient) *SeminarUsecase {
	seminarUsecase := &SeminarUsecase{repo: repo, brepo: brepo, log: log.NewHelper(logger), seminarClient: seminarClient}
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
	req.TopicId = ctx.Query().Get("topic_id")
	_, err := globalSeminarUsecase.seminarClient.StartTopic(ctx, &req)
	if err != nil {
		return nil, err
	}
	w := ctx.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	ctx.Response().WriteHeader(nethttp.StatusOK)
	flusher, _ := w.(http.Flusher)
	if err := globalSeminarUsecase.brepo.AddKafkaReader(req.TopicId, kafka.LastOffset); err != nil {
		return nil, err
	}
	tokenChan := make(chan *TokenMessage, 50)

	go func() {
		defer close(tokenChan)
		if err := globalSeminarUsecase.brepo.ReadMessagesByOffset(ctx, req.TopicId, tokenChan); err != nil {
			zap.L().Error(err.Error())
			return
		}
	}()

	for {
		select {
		case token := <-tokenChan:
			if token == nil {
				continue
			}
			if token.ContentType == "reasoning" {
				fmt.Fprintf(w, "event: reasoning\ndata: %v\n\n", sseResp{RoleUID: token.RoleUID, Content: token.Content})
			} else if token.ContentType == "text" {
				fmt.Fprintf(w, "event: text\ndata: %v\n\n", sseResp{RoleUID: token.RoleUID, Content: token.Content})
			} else if token.ContentType == "end" {
				fmt.Fprintf(w, "event: end\ndata: %v\n\n", "")
				flusher.Flush()
				return nil, nil
			}
			flusher.Flush()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (uc *SeminarUsecase) StopTopic(ctx context.Context, req *v1.StopTopicRequest) (*v1.StopTopicReply, error) {
	reply, err := uc.seminarClient.StopTopic(ctx, req)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func UploadDocument(ctx context.Context, file multipart.File, handler *multipart.FileHeader) (interface{}, error) {
	phone := utils.GetPhoneFromContext(ctx)
	stream, err := globalSeminarUsecase.seminarClient.UploadDocument(ctx)
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, 4096)
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		stream.Send(&v1.UploadDocumentRequest{
			Filename:    handler.Filename,
			Phone:       phone,
			ContentType: handler.Header.Get("Content-Type"),
			ChunkData:   buffer[:n],
		})
	}
	reply, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}
	return reply, nil
}
