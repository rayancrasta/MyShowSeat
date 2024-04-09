package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	_ "github.com/lib/pq" // Import PostgreSQL driver
	"github.com/redis/go-redis/v9"
)

type ReservationRequest struct {
	SeatReservationIDs []string `json:"seatreservation_ids"`
	BookedbyID         int      `json:"booked_by_id"`
}

// Reservation request structure, based on Reservation table DB schema
type ReservationForm struct {
	SeatIDs    []string `json:"seat_ids"`
	ShowID     int      `json:"show_id"`
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

	//reservation variable now has the json
	db := ConnecttoDB()

	//Check if seatID and showID exsists
	err = checkBooking(db, reservationform)
	if err != nil {
		http.Error(w, fmt.Sprintf("Booking Check failed: %v", err), http.StatusInternalServerError)
		return
	}

	//Create the Reservation Request
	var reservation ReservationRequest

	// Create the Seatreservation ID
	for _, seatID := range reservationform.SeatIDs {
		reservation.SeatReservationIDs = append(reservation.SeatReservationIDs, "SH_"+strconv.Itoa(reservationform.ShowID)+"_ST_"+seatID)
	}
	reservation.BookedbyID = reservationform.BookedbyID

	//Send the request to the producer function
	err = saveBooking(db, reservation, reservationform.ShowID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to book Seat: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Success: Seat %v for Show %v is booked for user %v", reservationform.SeatIDs, reservationform.ShowID, reservationform.BookedbyID)
}

func ConnecttoDB() (db *sqlx.DB) {

	db, err := sqlx.Open("postgres", pgConnectionString)
	if err != nil {
		log.Fatalf("Error connecting to postgresSQL: %v", err)
	}

	return db
}

func checkBooking(db *sqlx.DB, reservationform ReservationForm) error {

	// Check if seats exist
	for _, seatID := range reservationform.SeatIDs {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM Seat WHERE SeatID = $1)`, seatID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("SeatExists Error: %v", err)
		}
		if !exists {
			return fmt.Errorf("seat %s does not exist", seatID)
		}
		// log.Printf("DEBUG: Seatcheck done for seat: %v", seatID)
	}

	// Check if show exists
	var showExists bool
	err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM show WHERE showid = $1)`, reservationform.ShowID).Scan(&showExists)
	if err != nil {
		return fmt.Errorf("ShowExists Error: %v", err)
	}
	if !showExists {
		return fmt.Errorf("show with ID %s does not exist", reservationform.ShowID)
	}

	// log.Printf("DEBUG: showcheck done for show: %v", claimseatform.ShowID)

	return nil

}

func saveBooking(db *sqlx.DB, reservation ReservationRequest, showid int) error {

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
		var claimedByID sql.NullInt64
		if err := rows.Scan(&claimedByID); err != nil {
			return fmt.Errorf("error scanning claimedByID: %v", err)
		}

		// Check if claimedByID is NULL; By default its Null
		if !claimedByID.Valid {
			return fmt.Errorf("seats need to be claimed first")
		}

		if int(claimedByID.Int64) != reservation.BookedbyID {
			// If any claimedbyID is different from bookedByID, return false
			return fmt.Errorf("Some/Many seats are claimed by different user than one boooking it")
		}
	}

	// Above check is true
	// Insert into Postgres DB
	// Loop through each SeatReservationID and update the reservation
	err = updateReservationDB(db, reservation.SeatReservationIDs, reservation.BookedbyID)
	if err != nil {
		return fmt.Errorf("Update Reservation error: %v", err)
	}

	log.Println("Data saved to database")

	//Update Redis Cache with decremented capacity
	err = updateSeatLeftRedis(showid, len(reservation.SeatReservationIDs))
	if err != nil {
		return fmt.Errorf("Redis updated error: %v", err)
	}

	return nil

}

func generateBookingConfirmationID() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(900000) + 100000 // Generates a random number between 100000 and 999999
}

func updateReservationDB(db *sqlx.DB, SeatReservationIDs []string, BookedbyID int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("Error starting transaction: %v", err)
	}
	defer tx.Rollback()

	for _, seatReservationID := range SeatReservationIDs {
		_, err = tx.Exec(`
            UPDATE Reservation 
            SET BookedbyID = $1, Booked = true, Booking_confirmID = $2
            WHERE SeatReservationID = $3`,
			BookedbyID, generateBookingConfirmationID(), seatReservationID)

		if err != nil {
			// Rollback the transaction if any update fails.
			return fmt.Errorf("Error updating database: %v", err)
		}
	}

	// Commit the transaction if all updates are successful.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Error committing transaction: %v", err)
	}
	return nil
}

func updateSeatLeftRedis(showID int, seatsbooked int) error {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	defer rdb.Close()

	// Context for the Redis operations.
	ctx := context.Background()

	//Get current seatleft value from Redis
	seatsLeftCmd := rdb.Get(ctx, strconv.Itoa(showID))
	seatsLeft, err := seatsLeftCmd.Int()
	if err != nil {
		return fmt.Errorf("Error getting seatsLeft from Redis: %v", err)
	}

	new_seatsLeft := seatsLeft - seatsbooked

	// Update the value in Redis.
	err = rdb.Set(ctx, strconv.Itoa(showID), new_seatsLeft, 0).Err()
	if err != nil {
		return fmt.Errorf("Error setting value in Redis: %v", err)
	}

	return nil
}
