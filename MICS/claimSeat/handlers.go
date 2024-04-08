package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	_ "github.com/lib/pq" // Import PostgreSQL driver
)

type ClaimSeatForm struct {
	SeatIDs    []string `json:"seat_ids"`
	ShowID     string   `json:"show_id"`
	BookedbyID int      `json:"booked_by_id"` //user who is claiming
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
	fmt.Fprintf(w, "Success: Seatsfor Show %v is claimed for user %v", claimseatform.ShowID, claimseatform.BookedbyID)

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
		seatReservationIDs[i] = "SH_" + claimseatform.ShowID + "_ST_" + seatID
	}

	query := `
    SELECT 
        CASE 
            WHEN EXISTS (
                SELECT 1
                FROM Reservation
                WHERE SeatReservationID = ANY($1) AND Booked = TRUE 
            ) THEN 'Booked'
            WHEN EXISTS (
                SELECT 1
                FROM Reservation
                WHERE SeatReservationID = ANY($1) AND last_claim >= NOW() - INTERVAL '1 minute'
            ) THEN 'Claimed'
            ELSE 'Available'
        END AS status;`

	var status string

	err := db.QueryRow(query, pq.Array(seatReservationIDs)).Scan(&status)
	if err != nil {
		return fmt.Errorf("error querying seat availability: %v", err)
	}

	log.Printf("Seat availability status: %s", status)

	if status == "Booked" {
		return fmt.Errorf("the seats for Show %v is not available, already booked", claimseatform.ShowID)
	} else if status == "Claimed" {
		return fmt.Errorf("seat %v for Show %v is Claimed by Other User", claimseatform.SeatIDs, claimseatform.ShowID)
	} else {
		//Seat can be claimed
		//Dont insert just, update
		currenttime := time.Now()

		// Begin a transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %v", err)
		}
		defer tx.Rollback() // Rollback the transaction if it hasn't been committed

		// Loop through each seatReservationID
		for _, seatReservationID := range seatReservationIDs {
			_, err = tx.Exec(`
					UPDATE Reservation 
					SET ClaimedbyID = $1, last_claim = $2 
					WHERE SeatReservationID = $3`,
				claimseatform.BookedbyID, currenttime, seatReservationID)

			if err != nil {
				// Rollback the transaction and return error
				tx.Rollback()
				return fmt.Errorf("update claim query failed for SeatReservationID: %s - %v", seatReservationID, err)
			}
		}

		// Commit the transaction if all updates are successful
		if err := tx.Commit(); err != nil {
			// Rollback the transaction if commit fails
			tx.Rollback()
			return fmt.Errorf("failed to commit transaction: %v", err)
		}

		log.Println("Claims saved to database")

		//Rollback if seat reservation failed for any
	}

	// if claimexsists {
	// 	_, err = db.Exec(`
	// 		UPDATE Reservation
	// 		SET ClaimedbyID = $1, last_claim = $2
	// 		WHERE SeatReservationID = $3`,
	// 		claimseatform.BookedbyID, currenttime, seatReservationID)
	// } else {
	// 	_, err = db.Exec(`
	// 	INSERT INTO Reservation (SeatReservationID, ClaimedbyID, last_claim)
	// 	VALUES ($1, $2, $3)`,
	// 		seatReservationID, claimseatform.BookedbyID, currenttime)
	// }

	// if err != nil {
	// 	return fmt.Errorf("Update/Insert claim query failed")
	// }
	// log.Println("Claim saved to database")

	return nil
}
