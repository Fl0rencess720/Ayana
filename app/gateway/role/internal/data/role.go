package data

import (
	"github.com/Fl0rencess720/Wittgenstein/app/gateway/role/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
)

type userRepo struct {
	data *Data
	log  *log.Helper
}

func NewUserRepo(data *Data, logger log.Logger) biz.RoleRepo {
	return &userRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}
