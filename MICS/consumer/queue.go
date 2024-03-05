// Code for all things related to the queue
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const (
	kafkaBroker = "localhost:9092"
	kafkaTopic  = "reservation_requests"
)

type ReservationRequest struct {
	SeatReservationID string `json:"seat_id"`
	//LastClaim         string `json:"last_claim"`
	//ClaimedID         int    `json:"claimed_by_id"`
	BookedbyID int `json:"booked_by_id"`
	// IsBooked          bool   `json:"is_booked"`
	// BookingConfirmID string `json:"booking_confirm_id"`
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

	log.Println("Inside Consumer_saveToDatabase")
	//Check if seat is booked or not
	var isBooked bool

	err := db.Get(&isBooked, "SELECT Booked from Reservation Where SeatReservationID = $1", reservation.SeatReservationID)
	//TODO: SeatID may repeat, make it unqiue-r
	if err != nil && err != sql.ErrNoRows {

		return err
	}

	if isBooked {
		return fmt.Errorf("seat is booked", reservation.SeatReservationID)
	}

	// Get last claim TS for that seatID from DB
	var lastClaimTime time.Time
	err = db.Get(&lastClaimTime, "SELECT last_claim from Reservation Where SeatReservationID = $1 ORDER BY last_claim DESC LIMIT 1", reservation.SeatReservationID)
	//TODO: SeatID may repeat, make it unqiue-r
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	currenttime := time.Now()

	if currenttime.Sub(lastClaimTime).Minutes() < 5 {
		return fmt.Errorf("ticket is in process of being booked")
	}

	// Insert into Postgres DB
	_, err = db.Exec(`
			INSERT INTO Reservation (SeatReservationID, BookedbyID, Booked, Booking_confirmID)
			VALUES ($1, $2, $3, $4)`,
		reservation.SeatReservationID, reservation.BookedbyID, true, generateBookingConfirmationID())

	if err != nil {
		return err
	}

	log.Println("Data saved to database")
	return nil
}

func generateBookingConfirmationID() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(900000) + 100000 // Generates a random number between 100000 and 999999
}
