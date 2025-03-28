package data

import (
	"github.com/Fl0rencess720/Wittgenstein/app/service/seminar/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
)

type seminarRepo struct {
	data *Data
	log  *log.Helper
}

func NewSeminarRepo(data *Data, logger log.Logger) biz.SeminarRepo {
	return &seminarRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}
