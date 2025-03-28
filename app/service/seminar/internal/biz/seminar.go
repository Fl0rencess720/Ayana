package biz

import (
	"github.com/go-kratos/kratos/v2/log"
)

type SeminarRepo interface {
}

type SeminarUsecase struct {
	repo SeminarRepo
	log  *log.Helper
}

func NewSeminarUsecase(repo SeminarRepo, logger log.Logger) *SeminarUsecase {
	return &SeminarUsecase{repo: repo, log: log.NewHelper(logger)}
}
