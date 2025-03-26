package data

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Phone    string
	Password string
	Name     string
	Avatar   string
}
