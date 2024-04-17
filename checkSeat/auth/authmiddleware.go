package authmiddleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is missing", http.StatusUnauthorized)
			return
		}

		tokenString := strings.Split(authHeader, "Bearer ")[1]
		log.Println("Token: ", tokenString)

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Don't forget to validate the alg is what you expect:
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			// Secret
			return []byte("verysecretsecret"), nil
		})
		if err != nil {
			http.Error(w, "Error parsing the JWT token ", http.StatusUnauthorized)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			//Check the expiration
			if float64(time.Now().Unix()) > claims["exp"].(float64) {
				http.Error(w, "Claim time isnt correct ", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		} else {
			http.Error(w, "Claim isnt correct ", http.StatusUnauthorized)
			return
		}
	})
}
