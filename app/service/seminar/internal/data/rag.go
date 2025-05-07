package data

import (
	"context"

	"github.com/Fl0rencess720/Ayana/app/service/seminar/internal/biz"
	ri "github.com/cloudwego/eino-ext/components/indexer/redis"
	rr "github.com/cloudwego/eino-ext/components/retriever/redis"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"
)

var (
	keyPrefix = "AyanaDoc:"
	indexName = "AyanaIndex"
)

type ragRepo struct {
	data *Data
	log  *log.Helper
}

func NewRAGRepo(data *Data, logger log.Logger) biz.RAGRepo {
	ctx := context.Background()
	r := &ragRepo{
		data: data,
		log:  log.NewHelper(log.With(logger, "module", "seminar/data/rag")),
	}
	if err := r.InitVectorIndex(ctx); err != nil {
		zap.L().Error("Failed to init vector index", zap.Error(err))
	}
	return r
}

func (r *ragRepo) DocumentToVectorDB(ctx context.Context, uid string, docs []*schema.Document) error {
	i, err := ri.NewIndexer(ctx, &ri.IndexerConfig{
		Client:    r.data.redisClient,
		KeyPrefix: keyPrefix,
		DocumentToHashes: func(ctx context.Context, doc *schema.Document) (*ri.Hashes, error) {
			return &ri.Hashes{
				Key: doc.ID,
				Field2Value: map[string]ri.FieldValue{
					"content": {
						Value:    doc.Content,
						EmbedKey: "vector_content",
					},
					"document_id": {
						Value: uid,
					},
				},
			}, nil
		},
		BatchSize: 10,
		Embedding: r.data.embedder,
	})
	if err != nil {
		return err
	}
	_, err = i.Store(ctx, docs)
	if err != nil {
		return err
	}
	return nil
}

// TODO: 判定文件是否已经上传过
func (r *ragRepo) DocumentToMysql(ctx context.Context, document biz.Document) error {
	if err := r.data.mysqlClient.Model(&biz.Document{}).Create(&document).Error; err != nil {
		return err
	}
	return nil
}

func (r *ragRepo) InitVectorIndex(ctx context.Context) error {
	if _, err := r.data.redisClient.Do(ctx, "FT.INFO").Result(); err == nil {
		return nil
	}

	createIndexArgs := []interface{}{
		"FT.CREATE", indexName,
		"ON", "HASH",
		"PREFIX", "1", keyPrefix,
		"SCHEMA",
		"content", "TEXT",
		"document_id", "TAG",
		"vector_content", "VECTOR", "FLAT",
		"6",
		"DIM", 4096,
		"DISTANCE_METRIC", "COSINE",
		"TYPE", "FLOAT32",
	}

	if err := r.data.redisClient.Do(ctx, createIndexArgs...).Err(); err != nil {
		return err
	}

	if _, err := r.data.redisClient.Do(ctx, "FT.INFO", indexName).Result(); err != nil {
		return err
	}
	return nil
}

func (r *ragRepo) RetrieveDocuments(ctx context.Context, uid, query string) ([]*schema.Document, error) {
	filterQuery := "@document_id:{" + uid + "}"
	docs, err := r.data.retriever.Retrieve(ctx, query, rr.WithFilterQuery(filterQuery))
	if err != nil {
		return nil, err
	}
	return docs, nil
}
