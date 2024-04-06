package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Import the pq driver
)

const kafkaBroker = "localhost:9092"
const kafkaTopic = "reservation_requests"
const pgConnectionString = "host=localhost port=5432 user=rayanc dbname=tickets sslmode=disable"

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

type ClaimSeatForm struct {
	SeatID     string `json:"seat_id"`
	ShowID     string `json:"show_id"`
	BookedbyID int    `json:"booked_by_id"` //user who is claiming
}

func (app *Config) oldHandleSeatClaim(w http.ResponseWriter, r *http.Request) {
	log.Println("DEBUG: Inside Producer_HandleSeatClaim ")

	var claimseatform ClaimSeatForm

	//Read the payload
	err := json.NewDecoder(r.Body).Decode(&claimseatform)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse claim form: %v", err), http.StatusBadRequest)
		return
	}

	//Check if seat is booked or claimed
	db := ConnecttoDB()
	defer db.Close()

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

	err = db.QueryRow(query, seatReservationID).Scan(&status)

	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}

	//Initially claim doesnt exsist
	claimexsists := false

	if err != sql.ErrNoRows {
		// if rows exsist , claim exsists
		claimexsists = true
	}

	log.Printf("Status: %s", status)

	if status == "Booked" {
		http.Error(w, fmt.Sprintf("Seat %v for Show %v is already Booked", claimseatform.SeatID, claimseatform.ShowID), http.StatusBadRequest)
		return
	} else if status == "Claimed" {
		http.Error(w, fmt.Sprintf("Seat %v for Show %v is Claimed by Other User", claimseatform.SeatID, claimseatform.ShowID), http.StatusBadRequest)
		return
	} else {

		//Can be claimed

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
			log.Fatalf("claim query failed", err)
		}

		log.Println("Claim saved to database")

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Seat %v for Show %v is claimed for user %v", claimseatform.SeatID, claimseatform.ShowID, claimseatform.BookedbyID)
		return

	}
}

func ConnecttoDB() (db *sqlx.DB) {

	db, err := sqlx.Open("postgres", pgConnectionString)
	if err != nil {
		log.Fatalf("Error connecting to postgresSQL: %v", err)
	}

	return db
}

func (app *Config) HandleSeatClaim(w http.ResponseWriter, r *http.Request) {
	log.Println("DEBUG: Inside Producer_HandleSeatClaim ")

	var claimseatform ClaimSeatForm

	//Read the request payload
	err := json.NewDecoder(r.Body).Decode(&claimseatform)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse reservation form: %v", err), http.StatusBadRequest)
		return
	}

	//Send the request to the producer function
	err = produceClaimMessage(claimseatform)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send reservation request to Kafka: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
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
	err = produceReservationMessage(reservation)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send reservation request to Kafka: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func produceReservationMessage(reservation ReservationRequest) error {
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

	topic := "reservation_requests" // Create a variable to store the topic name

	// Send the message to the queue
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

func produceClaimMessage(claim ClaimSeatForm) error {
	log.Println("DEBUG: Inside Producer_produceMessage ")
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaBroker})

	if err != nil {
		log.Println(err)
		return err
	}

	defer producer.Close()

	//Make it byte
	message, err := json.Marshal(claim)
	if err != nil {
		log.Println(err)
		return err
	}

	topic := "claim_requests" // Create a variable to store the topic name

	// Send the message to the queue
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
