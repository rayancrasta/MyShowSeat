package main

import (
	initializers "authentication/initialisers"
	"os"

	"github.com/gin-gonic/gin"
)

func init() {
	initializers.LoadEnvVariables()
	initializers.ConnectToDB()
	initializers.SyncDatabase()
}

func main() {
	r := gin.Default()

	r.POST("/signup", SignUp)
	r.POST("/login", Login)

	err := r.Run(os.Getenv("PORT"))
	if err != nil {
		panic("[Error] failed to start Gin server due to: " + err.Error())
	}
}
