package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Import PostgreSQL driver
)

type ClaimSeatForm struct {
	SeatID     string `json:"seat_id"`
	ShowID     string `json:"show_id"`
	BookedbyID int    `json:"booked_by_id"` //user who is claiming
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
	//Send the request to the producer function
	err = saveClaim(db, claimseatform)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to claim the seat in DB: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Success: Seat %v for Show %v is claimed for user %v", claimseatform.SeatID, claimseatform.ShowID, claimseatform.BookedbyID)
	return
}

func ConnecttoDB() (db *sqlx.DB) {

	db, err := sqlx.Open("postgres", pgConnectionString)
	if err != nil {
		log.Fatalf("Error connecting to postgresSQL: %v", err)
	}

	return db
}

func saveClaim(db *sqlx.DB, claimseatform ClaimSeatForm) error {

	log.Println("Inside ClaimSeat_saveClaim")

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
		return fmt.Errorf("seat %v for Show %v is already Booked", claimseatform.SeatID, claimseatform.ShowID)
	} else if status == "Claimed" {
		return fmt.Errorf("seat %v for Show %v is Claimed by Other User", claimseatform.SeatID, claimseatform.ShowID)
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
