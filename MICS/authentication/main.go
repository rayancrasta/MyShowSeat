package main

import (
	initializers "authentication/initialisers"

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

	r.Run()
}
