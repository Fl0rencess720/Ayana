package data

import (
	"context"
	"sync"
	"time"

	roleV1 "github.com/Fl0rencess720/Ayana/api/gateway/role/v1"
	"github.com/Fl0rencess720/Ayana/app/service/seminar/internal/biz"
	"github.com/Fl0rencess720/Ayana/app/service/seminar/internal/conf"
	"github.com/Fl0rencess720/Ayana/pkgs/kafkatopic"
	embedding "github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/google/wire"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	grpcx "google.golang.org/grpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewSeminarRepo, NewRAGRepo, NewMysql, NewRedis,
	NewEmbedder, NewIndexer, NewRetriever, NewRoleServiceClient, NewBroadcastRepo, NewMilvus, NewKafkaClient)

type kafkaClient struct {
	kafkaWriter       *kafka.Writer
	pauseSignalReader *kafka.Reader
	pauseSignalWriter *kafka.Writer
}

type HybridRetriever struct {
	Client    *milvusclient.Client
	Embedding *embedding.Embedder
	TopK      int
}

type HybridIndexer struct {
	Client    *milvusclient.Client
	Embedding *embedding.Embedder
}

// Data .
type Data struct {
	mysqlClient  *gorm.DB
	redisClient  *redis.Client
	kafkaClient  *kafkaClient
	milvusClient *milvusclient.Client
	embedder     *embedding.Embedder

	indexer   *HybridIndexer
	retriever *HybridRetriever

	roleClient roleV1.RoleManagerClient

	mu                   sync.Mutex
	topicPauseContextMap map[string]chan biz.StateSignal
}

// NewData .
func NewData(c *conf.Data, logger log.Logger, mysqlClient *gorm.DB, redisClient *redis.Client, milvusClient *milvusclient.Client, kafkaClient *kafkaClient, roleClient roleV1.RoleManagerClient,
	embedder *embedding.Embedder, indexer *HybridIndexer, retriever *HybridRetriever) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	topicPauseContextMap := make(map[string]chan biz.StateSignal)
	return &Data{mysqlClient: mysqlClient, redisClient: redisClient, milvusClient: milvusClient, kafkaClient: kafkaClient, roleClient: roleClient, embedder: embedder, indexer: indexer, retriever: retriever, topicPauseContextMap: topicPauseContextMap}, cleanup, nil
}

func NewKafkaClient(c *conf.Data) *kafkaClient {
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{c.Kafka.Addr},
		Topic:   kafkatopic.TOPIC,
		Async:   true,
	})
	pauseSignalReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{c.Kafka.Addr},
		Topic:   kafkatopic.PAUSE_SIGNAL_TOPIC,
	})
	pauseSignalWriter := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{c.Kafka.Addr},
		Topic:   kafkatopic.PAUSE_SIGNAL_TOPIC,
		Async:   true,
	})
	return &kafkaClient{
		kafkaWriter:       writer,
		pauseSignalReader: pauseSignalReader,
		pauseSignalWriter: pauseSignalWriter,
	}
}

func NewEmbedder(c *conf.Data) *embedding.Embedder {
	ctx := context.Background()
	embedder, err := embedding.NewEmbedder(ctx, &embedding.EmbeddingConfig{
		APIKey:  viper.GetString("rag.api_key"),
		Model:   viper.GetString("rag.model"),
		BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
		Region:  "cn-beijing",
	})
	if err != nil {
		panic("failed to create embedder")
	}
	return embedder
}

func NewIndexer(vdb *milvusclient.Client, embedder *embedding.Embedder) *HybridIndexer {
	return &HybridIndexer{
		Client:    vdb,
		Embedding: embedder,
	}
}

func NewRetriever(vdb *milvusclient.Client, embedder *embedding.Embedder) *HybridRetriever {
	r := &HybridRetriever{
		Client:    vdb,
		Embedding: embedder,
		TopK:      8,
	}
	return r
}

func NewMysql(c *conf.Data) *gorm.DB {
	db, err := gorm.Open(mysql.Open(c.Database.Source), &gorm.Config{})
	if err != nil {
		panic("failed to connect mysql")
	}
	if err := db.AutoMigrate(&biz.Topic{}, &biz.Speech{}, &biz.Document{}, &biz.LoadDocument{}, &biz.MCPServer{}); err != nil {
		panic("failed to migrate mysql")
	}

	return db
}

func NewRedis(c *conf.Data) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         c.Redis.Addr,
		Password:     c.Redis.Password,
		DB:           int(c.Redis.Db),
		DialTimeout:  c.Redis.DialTimeout.AsDuration(),
		WriteTimeout: c.Redis.WriteTimeout.AsDuration(),
		ReadTimeout:  c.Redis.ReadTimeout.AsDuration(),
	})
	return rdb
}

func NewMilvus(c *conf.Data) *milvusclient.Client {
	ctx := context.Background()
	client, err := milvusclient.New(context.Background(), &milvusclient.ClientConfig{
		Address: c.Milvus.Endpoint,
		APIKey:  c.Milvus.ApiKey,
	})
	if err != nil {
		panic(err)
	}
	has, err := client.HasCollection(ctx, milvusclient.NewHasCollectionOption(c.Milvus.Collection))
	if err != nil {
		zap.L().Error("init Milvus client failed", zap.Error(err))
		return nil
	}
	if has {
		return client
	}
	schema := entity.NewSchema().WithDynamicFieldEnabled(true)
	schema.WithField(entity.NewField().
		WithName("id").
		WithDataType(entity.FieldTypeInt64).
		WithIsPrimaryKey(true).
		WithIsAutoID(true),
	).WithField(entity.NewField().
		WithName("uid").
		WithDataType(entity.FieldTypeVarChar).
		WithMaxLength(2000),
	).WithField(entity.NewField().
		WithName("text").
		WithDataType(entity.FieldTypeVarChar).
		WithMaxLength(1000).
		WithEnableAnalyzer(true),
	).WithField(entity.NewField().
		WithName("sparse").
		WithDataType(entity.FieldTypeSparseVector),
	).WithField(entity.NewField().
		WithName("dense").
		WithDataType(entity.FieldTypeFloatVector).
		WithDim(4096),
	).WithFunction(entity.NewFunction().
		WithName("text_to_sparse_vector").
		WithType(entity.FunctionTypeBM25).
		WithInputFields("text").
		WithOutputFields("sparse"))
	sparseIndex := milvusclient.NewCreateIndexOption(c.Milvus.Collection, "sparse",
		index.NewSparseInvertedIndex(entity.BM25, 0.2))
	denseIndex := milvusclient.NewCreateIndexOption(c.Milvus.Collection, "dense",
		index.NewAutoIndex(index.MetricType(entity.COSINE)))
	uidIndex := milvusclient.NewCreateIndexOption(c.Milvus.Collection, "uid", index.NewAutoIndex(index.MetricType(entity.FieldTypeVarChar)))
	err = client.CreateCollection(ctx,
		milvusclient.NewCreateCollectionOption(c.Milvus.Collection, schema).
			WithIndexOptions(sparseIndex, denseIndex, uidIndex))
	if err != nil {
		zap.L().Error("New Milvus failed", zap.Error(err))
		return nil
	}
	return client
}

func NewRoleServiceClient(sr *conf.Service, rr registry.Discovery) roleV1.RoleManagerClient {
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint(sr.Role.Endpoint),
		grpc.WithDiscovery(rr),
		grpc.WithMiddleware(
			tracing.Client(),
			recovery.Recovery(),
		),
		grpc.WithTimeout(2*time.Second),
		grpc.WithOptions(grpcx.WithStatsHandler(&tracing.ClientHandler{})),
	)
	if err != nil {
		panic(err)
	}
	c := roleV1.NewRoleManagerClient(conn)
	return c
}
