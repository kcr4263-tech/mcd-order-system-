package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	// Initialize logs directory and file
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatalf("Failed to create logs directory: %v", err)
	}
	logFile, err := os.OpenFile("logs/order.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()
	// Set log output to both stdout and file
	log.SetOutput(logFile)

	// Initialize Database
	InitDB()
	defer CloseDB()

	// Routing (Go 1.22+ Standard Mux)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/orders", CORS(CreateOrderHandler))
	mux.HandleFunc("GET /api/orders", CORS(GetOrdersHandler))
	mux.HandleFunc("GET /api/orders/{orderNo}", CORS(GetOrderDetailsHandler))
	mux.HandleFunc("PUT /api/orders/{orderNo}/status", CORS(UpdateOrderStatusHandler))

	// Print startup message to terminal (stdout)
	fmt.Println("サーバー起動: http://localhost:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// CORS Middleware to handle preflight and headers
func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}
