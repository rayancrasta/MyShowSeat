package authmiddleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
			http.Error(w, "Error: Authorization header is missing", http.StatusUnauthorized)
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
			http.Error(w, "Error: Error parsing the JWT token ", http.StatusInternalServerError)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			//Check the expiration
			if float64(time.Now().Unix()) > claims["exp"].(float64) {
				http.Error(w, "Error: Claim time isnt correct ", http.StatusUnauthorized)
				return
			}
			//Attach to request
			// Token is valid, add userID to request body
			body := make(map[string]interface{})
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "Error: Failed to decode request body", http.StatusInternalServerError)
				return
			}

			// Convert the value of claims["sub"] to a float64
			sub, ok := claims["sub"].(float64)
			if !ok {
				http.Error(w, "Error: Failed to convert sub to float64", http.StatusInternalServerError)
				return
			}
			// Convert the float64 value to an integer
			value := int(sub)
			body["user_id"] = value

			// Encode the modified body and create a new request with it
			newBody, err := json.Marshal(body)
			if err != nil {
				http.Error(w, "Error: Failed to encode modified body", http.StatusInternalServerError)
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(newBody))
			next.ServeHTTP(w, r)
		} else {
			http.Error(w, "Error: Claim isnt correct ", http.StatusUnauthorized)
			return
		}
	})
}
