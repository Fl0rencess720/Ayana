package data

import (
	"context"
	"time"

	roleV1 "github.com/Fl0rencess720/Wittgenstein/api/gateway/role/v1"
	"github.com/Fl0rencess720/Wittgenstein/app/service/seminar/internal/biz"
	"github.com/Fl0rencess720/Wittgenstein/app/service/seminar/internal/conf"
	embedding "github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	grpcx "google.golang.org/grpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewSeminarRepo, NewRAGRepo, NewMysql, NewRedis, NewEmbedder, NewRoleServiceClient)

// Data .
type Data struct {
	mysqlClient *gorm.DB
	redisClient *redis.Client
	roleClient  roleV1.RoleManagerClient

	embedder *embedding.Embedder
}

// NewData .
func NewData(c *conf.Data, logger log.Logger, mysqlClient *gorm.DB, redisClient *redis.Client, roleClient roleV1.RoleManagerClient, embedder *embedding.Embedder) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	return &Data{mysqlClient: mysqlClient, redisClient: redisClient, roleClient: roleClient, embedder: embedder}, cleanup, nil
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

func NewMysql(c *conf.Data) *gorm.DB {
	db, err := gorm.Open(mysql.Open(c.Database.Source), &gorm.Config{})
	if err != nil {
		panic("failed to connect mysql")
	}
	if err := db.AutoMigrate(&biz.Topic{}, &biz.Speech{}, &biz.Document{}); err != nil {
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
