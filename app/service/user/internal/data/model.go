package data

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Phone    string `gorm:"type:varchar(20);uniqueIndex"`
	Password string
	Name     string
	Avatar   string
}
