package data

import (
	"context"
	"time"

	roleV1 "github.com/Fl0rencess720/Ayana/api/gateway/role/v1"
	seminarV1 "github.com/Fl0rencess720/Ayana/api/gateway/seminar/v1"
	userV1 "github.com/Fl0rencess720/Ayana/api/gateway/user/v1"
	"github.com/Fl0rencess720/Ayana/app/gateway/interface/internal/conf"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-redis/redis/extra/redisotel"
	"github.com/go-redis/redis/v8"
	"github.com/google/wire"
	grpcx "google.golang.org/grpc"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewRoleRepo, NewUserRepo, NewRedis, NewRoleServiceClient, NewUserServiceClient, NewSeminarServiceClient)

// Data .
type Data struct {
	redisClient *redis.Client
	rc          roleV1.RoleManagerClient
	uc          userV1.UserClient
	sc          seminarV1.SeminarClient
}

// NewData .
func NewData(c *conf.Data, logger log.Logger, redisClient *redis.Client, uc userV1.UserClient, rc roleV1.RoleManagerClient, sc seminarV1.SeminarClient) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	return &Data{uc: uc, rc: rc, sc: sc, redisClient: redisClient}, cleanup, nil
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
	rdb.AddHook(redisotel.TracingHook{})
	return rdb
}

func NewUserServiceClient(sr *conf.Service, rr registry.Discovery) userV1.UserClient {
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint(sr.User.Endpoint),
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
	c := userV1.NewUserClient(conn)
	return c
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

func NewSeminarServiceClient(sr *conf.Service, rr registry.Discovery) seminarV1.SeminarClient {
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint(sr.Seminar.Endpoint),
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
	c := seminarV1.NewSeminarClient(conn)
	return c
}
