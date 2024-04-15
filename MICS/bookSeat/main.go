package main

import (
	"fmt"
	"log"
	"net/http"
)

const webPort = "8091"

type Config struct {
}

const pgConnectionString = "host=localhost port=5432 user=rayanc dbname=tickets sslmode=disable"

func main() {
	app := Config{}

	log.Printf("Starting BookSeat service on port: %s", webPort)

	// HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", webPort),
		Handler: app.routes(),
	}

	//DB connection check
	_, err := ConnectToDB()
	if err != nil {
		log.Fatalf("Error: DB connection %v", err)
		return
	}

	//Start the web server
	err = srv.ListenAndServe()

	if err != nil {
		log.Panic(err)
	}

}
