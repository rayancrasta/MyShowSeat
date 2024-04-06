package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/jmoiron/sqlx"
)

const (
	kafkaBroker = "localhost:9092"
	kafkaTopic  = "claim_requests"
)

type ClaimSeatForm struct {
	SeatID     string `json:"seat_id"`
	ShowID     string `json:"show_id"`
	BookedbyID int    `json:"booked_by_id"` //user who is claiming
}

func consumeMessages(db *sqlx.DB) {
	log.Println("DEBUG: Inside Consumer_consumeMessages ")

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  kafkaBroker,
		"group.id":           "reservation-consumer-group", //check
		"auto.offset.reset":  "latest",                     // check
		"enable.auto.commit": "true",                       //check
	})

	if err != nil {
		log.Fatalf("Error creating Kafka consumer: %v", err)
	}

	//Subscribe to Kafka topic
	consumer.SubscribeTopics([]string{kafkaTopic}, nil) //checkDoc

	// Signal channel to handle graceful termination in case of interrupts (SIGINT (controlC) or SIGTERM).
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	//Infinite loop to continuosuly consume messages
	for {
		select {
		case sig := <-sigchan:
			log.Printf("Caught signal: %v: terminating ", sig)
			consumer.Close()
			return
		default:
			//Read the message from Kafka topic
			msg, err := consumer.ReadMessage(-1) // Timeout parameter. -1 is to wait indenitely for the message
			//block until a new message is available or an error occurs.

			if err == nil { //success
				var claimseatform ClaimSeatForm
				err := json.Unmarshal(msg.Value, &claimseatform)

				if err != nil {
					log.Printf("Error decoding JSON: %v", err)
					continue
				}
				//Check if ticket was booked or not is already done at the producer end
				//Save the data to Postgres
				err = saveClaim(db, claimseatform)

				if err != nil {
					log.Printf("Error saving to PostgreSQL: %v", err)
				}

			} else {
				fmt.Printf("Consumer error: %v\n", err)
			}
		}

	}
}

func saveClaim(db *sqlx.DB, claimseatform ClaimSeatForm) error {

	log.Println("Inside Claim_saveClaim")

	//Check if seat is booked or claimed
	seatReservationID := "SH_" + claimseatform.ShowID + "_ST_" + claimseatform.SeatID

	query := `
		SELECT 
			CASE 
				WHEN Booked = TRUE THEN 'Booked'
				WHEN last_claim IS NOT NULL AND last_claim >= NOW() - INTERVAL '1 minute' THEN 'Claimed'
				ELSE 'NotBooked'
			END AS status
		FROM Reservation
		WHERE SeatReservationID = $1;`

	var status string

	err := db.QueryRow(query, seatReservationID).Scan(&status)
	log.Printf("Status: %s", status)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error querying Booked status: %v", err)
	}

	//Initially claim doesnt exsist
	claimexsists := false

	if err != sql.ErrNoRows {
		// if rows exsist , claim exsists
		claimexsists = true
	}

	if status == "Booked" {
		return fmt.Errorf("Seat %v for Show %v is already Booked", claimseatform.SeatID, claimseatform.ShowID)
	} else if status == "Claimed" {
		return fmt.Errorf("Seat %v for Show %v is Claimed by Other User", claimseatform.SeatID, claimseatform.ShowID)
	} else {

		//Seat can be claimed
		currenttime := time.Now()

		if claimexsists {
			_, err = db.Exec(`
				UPDATE Reservation 
				SET ClaimedbyID = $1, last_claim = $2 
				WHERE SeatReservationID = $3`,
				claimseatform.BookedbyID, currenttime, seatReservationID)
		} else {
			_, err = db.Exec(`
			INSERT INTO Reservation (SeatReservationID, ClaimedbyID, last_claim)
			VALUES ($1, $2, $3)`,
				seatReservationID, claimseatform.BookedbyID, currenttime)
		}

		if err != nil {
			return fmt.Errorf("Update/Insert claim query failed")
		}
		log.Println("Claim saved to database")
	}
	return nil
}
