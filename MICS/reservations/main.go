package main

import (
	"log"

	"github.com/jmoiron/sqlx"
)

const pgConnectionString = "host=localhost port=5432 user=rayanc dbname=tickets sslmode=disable"

func main() {
	log.Println("Consumer started")

	db, err := sqlx.Open("postgres", pgConnectionString)
	if err != nil {
		log.Fatalf("Error connecting to postgresSQL: %v", err)
	}

	defer db.Close()

	//Ping and check
	err = db.Ping()
	if err != nil {
		log.Fatalf("Error pinging PostgreSQL: %v", err)
	}

	go consumeMessages(db)

	//Keep the main thread alive
	select {}
}
