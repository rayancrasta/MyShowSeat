package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	_ "github.com/lib/pq" // Import PostgreSQL driver
	"github.com/redis/go-redis/v9"
)

type ReservationRequest struct {
	SeatReservationIDs []string `json:"seatreservation_ids"`
	BookedbyID         int      `json:"user_id"`
}

type PaymentData struct {
	Price          int      `json:"price"`
	Userid         int      `json:"user_id"`
	Seats          []string `json:"seat_ids"`
	Paymentconf_id int      `json:"paymentconf_id"`
}

// Reservation request structure, based on Reservation table DB schema
type ReservationForm struct {
	SeatIDs    []string `json:"seat_ids"`
	ShowID     int      `json:"show_id"`
	BookedbyID int      `json:"user_id"` //user who is trying to book
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

	log.Println(reservationform)
	//reservation variable now has the json
	db := ConnecttoDB()

	//Check if seatID and showID exsists
	err = checkBookingDataValid(db, reservationform)
	if err != nil {
		http.Error(w, fmt.Sprintf("Booking Data Check failed: %v", err), http.StatusInternalServerError)
		return
	}

	// SIMULATE PAYMENT SERVICE HERE
	// DO A SELECT UPDATE HERE TO LOCK THE ROWS

	// Begin a transaction
	tx, err := db.Beginx()
	err = lockRowBeforePayment(tx, db, reservationform.SeatIDs)
	if err != nil {
		http.Error(w, fmt.Sprintf("DEBUG: Couldnt lock seats before payment check", err), http.StatusInternalServerError)
		return
	}
	// Go routine that waits for incoming payment data
	sort.Strings(reservationform.SeatIDs)

	paymenturl := getPaymentUrl(reservationform.SeatIDs, reservationform.BookedbyID)
	paymentDataChan := make(chan PaymentData)

	go listenForPaymentData(paymenturl, paymentDataChan)

	// Wait for payment data
	paymentData := <-paymentDataChan

	log.Println("Payment data; price: ", paymentData.Price, " conf id : ", paymentData.Paymentconf_id, " seats: ", paymentData.Seats)

	// //Dummy check
	// log.Println("OG: ", reservationform.BookedbyID)
	// log.Println("GOT: ", paymentData.Userid)

	//Check from paymentData and OG
	if reservationform.BookedbyID != paymentData.Userid {
		http.Error(w, fmt.Sprintf("DEBUG: User arent same as Payment: %v", err), http.StatusInternalServerError)
		return
	}
	//Sort for proper check
	sort.Strings(paymentData.Seats)

	log.Print("Payment Seats", paymentData.Seats)
	log.Print("Reservation Seats", reservationform.SeatIDs)

	if !isSeatsSame(paymentData.Seats, reservationform.SeatIDs) {
		http.Error(w, fmt.Sprintf("DEBUG: Seats arent same as Payment: OG: %v %v", err), http.StatusInternalServerError)
		return
	}

	//Proceed with saving the data, in the db

	//Create the Reservation Request
	var reservation ReservationRequest

	// Create the Seatreservation ID
	for _, seatID := range reservationform.SeatIDs {
		reservation.SeatReservationIDs = append(reservation.SeatReservationIDs, "SH_"+strconv.Itoa(reservationform.ShowID)+"_ST_"+seatID)
	}
	reservation.BookedbyID = reservationform.BookedbyID

	//Send the request to the producer function
	err = saveBooking(tx, db, reservation, reservationform.ShowID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to book Seat: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Success: Seat %v for Show %v is booked for user %v", reservationform.SeatIDs, reservationform.ShowID, reservationform.BookedbyID)
}

func isSeatsSame(slice1, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	for i := 0; i < len(slice1); i++ {
		if slice1[i] != slice2[i] {
			return false
		}
	}

	return true
}

func ConnecttoDB() (db *sqlx.DB) {

	db, err := sqlx.Open("postgres", pgConnectionString)
	if err != nil {
		log.Fatalf("Error connecting to postgresSQL: %v", err)
	}

	return db
}

func lockRowBeforePayment(tx *sqlx.Tx, db *sqlx.DB, seatIDs []string) error {
	_, err := tx.Exec(`
        SELECT SeatReservationID
        FROM Reservation
        WHERE SeatReservationID = ANY($1)
        FOR UPDATE`, pq.Array(seatIDs))
	if err != nil {
		return fmt.Errorf("error locking rows: %v", err)
	}
	return nil
}

func getPaymentUrl(seats []string, userid int) string {
	var result strings.Builder

	// Regular expression to match special characters
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")

	// Iterate over each string in the slice
	for _, str := range seats {
		// Replace special characters with an empty string
		processedStr := reg.ReplaceAllString(str, "")

		// Append the processed string to the result
		result.WriteString(processedStr)
	}

	// Return the final concatenated string
	finalurl := "/paymentData" + strconv.Itoa(userid) + result.String()
	log.Print("Payment url: ", finalurl)
	return finalurl
}

func checkBookingDataValid(db *sqlx.DB, reservationform ReservationForm) error {
	//Check if Seats and showid exsist

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
		return fmt.Errorf("show with ID %d does not exist", reservationform.ShowID)
	}

	// log.Printf("DEBUG: showcheck done for show: %v", claimseatform.ShowID)

	return nil
}

func saveBooking(tx *sqlx.Tx, db *sqlx.DB, reservation ReservationRequest, showid int) error {
	log.Println("Inside Consumer_saveToDatabase")
	defer tx.Rollback() // Rollback the transaction if it hasn't been committed

	// Check if any of the provided SeatReservationIDs are already booked
	var count int
	err := tx.Get(&count, `
    SELECT COUNT(*)
    FROM Reservation
    WHERE SeatReservationID = ANY($1) AND Booked = TRUE`, pq.Array(reservation.SeatReservationIDs))

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error querying booked status: %v", err)
	}

	if count > 0 {
		return fmt.Errorf("the exact seat range isn't available")
	} else {
		// At least one of the seats is already booked, so we need to lock the rows
		_, err := tx.Exec(`
        SELECT SeatReservationID
        FROM Reservation
        WHERE SeatReservationID = ANY($1)
        FOR UPDATE`, pq.Array(reservation.SeatReservationIDs))
		if err != nil {
			return fmt.Errorf("error locking rows: %v", err)
		}
	}

	// Check if all claimedbyID match the bookedbyID
	rows, err := tx.Queryx(`
        SELECT ClaimedbyID
        FROM Reservation
        WHERE SeatReservationID = ANY($1)
        FOR UPDATE`, pq.Array(reservation.SeatReservationIDs))

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no rows found for SeatReservationIDs: %v", reservation.SeatReservationIDs)
		}
		return fmt.Errorf("error querying claimedbyIDs: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var claimedByID sql.NullInt64
		if err := rows.Scan(&claimedByID); err != nil {
			return fmt.Errorf("error scanning claimedByID: %v", err)
		}

		if !claimedByID.Valid {
			return fmt.Errorf("seats need to be claimed first")
		}

		if int(claimedByID.Int64) != reservation.BookedbyID {
			return fmt.Errorf("some/many seats are claimed by a different user than the one booking them")
		}
	}

	// Update the reservation in the database
	err = updateReservationDB(tx, reservation.SeatReservationIDs, reservation.BookedbyID)
	if err != nil {
		return fmt.Errorf("update reservation error: %v", err)
	}

	log.Println("Data saved to database")

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Update Redis Cache with decremented capacity
	err = updateSeatLeftRedis(showid, len(reservation.SeatReservationIDs))
	if err != nil {
		return fmt.Errorf("redis update error: %v", err)
	}

	return nil
}

func updateReservationDB(tx *sqlx.Tx, SeatReservationIDs []string, BookedbyID int) error {
	for _, seatReservationID := range SeatReservationIDs {
		_, err := tx.Exec(`
            UPDATE Reservation 
            SET BookedbyID = $1, Booked = true, Booking_confirmID = $2
            WHERE SeatReservationID = $3`,
			BookedbyID, generateBookingConfirmationID(), seatReservationID)

		if err != nil {
			return fmt.Errorf("error updating database: %v", err)
		}
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
		return fmt.Errorf("error getting seatsLeft from Redis: %v", err)
	}

	new_seatsLeft := seatsLeft - seatsbooked

	// Update the value in Redis.
	err = rdb.Set(ctx, strconv.Itoa(showID), new_seatsLeft, 0).Err()
	if err != nil {
		return fmt.Errorf("error setting value in Redis: %v", err)
	}

	return nil
}

func generateBookingConfirmationID() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(900000) + 100000 // Generates a random number between 100000 and 999999
}

func listenForPaymentData(paymenturl string, paymentDataChan chan PaymentData) {

	defer fmt.Printf("DEBUG_Conc: Exited the listenForPaymentData")

	server := &http.Server{Addr: ":8097"} // Create an HTTP server instance

	// ServeMux to handle routes
	mux := http.NewServeMux()

	// Channel to signal when the response is sent
	responseSent := make(chan struct{})

	// Register handler for the paymenturl
	mux.HandleFunc(paymenturl, func(w http.ResponseWriter, r *http.Request) {
		// Check if the request method is POST
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var paymentData PaymentData
		err := json.NewDecoder(r.Body).Decode(&paymentData)
		if err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		// Send the payment data to the channel
		paymentDataChan <- paymentData

		// Respond with a success message
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message": "Payment data received successfully"}`)

		// Signal that the response is sent
		close(responseSent)
	})

	// Set the ServeMux as the server's handler
	server.Handler = mux

	fmt.Println("Server listening for payment data on port 8097:", paymenturl)

	// Start the HTTP server in seperate go routine, else server.ListenandServe will block the main thread
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("Error starting server:", err)
		}
	}()

	// Wait for the response to be sent
	<-responseSent

	// Shutdown the server gracefully
	if err := server.Shutdown(context.Background()); err != nil {
		fmt.Println("Error shutting down server:", err)
	}
}
