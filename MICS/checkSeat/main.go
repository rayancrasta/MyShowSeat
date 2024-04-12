package main

import (
	"fmt"
	"log"
	"net/http"
)

const webPort = "8093"

type Config struct {
}

const pgConnectionString = "host=localhost port=5432 user=rayanc dbname=tickets sslmode=disable"

func main() {
	app := Config{}

	log.Printf("Starting CheckSeat service on port: %s", webPort)

	// HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", webPort),
		Handler: app.routes(),
	}

	//Start the web server
	err := srv.ListenAndServe()

	if err != nil {
		log.Panic(err)
	}

}
