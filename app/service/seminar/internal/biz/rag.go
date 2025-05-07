package biz

import (
	"bytes"
	"context"
	"io"

	v1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/seminar/v1"
	"github.com/cloudwego/eino-ext/components/document/parser/pdf"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RAGRepo interface {
	DocumentToVectorDB(ctx context.Context, uid string, docs []*schema.Document) error
	DocumentToMysql(ctx context.Context, document Document) error
}

type RAGUsecase struct {
	repo RAGRepo
	log  *log.Helper
}

type Document struct {
	ID          uint   `gorm:"primaryKey"`
	UID         string `gorm:"unique;index"`
	Filename    string
	ContentType string
	Description string
	TotalSize   int64
	Phone       string `gorm:"index"`
}

type LoadDocument struct {
	ID          uint   `gorm:"primaryKey"`
	DocumentUID string `gorm:"foreignKey:DocumentUID;references:UID"`
	TopicUID    string `gorm:"foreignKey:TopicUID;references:UID"`
}

func NewRAGUsecase(repo RAGRepo, logger log.Logger) *RAGUsecase {
	return &RAGUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *RAGUsecase) UploadDocument(stream v1.Seminar_UploadDocumentServer) error {
	var (
		buf         bytes.Buffer
		filename    string
		phone       string
		contentType string
		firstChunk  = true
		ctx         = context.Background()
	)

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "接收数据失败: %v", err)
		}

		if firstChunk {
			firstChunk = false
			filename = req.GetFilename()
			contentType = req.GetContentType()
			phone = req.GetPhone()
		} else {
			if req.GetFilename() != filename || req.GetContentType() != contentType {
				return status.Error(codes.InvalidArgument, "元数据不可变更")
			}
		}
		if _, err := buf.Write(req.GetChunkData()); err != nil {
			return status.Errorf(codes.Internal, "内存写入失败: %v", err)
		}
	}
	reader := bytes.NewReader(buf.Bytes())
	docs, err := parseDocument(ctx, reader)
	if err != nil {
		return err
	}
	splittedDocs, err := splitDocument(ctx, docs)
	if err != nil {
		return err
	}
	uid := uuid.New().String()
	if err := uc.repo.DocumentToVectorDB(ctx, uid, splittedDocs); err != nil {
		return err
	}
	if err := uc.repo.DocumentToMysql(ctx, Document{
		UID:         uid,
		Filename:    filename,
		ContentType: contentType,
		TotalSize:   int64(buf.Len()),
		Phone:       phone,
	}); err != nil {
		return err
	}
	return nil
}

// TODO: 对象优化
func parseDocument(ctx context.Context, reader io.Reader) ([]*schema.Document, error) {
	parser, err := pdf.NewPDFParser(ctx, &pdf.Config{
		ToPages: true,
	})
	if err != nil {
		return nil, err
	}
	docs, err := parser.Parse(ctx, reader)
	if err != nil {
		return nil, err
	}
	return docs, nil
}

func splitDocument(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	splitter, err := recursive.NewSplitter(ctx, &recursive.Config{
		ChunkSize:   800,
		OverlapSize: 200,
		Separators:  []string{"\n", ".", "?", "!", "。"},
		LenFunc:     nil,
		KeepType:    recursive.KeepTypeEnd,
	})
	if err != nil {
		return nil, err
	}
	splittedDocs, err := splitter.Transform(ctx, docs)
	if err != nil {
		return nil, err
	}
	for _, doc := range splittedDocs {
		doc.ID = uuid.NewString()
	}
	return splittedDocs, nil
}
