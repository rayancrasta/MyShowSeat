package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// Struct to check if seats
type seatQuery struct {
	ShowID int `json:"show_id"`
}

func (app *Config) HandleisFull(w http.ResponseWriter, r *http.Request) {

	var seatquery seatQuery
	//Read the request payload
	err := json.NewDecoder(r.Body).Decode(&seatquery)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: Failed to parse reservation form: %v", err), http.StatusBadRequest)
		return
	}

	//Check from Redis
	// Get the seatleft count using the showid
	seatleft, err := getSeatCount(seatquery.ShowID)
	if err == redis.Nil {
		writeJSONOutput(w, "CheckDB")
		return
	}
	if err != redis.Nil && err != nil {
		http.Error(w, fmt.Sprintf("Error: Failed to get count from Redis: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Seatleft: %d", seatleft)

	if seatleft <= 0 {
		writeJSONOutput(w, "Not Available")
		return
	} else {
		writeJSONOutput(w, "Available")
		return
	}

}

func writeJSONOutput(w http.ResponseWriter, status string) {
	// Construct a JSON object
	response := map[string]string{
		"status": status,
	}

	// Convert the JSON object to a JSON string
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Error: Failed to convert status to json", http.StatusInternalServerError)
		return
	}
	// Set the Content-Type header to indicate JSON response
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

func getSeatCount(showID int) (int, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	defer rdb.Close()
	// Context for the Redis operations.
	ctx := context.Background()

	//Get current seatleft value from Redis
	seatsLeftCmd := rdb.Get(ctx, strconv.Itoa(showID))

	// Check if the key doesn't exist in Redis
	if seatsLeftCmd.Err() == redis.Nil {
		return -1, seatsLeftCmd.Err()
	}

	if err := seatsLeftCmd.Err(); err != nil {
		return -1, fmt.Errorf("error getting seatsLeft from Redis: %v", err)
	}

	seatsLeft, err := seatsLeftCmd.Int()
	if err != nil {
		return -1, fmt.Errorf("error converting seatsLeft value: %v", err)
	}

	return seatsLeft, nil
}
