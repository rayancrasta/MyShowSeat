package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

const kafkaBroker = "localhost:9092"
const kafkaTopic = "reservation_requests"

// Reservation request structure, based on Reservation table DB schema
type ReservationForm struct {
	SeatID     string `json:"seat_id"`
	ShowID     string `json:"show_id"`
	BookedbyID int    `json:"booked_by_id"` //user who is trying to book
}

type ReservationRequest struct {
	SeatReservationID string `json:"seat_id"`
	//LastClaim         string `json:"last_claim"`
	//ClaimedID         int    `json:"claimed_by_id"`
	BookedbyID int `json:"booked_by_id"`
	// IsBooked          bool   `json:"is_booked"`
	// BookingConfirmID string `json:"booking_confirm_id"`
}

func (app *Config) HandleReservation(w http.ResponseWriter, r *http.Request) {
	log.Println("DEBUG: Inside Producer_HandleReservation ")

	var reservationform ReservationForm

	//Read the request payload
	err := json.NewDecoder(r.Body).Decode(&reservationform)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse reservation form: %v", err), http.StatusBadRequest)
		return
	}

	//Create the Reservation Request
	var reservation ReservationRequest

	// Create the Seatreservation ID
	reservation.SeatReservationID = "SH_" + reservationform.ShowID + "_ST_" + reservationform.SeatID //logic can be made more complex
	reservation.BookedbyID = reservationform.BookedbyID

	//reservation variable now has the json

	//Send the request to the producer function
	err = produceMessage(reservation)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send reservation request to Kafka: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func produceMessage(reservation ReservationRequest) error {
	log.Println("DEBUG: Inside Producer_produceMessage ")
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaBroker})

	if err != nil {
		log.Println(err)
		return err
	}

	defer producer.Close()

	//Make it byte
	message, err := json.Marshal(reservation)
	if err != nil {
		log.Println(err)
		return err
	}

	topic := kafkaTopic // Create a variable to store the topic name

	err = producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny},
		Value: message,
	}, nil)

	if err != nil {
		log.Println(err)
		return err
	}

	// Wait for any outstanding messages to be delivered
	producer.Flush(3 * 1000) // 15-second timeout, adjust as needed

	return nil
}
