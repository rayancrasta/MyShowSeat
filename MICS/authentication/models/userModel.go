package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Userid   int `gorm:"unique"`
	Username string
	Password string
}
