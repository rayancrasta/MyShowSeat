// Code for all things related to the queue
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const (
	kafkaBroker = "localhost:9092"
	kafkaTopic  = "reservation_requests"
)

type ReservationRequest struct {
	SeatID           string `json:"seat_id"`
	LastClaim        string `json:"last_claim"`
	ClaimedID        int    `json:"claimed_by_id"`
	BookedbyID       int    `json:"booked_by_id"`
	IsBooked         bool   `json:"is_booked"`
	BookingConfirmID string `json:"booking_confirm_id"`
}

func consumeMessages(db *sqlx.DB) {
	log.Println("DEBUG: Inside Consumer_consumeMessages ")

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  kafkaBroker,
		"group.id":           "reservation-consumer-group", //check
		"auto.offset.reset":  "earliest",                   // check
		"enable.auto.commit": "false",                      //check
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
				var reservation ReservationRequest
				err := json.Unmarshal(msg.Value, &reservation)

				if err != nil {
					log.Printf("Error decoding JSON: %v", err)
					continue
				}
				// Future : Check if ticket is booked or not

				//Save the data to Postgres
				err = saveToDatabase(db, reservation)

				if err != nil {
					log.Printf("Error saving to PostgreSQL: %v", err)
				}

			} else {
				fmt.Printf("Consumer error: %v\n", err)
			}
		}

	}
}

func saveToDatabase(db *sqlx.DB, reservation ReservationRequest) error {
	// Insert into Postgres DB
	_, err := db.Exec(`
			INSERT INTO Reservation (SeatID, last_claim, ClaimedbyID, BookedbyID, Booked, Booking_confirmID)
			VALUES ($1, $2, $3, $4, $5, $6)`,
		reservation.SeatID, reservation.LastClaim, reservation.ClaimedID, reservation.BookedbyID, reservation.IsBooked, reservation.BookingConfirmID)
	if err == nil {
		log.Println("Data saved to database")
	}
	return err
}
