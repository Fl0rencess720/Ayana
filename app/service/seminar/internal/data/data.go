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
	rr "github.com/cloudwego/eino-ext/components/retriever/redis"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/google/wire"
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
	NewEmbedder, NewRetriever, NewRoleServiceClient, NewBroadcastRepo, NewKafkaClient)

type kafkaClient struct {
	kafkaWriter       *kafka.Writer
	pauseSignalReader *kafka.Reader
	pauseSignalWriter *kafka.Writer
}

// Data .
type Data struct {
	mysqlClient *gorm.DB
	redisClient *redis.Client
	kafkaClient *kafkaClient
	embedder    *embedding.Embedder
	retriever   *rr.Retriever
	roleClient  roleV1.RoleManagerClient

	mu                   sync.Mutex
	topicPauseContextMap map[string]chan biz.StateSignal
}

// NewData .
func NewData(c *conf.Data, logger log.Logger, mysqlClient *gorm.DB, redisClient *redis.Client, kafkaClient *kafkaClient, roleClient roleV1.RoleManagerClient,
	embedder *embedding.Embedder, retriever *rr.Retriever) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	topicPauseContextMap := make(map[string]chan biz.StateSignal)
	return &Data{mysqlClient: mysqlClient, redisClient: redisClient, kafkaClient: kafkaClient, roleClient: roleClient, embedder: embedder, retriever: retriever, topicPauseContextMap: topicPauseContextMap}, cleanup, nil
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

func NewRetriever(vdb *redis.Client, embedder *embedding.Embedder) *rr.Retriever {
	ctx := context.Background()
	r, err := rr.NewRetriever(ctx, &rr.RetrieverConfig{
		Client:            vdb,
		Index:             indexName,
		VectorField:       "vector_content",
		DistanceThreshold: nil,
		Dialect:           2,
		ReturnFields:      []string{"vector_content", "content"},
		DocumentConverter: nil,
		TopK:              5,
		Embedding:         embedder,
	})
	if err != nil {
		zap.L().Error("Failed to create retriever", zap.Error(err))
		return nil
	}
	return r
}

func NewMysql(c *conf.Data) *gorm.DB {
	db, err := gorm.Open(mysql.Open(c.Database.Source), &gorm.Config{})
	if err != nil {
		panic("failed to connect mysql")
	}
	if err := db.AutoMigrate(&biz.Topic{}, &biz.Speech{}, &biz.Document{}, &biz.LoadDocument{}); err != nil {
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
