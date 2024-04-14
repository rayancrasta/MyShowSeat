package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Import PostgreSQL driver
)

type ClaimSeatForm struct {
	SeatIDs    []string `json:"seat_ids"`
	ShowID     int      `json:"show_id"`
	BookedbyID int      `json:"user_id"` //user who is claiming
}

func (app *Config) HandleSeatClaim(w http.ResponseWriter, r *http.Request) {
	log.Println("DEBUG: Inside claimSeat_HandleSeatClaim ")

	var claimseatform ClaimSeatForm

	//Read the request payload
	err := json.NewDecoder(r.Body).Decode(&claimseatform)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse claim form: %v", err), http.StatusBadRequest)
		return
	}

	db := ConnecttoDB()
	//Validation to check if show and seat match
	err = checkClaim(db, claimseatform)
	if err != nil {
		http.Error(w, fmt.Sprintf("Claim Check failed: %v", err), http.StatusInternalServerError)
		return
	}

	//Send the request to the producer function
	err = saveClaim(db, claimseatform)

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to claim the seat in DB: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Success: Seats %v for Show %v is claimed for user %v", claimseatform.SeatIDs, claimseatform.ShowID, claimseatform.BookedbyID)

}

func ConnecttoDB() (db *sqlx.DB) {

	db, err := sqlx.Open("postgres", pgConnectionString)
	if err != nil {
		log.Fatalf("Error connecting to postgresSQL: %v", err)
	}

	return db
}

func checkClaim(db *sqlx.DB, claimseatform ClaimSeatForm) error {

	// Check if seats exist
	for _, seatID := range claimseatform.SeatIDs {
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
	err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM show WHERE showid = $1)`, claimseatform.ShowID).Scan(&showExists)
	if err != nil {
		return fmt.Errorf("ShowExists Error: %v", err)
	}
	if !showExists {
		return fmt.Errorf("show with ID %s does not exist", claimseatform.ShowID)
	}

	// log.Printf("DEBUG: showcheck done for show: %v", claimseatform.ShowID)

	return nil

}

func saveClaim(db *sqlx.DB, claimseatform ClaimSeatForm) error {
	log.Println("Inside ClaimSeat_saveClaim")

	// Create an array of seatReservationIDs
	seatReservationIDs := make([]string, len(claimseatform.SeatIDs))
	for i, seatID := range claimseatform.SeatIDs {
		seatReservationIDs[i] = "SH_" + strconv.Itoa(claimseatform.ShowID) + "_ST_" + seatID
	}

	// Begin a transaction
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback() // Rollback the transaction if it hasn't been committed

	// Loop through each seatReservationID
	for _, seatReservationID := range seatReservationIDs {
		// Execute a SELECT statement with FOR UPDATE to lock the row
		var status string
		err = tx.QueryRowx(`
            SELECT 
                CASE 
                    WHEN Booked THEN 'Booked'
                    WHEN last_claim >= NOW() - INTERVAL '1 minute' THEN 'Claimed'
                    ELSE 'Available'
                END AS status
            FROM Reservation
            WHERE SeatReservationID = $1
            FOR UPDATE`, seatReservationID).Scan(&status)

		if err != nil {
			// Rollback the transaction and return error
			tx.Rollback()
			return fmt.Errorf("error querying seat availability: %v", err)
		}

		log.Printf("Seat %s availability status: %s", seatReservationID, status)

		if status == "Booked" {
			return fmt.Errorf("the seats for Show %v are not available, already booked", claimseatform.ShowID)
		} else if status == "Claimed" {
			return fmt.Errorf("seats %v for Show %v are claimed by another user", claimseatform.SeatIDs, claimseatform.ShowID)
		}

		// Update the reservation row
		_, err = tx.Exec(`
            UPDATE Reservation 
            SET ClaimedbyID = $1, last_claim = NOW() 
            WHERE SeatReservationID = $2`,
			claimseatform.BookedbyID, seatReservationID)

		if err != nil {
			// Rollback the transaction and return error
			tx.Rollback()
			return fmt.Errorf("update claim query failed for SeatReservationID: %s - %v", seatReservationID, err)
		}

		log.Printf("Claim saved for SeatReservationID: %s", seatReservationID)
	}

	// Commit the transaction if all updates are successful
	if err := tx.Commit(); err != nil {
		// Rollback the transaction if commit fails
		tx.Rollback()
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Println("Claims saved to database")
	return nil
}
