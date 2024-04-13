package main

import (
	"bytes"
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
)

type PaymentRequest struct {
	Price    int      `json:"price"`
	Tokenpsp int      `json:"token_psp"`
	Userid   int      `json:"user_id"`
	Clientid int      `json:"client_id"`
	Seats    []string `json:"seat_ids"`
}

type paymentData struct {
	Price          int      `json:"price"`
	Userid         int      `json:"user_id"`
	Seats          []string `json:"seat_ids"`
	Paymentconf_id int      `json:"paymentconf_id"`
}

func (app *Config) checkPayment(w http.ResponseWriter, r *http.Request) {
	var paymentrequest PaymentRequest

	//Read the request payload
	err := json.NewDecoder(r.Body).Decode(&paymentrequest)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse payment form: %v", err), http.StatusBadRequest)
		return
	}

	var paymentdata paymentData

	paymentdata.Price = paymentrequest.Price
	paymentdata.Userid = paymentrequest.Userid
	paymentdata.Seats = paymentrequest.Seats
	paymentdata.Paymentconf_id = generatePaymentConfirmationID()

	//Simulating a psp checking operation
	time.Sleep(30 * time.Millisecond)

	//Send the request to the savebooking webhook
	jsonData, err := json.Marshal(paymentdata)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse payment data form: %v", err), http.StatusInternalServerError)
		return
	}
	sort.Strings(paymentdata.Seats)
	//Make a HTTP POST call, to paymentData endpoint
	resp, err := http.Post(generatePaymentUrl(paymentdata.Seats, paymentdata.Userid), "application/json", bytes.NewBuffer(jsonData))

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send data to PaymentData: %v", err), http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"message": "Payment data sent successfully"}`)
}

func generatePaymentUrl(seats []string, userid int) string {
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
	finalurl := "http://localhost:8097/paymentData" + strconv.Itoa(userid) + result.String()
	log.Print("Payment url: ", finalurl)
	return finalurl
}

func generatePaymentConfirmationID() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(900000) + 100000 // Generates a random number between 100000 and 999999
}
