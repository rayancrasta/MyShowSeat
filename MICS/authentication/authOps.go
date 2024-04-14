package main

import (
	initializers "authentication/initialisers"
	"authentication/models"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func SignUp(c *gin.Context) {
	//Get email password
	var body struct {
		Userid   int    `json:"userid"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if c.Bind(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read body",
		})
		return
	}
	fmt.Println("Parsed Body:", body)
	//hashing the password
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 10)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read body",
		})
		return
	}

	//User details
	user := models.User{Userid: body.Userid, Username: body.Username, Password: string(hash)}

	result := initializers.DB.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to create User",
		})
		return
	}
	//Respond
	c.JSON(http.StatusOK, gin.H{})

}

func Login(c *gin.Context) {
	//Get credentials
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if c.Bind(&body) != nil {
		fmt.Println("Parsed Body:", body)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read body",
		})
		return
	}
	fmt.Println("Parsed Body:", body)

	// Lookup the userid

	var user models.User
	initializers.DB.First(&user, "username = ?", body.Username)

	if user.Userid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid Userid or password",
		})
		return
	}

	//Compare the password after hashing with password in db
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(body.Password))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid Userid or password",
		})
		return
	}

	log.Println("Userid: ", user.Userid)
	log.Println("Password: ", user.Password)
	//Generate a JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": int(user.Userid),
		"exp": time.Now().Add(time.Hour * 5).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString([]byte(os.Getenv("SECRET")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to create token",
		})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{
		"token": tokenString,
	})
}
