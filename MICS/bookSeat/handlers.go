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
	"github.com/lib/pq"
	_ "github.com/lib/pq" // Import PostgreSQL driver
)

type ReservationRequest struct {
	SeatReservationIDs []string `json:"seatreservation_ids"`
	BookedbyID         int      `json:"booked_by_id"`
}

// Reservation request structure, based on Reservation table DB schema
type ReservationForm struct {
	SeatID     []string `json:"seat_ids"`
	ShowID     string   `json:"show_id"`
	BookedbyID int      `json:"booked_by_id"` //user who is trying to book
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
	for _, seatID := range reservationform.SeatID {
		reservation.SeatReservationIDs = append(reservation.SeatReservationIDs, "SH_"+reservationform.ShowID+"_ST_"+seatID)
	}
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

	// Count the number of booked seats for any of the provided SeatReservationIDs
	err := db.Get(&isBooked, "SELECT COUNT(*) > 0 FROM Reservation WHERE SeatReservationID = ANY($1) and Booked= TRUE", pq.Array(reservation.SeatReservationIDs))

	// If there is an error or no seats are booked, set isBooked to false
	if err != nil || !isBooked.Valid || !isBooked.Bool {
		isBooked.Valid = true
		isBooked.Bool = false
	} else {
		// At least one seat is booked, so set isBooked to true
		isBooked.Valid = true
		isBooked.Bool = true
	}

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error querying Booked status: %v", err)
	}

	if isBooked.Valid && isBooked.Bool {
		return fmt.Errorf("The exact seat range isnt available")
	}

	//Check if all claimedbyID is same as bookedbyID
	// Select all claimedbyIDs for the provided SeatReservationIDs
	rows, err := db.Query("SELECT ClaimedbyID FROM Reservation WHERE SeatReservationID = ANY($1)", pq.Array(reservation.SeatReservationIDs))
	// <WORKTODO>
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no rows found for SeatReservationIDs: %v", reservation.SeatReservationIDs)
		} else {
			return fmt.Errorf("error querying ClaimedbyIDs: %v", err)
		}
	}
	defer rows.Close()

	// Check if all claimedbyIDs are the same as bookedByID
	for rows.Next() {
		var claimedByID int
		if err := rows.Scan(&claimedByID); err != nil {
			return fmt.Errorf("error scanning claimedByID: %v", err)
		}
		if claimedByID != reservation.BookedbyID {
			// If any claimedbyID is different from bookedByID, return false
			return fmt.Errorf("Some/Many seats are claimed by different user than one boooking it")
		}
	}

	// Above check is true
	// Insert into Postgres DB
	// Loop through each SeatReservationID and update the reservation
	for _, seatReservationID := range reservation.SeatReservationIDs {
		_, err = db.Exec(`
        UPDATE Reservation 
        SET BookedbyID = $1, Booked = true, Booking_confirmID = $2
        WHERE SeatReservationID = $3`,
			reservation.BookedbyID, generateBookingConfirmationID(), seatReservationID)

		if err != nil {
			return fmt.Errorf("error saving booking to DB for SeatReservationID: %s", seatReservationID)
		}
	}

	log.Println("Data saved to database")
	return nil
	// } else {
	// 	// Get last claim TS for that seatID from DB
	// 	var lastClaimTime time.Time
	// 	err = db.Get(&lastClaimTime, "SELECT last_claim from Reservation Where SeatReservationID = $1 ORDER BY last_claim DESC LIMIT 1", reservation.SeatReservationID)
	// 	//TODO: SeatID may repeat, make it unqiue-r
	// 	if err != nil && err != sql.ErrNoRows {
	// 		return fmt.Errorf("error querying lastclaim: %v", err)
	// 	}
	// 	currenttime := time.Now()

	// 	if currenttime.Sub(lastClaimTime).Minutes() < 1 {
	// 		return fmt.Errorf("ticket is claimed before 1mins by some other user")
	// 	} else {
	// 		return fmt.Errorf("booking can be done again by other user from claim part")
	// 	}
	// }
}

func generateBookingConfirmationID() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(900000) + 100000 // Generates a random number between 100000 and 999999
}
