package data

import (
	"github.com/Fl0rencess720/Ayana/app/service/role/internal/biz"
	"github.com/Fl0rencess720/Ayana/app/service/role/internal/conf"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/extra/redisotel"
	"github.com/go-redis/redis/v8"
	"github.com/google/wire"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewRoleRepo, NewMysql, NewRedis)

// Data .
type Data struct {
	mysqlClient *gorm.DB
	redisClient *redis.Client
}

// NewData .
func NewData(c *conf.Data, logger log.Logger, mysqlClient *gorm.DB, redisClient *redis.Client) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	return &Data{mysqlClient: mysqlClient, redisClient: redisClient}, cleanup, nil
}

func NewMysql(c *conf.Data) *gorm.DB {
	db, err := gorm.Open(mysql.Open(c.Database.Source), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		panic("failed to connect mysql")
	}
	if err := db.AutoMigrate(biz.AIModel{}); err != nil {
		panic("failed to migrate mysql")
	}
	if err := db.AutoMigrate(biz.Role{}); err != nil {
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
	rdb.AddHook(redisotel.TracingHook{})
	return rdb
}
