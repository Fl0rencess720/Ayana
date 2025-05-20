package data

import (
	"context"
	"sync"

	"github.com/Fl0rencess720/Ayana/app/service/seminar/internal/biz"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"go.uber.org/zap"
)

type ragRepo struct {
	data *Data
	log  *log.Helper
}

func NewRAGRepo(data *Data, logger log.Logger) biz.RAGRepo {
	r := &ragRepo{
		data: data,
		log:  log.NewHelper(log.With(logger, "module", "seminar/data/rag")),
	}
	return r
}

func (r *ragRepo) DocumentToVectorDB(ctx context.Context, uid string, docs []*schema.Document) error {
	if err := r.data.indexer.Store(ctx, uid, docs); err != nil {
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

func (r *ragRepo) RetrieveDocuments(ctx context.Context, uid, query string) ([]*schema.Document, error) {
	filter := `uid == "` + uid + `"`
	docs, err := r.data.retriever.Retrieve(ctx, query, filter)
	if err != nil {
		return nil, err
	}
	return docs, nil
}

func (hi *HybridIndexer) Store(ctx context.Context, uid string, docs []*schema.Document) error {
	texts := make([]string, 0, len(docs))
	uids := make([]string, 0, len(docs))
	for i := 0; i < len(docs); i++ {
		uids = append(uids, uid)
	}
	for _, doc := range docs {
		texts = append(texts, doc.Content)
	}

	v2, err := hi.Embedding.EmbedStrings(ctx, texts)
	if err != nil {
		return err
	}
	denseVector := convertFloat64ToFloat32(v2)
	_, err = hi.Client.Insert(ctx, milvusclient.NewColumnBasedInsertOption("ayana_documents").
		WithVarcharColumn("text", texts).
		WithVarcharColumn("uid", uids).
		WithFloatVectorColumn("dense", len(v2[0]), denseVector),
	)
	if err != nil {
		return err
	}
	return nil
}

func (hr *HybridRetriever) Retrieve(ctx context.Context, query string, filter string) ([]*schema.Document, error) {
	queryVector := []float32{0.3580376395471989, -0.6023495712049978, 0.18414012509913835, -0.26286205330961354, 0.9029438446296592}
	sparseVector, _ := entity.NewSliceSparseEmbedding([]uint32{3573, 5263}, []float32{0.34701499, 0.263937551})

	denseReq := milvusclient.NewAnnRequest("dense", 2, entity.FloatVector(queryVector)).
		WithAnnParam(index.NewIvfAnnParam(10)).
		WithSearchParam(index.MetricTypeKey, "COSINE").WithFilter(filter)
	annParam := index.NewSparseAnnParam()
	annParam.WithDropRatio(0.2)
	sparseReq := milvusclient.NewAnnRequest("sparse", 2, sparseVector).
		WithAnnParam(annParam).
		WithSearchParam(index.MetricTypeKey, "BM25")
	reranker := milvusclient.NewWeightedReranker([]float64{0.8, 0.3})
	resultSets, err := hr.Client.HybridSearch(ctx, milvusclient.NewHybridSearchOption(
		"ayana_documents",
		hr.TopK,
		denseReq,
		sparseReq,
	).WithReranker(reranker).WithOutputFields("text"))
	if err != nil {
		zap.L().Error("HybridSearch failed", zap.Error(err))
		return nil, err
	}
	docs := make([]*schema.Document, len(resultSets))
	for i, resultSet := range resultSets {
		text, err := resultSet.GetColumn("text").GetAsString(i)
		if err != nil {
			return nil, err
		}
		docs = append(docs, &schema.Document{
			ID:      resultSet.IDs.FieldData().String(),
			Content: text,
		})
	}
	return docs, nil
}
func convertFloat64ToFloat32(input [][]float64) [][]float32 {
	result := make([][]float32, len(input))
	var wg sync.WaitGroup

	for i, row := range input {
		wg.Add(1)
		go func(i int, row []float64) {
			defer wg.Done()
			result[i] = make([]float32, len(row))
			for j, val := range row {
				result[i][j] = float32(val)
			}
		}(i, row)
	}

	wg.Wait()
	return result
}
