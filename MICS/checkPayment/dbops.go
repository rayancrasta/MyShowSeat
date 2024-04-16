package main

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func ConnectToDB() (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", pgConnectionString)
	if err != nil {
		return db, err
	}

	return db, nil
}
