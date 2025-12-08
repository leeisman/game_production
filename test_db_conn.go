package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"log"
)

func main() {
	// Try password: postgres
	connStr := "host=localhost port=15432 user=postgres password=postgres dbname=casino_db sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		fmt.Printf("❌ Failed with password 'postgres': %v\n", err)
	} else {
		fmt.Println("✅ Success with password 'postgres'!")
		return
	}

	// Try password: password
	connStr2 := "host=localhost port=15432 user=postgres password=password dbname=casino_db sslmode=disable"
	db2, err := sql.Open("postgres", connStr2)
	if err != nil {
		log.Fatal(err)
	}
	defer db2.Close()

	err = db2.Ping()
	if err != nil {
		fmt.Printf("❌ Failed with password 'password': %v\n", err)
	} else {
		fmt.Println("✅ Success with password 'password'!")
	}
}
