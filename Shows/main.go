package main

import (
	"fmt"
	"log"
	"net/http"
)

const webPort = "8095"

const pgConnectionString = "host=localhost port=5432 user=rayanc dbname=tickets sslmode=disable"

type Config struct {
}

func main() {
	app := Config{}

	log.Printf("Starting ClaimSeat service on port: %s", webPort)

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
