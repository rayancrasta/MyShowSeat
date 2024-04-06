package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Import PostgreSQL driver
)

type ReservationRequest struct {
	SeatReservationID string `json:"seat_id"`
	BookedbyID        int    `json:"booked_by_id"`
}

// Reservation request structure, based on Reservation table DB schema
type ReservationForm struct {
	SeatID     string `json:"seat_id"`
	ShowID     string `json:"show_id"`
	BookedbyID int    `json:"booked_by_id"` //user who is trying to book
}

func (app *Config) HandleBookSeat(w http.ResponseWriter, r *http.Request) {
	log.Println("DEBUG: Inside bookSeat_HandleBookSeat ")

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
	db := ConnecttoDB()
	//Send the request to the producer function
	err = saveBooking(db, reservation)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to book Seat: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Success: Seat %v for Show %v is booked for user %v", reservationform.SeatID, reservationform.ShowID, reservationform.BookedbyID)
}

func ConnecttoDB() (db *sqlx.DB) {

	db, err := sqlx.Open("postgres", pgConnectionString)
	if err != nil {
		log.Fatalf("Error connecting to postgresSQL: %v", err)
	}

	return db
}

func saveBooking(db *sqlx.DB, reservation ReservationRequest) error {

	log.Println("Inside Consumer_saveToDatabase")
	//Check if seat is booked or not
	var isBooked sql.NullBool

	err := db.Get(&isBooked, "SELECT Booked FROM Reservation WHERE SeatReservationID = $1", reservation.SeatReservationID)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error querying Booked status: %v", err)
	}

	if isBooked.Valid && isBooked.Bool {
		return fmt.Errorf("seat %v is already booked", reservation.SeatReservationID)
	}

	//Check is claimedbyID is same as bookedbyID
	var ClaimedbyID int
	query := "SELECT ClaimedbyID FROM Reservation WHERE SeatReservationID = $1"
	err = db.QueryRow(query, reservation.SeatReservationID).Scan(&ClaimedbyID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no rows found for SeatReservationID: %s", reservation.SeatReservationID)
		} else {
			return fmt.Errorf("error querying ClaimedbyID: %v", err)
		}
	}

	fmt.Printf("ClaimedID: %d", ClaimedbyID)
	fmt.Printf("BookedbyID: %d", reservation.BookedbyID)

	if ClaimedbyID == reservation.BookedbyID {
		// Insert into Postgres DB
		_, err = db.Exec(`
		UPDATE Reservation 
		SET BookedbyID = $1, Booked = true, Booking_confirmID = $2
		WHERE SeatReservationID = $3`,
			reservation.BookedbyID, generateBookingConfirmationID(), reservation.SeatReservationID)

		if err != nil {
			return fmt.Errorf("error saving booking to DB")
		}

		log.Println("Data saved to database")
		return nil
	} else {
		// Get last claim TS for that seatID from DB
		var lastClaimTime time.Time
		err = db.Get(&lastClaimTime, "SELECT last_claim from Reservation Where SeatReservationID = $1 ORDER BY last_claim DESC LIMIT 1", reservation.SeatReservationID)
		//TODO: SeatID may repeat, make it unqiue-r
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("error querying lastclaim: %v", err)
		}
		currenttime := time.Now()

		if currenttime.Sub(lastClaimTime).Minutes() < 1 {
			return fmt.Errorf("ticket is claimed before 1mins by some other user")
		} else {
			return fmt.Errorf("booking can be done again by other user from claim part")
		}
	}
}

func generateBookingConfirmationID() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(900000) + 100000 // Generates a random number between 100000 and 999999
}
