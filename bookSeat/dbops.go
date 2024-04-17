package main

import (
	"github.com/jmoiron/sqlx"
)

func ConnectToDB() (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", pgConnectionString)
	if err != nil {
		return db, err
	}

	return db, nil
}
