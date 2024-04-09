package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Import PostgreSQL driver
)

type Show struct {
	ShowName  string    `json:"show_name"`
	VenueID   int       `json:"venue_id"`
	HallID    int       `json:"hall_id"`
	Starttime time.Time `json:"show_start_time"`
	Endtime   time.Time `json:"show_end_time"`
}

func (app *Config) createShow(w http.ResponseWriter, r *http.Request) {

	var show Show

	err := json.NewDecoder(r.Body).Decode(&show)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse show form: %v", err), http.StatusBadRequest)
		return
	}

	db := ConnecttoDB()

	//Check if HallID and VenueID is correct
	err = checkValidValues(db, show)
	if err != nil {
		http.Error(w, fmt.Sprintf("CheckFailed : %v", err), http.StatusBadRequest)
		return
	}

	//Get capactiy of the hall, using hallID and venueID
	hallCapacity, err := getHallCapacity(db, show.VenueID, show.HallID)
	log.Printf("HallCapacity: %s", hallCapacity)
	if err != nil {
		http.Error(w, fmt.Sprintf("HallCapacity failed : %v", err), http.StatusBadRequest)
		return
	}

	// Create show in show table
	var showid int
	err = db.QueryRow(`INSERT INTO Show (ShowName, VenueID, HallID, Time_start, Time_end)
						VALUES ($1, $2, $3, $4, $5)
						ON CONFLICT (ShowName, VenueID, HallID, Time_start) DO NOTHING
						RETURNING showid`,
		show.ShowName, show.VenueID, show.HallID, show.Starttime, show.Endtime).Scan(&showid)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Show already exists in the database", http.StatusBadRequest)
			return
		} else {
			// Handle other errors
			fmt.Printf("ShowInsert error : %v\n", err)
			return
		}
	}

	// Construct a JSON object
	response := map[string]int{
		"showid": showid,
	}

	// Convert the JSON object to a JSON string
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to convert showID to json", http.StatusInternalServerError)
		return
	}

	// Slice to store seatIDs
	var seatIDs []string

	seatIDs, err = getSeatIDs(db, show.VenueID, show.HallID)
	if err != nil {
		http.Error(w, fmt.Sprintf("GetSeatIDs failed : %v", err), http.StatusInternalServerError)
		return
	}

	// Create entries in reservation table
	// Columns that will be created : showid, seatreservationid
	err = insertReservations(db, showid, seatIDs)
	if err != nil {
		http.Error(w, fmt.Sprintf("Reservation table entry failed : %v", err), http.StatusInternalServerError)
		return
	}

	// Set the Content-Type header to indicate JSON response
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)

}

func ConnecttoDB() (db *sqlx.DB) {

	db, err := sqlx.Open("postgres", pgConnectionString)
	if err != nil {
		log.Fatalf("Error connecting to postgresSQL: %v", err)
	}

	return db
}

func getSeatIDs(db *sqlx.DB, venueID int, hallID int) ([]string, error) {

	var seatIDs []string

	query := `SELECT seatid FROM seat WHERE venueid = $1 AND hallid = $2`
	rows, err := db.Query(query, venueID, hallID)
	if err != nil {
		return seatIDs, err
	}
	defer rows.Close()

	// Iterate through the rows and append seatIDs to the slice
	for rows.Next() {
		var seatID string
		if err := rows.Scan(&seatID); err != nil {
			return seatIDs, err
		}
		seatIDs = append(seatIDs, seatID)
	}
	if err := rows.Err(); err != nil {
		return seatIDs, err
	}
	return seatIDs, nil
}

func checkValidValues(db *sqlx.DB, show Show) error {
	//Validate the VenueID, HallID
	// Check if Venue exists
	var venueExists bool
	err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM venue WHERE venueid = $1)`, show.VenueID).Scan(&venueExists)
	if err != nil {
		return fmt.Errorf("VenueExsits Error: %v", err)
	}
	if !venueExists {
		return fmt.Errorf("venue with ID %d does not exist", show.VenueID)
	}
	// Check if HallID exists
	var hallExsists bool
	err = db.QueryRow(`SELECT EXISTS (SELECT 1 FROM hall WHERE hallid = $1)`, show.HallID).Scan(&hallExsists)
	if err != nil {
		return fmt.Errorf("HallExsits Error: %v", err)
	}
	if !hallExsists {
		return fmt.Errorf("hall with ID %d does not exist", show.HallID)
	}

	return nil
}

func getHallCapacity(db *sqlx.DB, VenueID int, HallID int) (string, error) {
	var hallCapacity int
	err := db.QueryRow(`SELECT capacity FROM hall WHERE hallid = $1 AND venueid = $2`, HallID, VenueID).Scan(&hallCapacity)

	if err != nil {
		return "", fmt.Errorf("HallCapacity Error: %v", err)
	}

	return strconv.Itoa(hallCapacity), nil
}

func insertReservations(db *sqlx.DB, showID int, seatIDs []string) error {
	// Begin a transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // Rollback transaction if not committed

	for _, seatID := range seatIDs {
		seatReservationID := "SH_" + strconv.Itoa(showID) + "_ST_" + seatID
		_, err := tx.Exec("INSERT INTO reservation (seatreservationid, showid) VALUES ($1, $2)", seatReservationID, showID)
		if err != nil {
			return err
		}
	}

	// Commit the transaction if all inserts were successful
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
