package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func InitDB() {
	var err error
	db, err = sql.Open("sqlite3", "./order.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	createTableSQL := `CREATE TABLE IF NOT EXISTS order_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_no TEXT NOT NULL,
		terminal_no TEXT NOT NULL,
		order_status TEXT NOT NULL,
		item_no INTEGER NOT NULL,
		menu_name TEXT NOT NULL,
		unit_price INTEGER NOT NULL,
		quantity INTEGER NOT NULL,
		subtotal INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
}

func CloseDB() {
	if db != nil {
		db.Close()
	}
}

// Generate sequential Order Number (MMDD-NNN)
func GenerateOrderNo() (string, error) {
	now := time.Now()
	dateStr := now.Format("0102") // MMDD

	// Query the count of unique order numbers generated today to establish the next index
	query := `SELECT COUNT(DISTINCT order_no) FROM order_items WHERE order_no LIKE ?`
	var count int
	err := db.QueryRow(query, dateStr+"-%").Scan(&count)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%03d", dateStr, count+1), nil
}
