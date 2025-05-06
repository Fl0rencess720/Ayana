package data

import (
	"context"

	"github.com/Fl0rencess720/Wittgenstein/app/service/seminar/internal/biz"
	ri "github.com/cloudwego/eino-ext/components/indexer/redis"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	keyPrefix = "Wittgenstein"
)

type ragRepo struct {
	data *Data
	log  *log.Helper
}

func NewRAGRepo(data *Data, logger log.Logger) biz.RAGRepo {
	return &ragRepo{
		data: data,
		log:  log.NewHelper(log.With(logger, "module", "seminar/data/rag")),
	}
}

func (r *ragRepo) DocumentToVectorDB(ctx context.Context, uid string, docs []*schema.Document) error {

	i, err := ri.NewIndexer(ctx, &ri.IndexerConfig{
		Client:           r.data.redisClient,
		KeyPrefix:        keyPrefix,
		DocumentToHashes: nil,
		BatchSize:        10,
		Embedding:        r.data.embedder,
	})
	if err != nil {
		return err
	}
	if err := r.InitVectorIndex(ctx, uid); err != nil {
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

func (r *ragRepo) InitVectorIndex(ctx context.Context, uid string) error {
	if _, err := r.data.redisClient.Do(ctx, "FT.INFO", uid).Result(); err == nil {
		return nil
	}

	createIndexArgs := []interface{}{
		"FT.CREATE", uid,
		"ON", "HASH",
		"PREFIX", "1", keyPrefix,
		"SCHEMA",
		"content", "TEXT",
		"vector_content", "VECTOR", "FLAT",
		"6",
		"TYPE", "FLOAT32",
		"DIM", 4096,
		"DISTANCE_METRIC", "COSINE",
	}

	if err := r.data.redisClient.Do(ctx, createIndexArgs...).Err(); err != nil {
		return err
	}

	if _, err := r.data.redisClient.Do(ctx, "FT.INFO", uid).Result(); err != nil {
		return err
	}
	return nil
}
