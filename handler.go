package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Request/Response Structures
type OrderItemReq struct {
	MenuName  string `json:"menuName"`
	UnitPrice int    `json:"unitPrice"`
	Quantity  int    `json:"quantity"`
}

type OrderReq struct {
	TerminalNo  string         `json:"terminalNo"`
	MessageType string         `json:"messageType"`
	TotalAmount int            `json:"totalAmount"`
	Items       []OrderItemReq `json:"items"`
}

type OrderStatusReq struct {
	OrderStatus string `json:"orderStatus"`
}

type SuccessResponse struct {
	Result      string `json:"result"`
	OrderNo     string `json:"orderNo"`
	OrderStatus string `json:"orderStatus"`
	TotalAmount int    `json:"totalAmount"`
	Message     string `json:"message"`
}

type OrderSummaryResponse struct {
	OrderNo     string `json:"orderNo"`
	TerminalNo  string `json:"terminalNo"`
	OrderStatus string `json:"orderStatus"`
	TotalAmount int    `json:"totalAmount"`
}

type OrderDetailDB struct {
	ID          int    `json:"id"`
	OrderNo     string `json:"orderNo"`
	TerminalNo  string `json:"terminalNo"`
	OrderStatus string `json:"orderStatus"`
	ItemNo      int    `json:"itemNo"`
	MenuName    string `json:"menuName"`
	UnitPrice   int    `json:"unitPrice"`
	Quantity    int    `json:"quantity"`
	Subtotal    int    `json:"subtotal"`
	CreatedAt   string `json:"createdAt"`
}

// 1. POST /api/orders
func CreateOrderHandler(w http.ResponseWriter, rInterval *http.Request) {
	var req OrderReq
	if err := json.NewDecoder(rInterval.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Logging Incoming Request
	reqJSON, _ := json.Marshal(req)
	log.Printf("[Incoming Request] %s", string(reqJSON))

	// Validation
	if req.TerminalNo == "" || req.MessageType != "ORDER_CONFIRM" || req.TotalAmount < 1 || len(req.Items) < 1 || len(req.Items) > 5 {
		http.Error(w, "Validation Error: Header Checks Failed", http.StatusBadRequest)
		return
	}

	calculatedTotal := 0
	menuMap := make(map[string]bool)

	for _, item := range req.Items {
		if item.MenuName == "" || item.UnitPrice < 1 || item.Quantity < 1 || item.Quantity > 5 {
			http.Error(w, "Validation Error: Item Checks Failed", http.StatusBadRequest)
			return
		}
		if menuMap[item.MenuName] {
			http.Error(w, "Validation Error: Duplicate Menu Items Not Allowed", http.StatusBadRequest)
			return
		}
		menuMap[item.MenuName] = true
		calculatedTotal += item.UnitPrice * item.Quantity
	}

	if calculatedTotal != req.TotalAmount {
		http.Error(w, "Validation Error: Total Amount Mismatch", http.StatusBadRequest)
		return
	}

	// Generate Order Number
	orderNo, err := GenerateOrderNo()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	initialStatus := "オーダー受信"

	// Insert into DB
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	for i, item := range req.Items {
		subtotal := item.UnitPrice * item.Quantity
		itemNo := i + 1
		query := `INSERT INTO order_items (order_no, terminal_no, order_status, item_no, menu_name, unit_price, quantity, subtotal) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
		
		log.Printf("[DB Insert Item %d] OrderNo: %s, Item: %s, Qty: %d, Subtotal: %d", itemNo, orderNo, item.MenuName, item.Quantity, subtotal)
		
		_, err := tx.Exec(query, orderNo, req.TerminalNo, initialStatus, itemNo, item.MenuName, item.UnitPrice, item.Quantity, subtotal)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
	tx.Commit()

	// Return Success Response
	res := SuccessResponse{
		Result:      "OK",
		OrderNo:     orderNo,
		OrderStatus: initialStatus,
		TotalAmount: req.TotalAmount,
		Message:     "注文を受け付けました",
	}

	resJSON, _ := json.Marshal(res)
	log.Printf("[Outgoing Response] %s", string(resJSON))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(resJSON)
}

// 2. GET /api/orders (List & Filtered View)
func GetOrdersHandler(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	var rows *sql.Rows
	var err error

	query := `SELECT order_no, terminal_no, order_status, SUM(subtotal) FROM order_items `
	if statusFilter != "" {
		query += `WHERE order_status = ? GROUP BY order_no`
		rows, err = db.Query(query, statusFilter)
	} else {
		query += `GROUP BY order_no`
		rows, err = db.Query(query)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	orders := []OrderSummaryResponse{}
	for rows.Next() {
		var o OrderSummaryResponse
		if err := rows.Scan(&o.OrderNo, &o.TerminalNo, &o.OrderStatus, &o.TotalAmount); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		orders = append(orders, o)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

// 3. GET /api/orders/{orderNo}
func GetOrderDetailsHandler(w http.ResponseWriter, r *http.Request) {
	orderNo := r.PathValue("orderNo")

	query := `SELECT id, order_no, terminal_no, order_status, item_no, menu_name, unit_price, quantity, subtotal, created_at FROM order_items WHERE order_no = ?`
	rows, err := db.Query(query, orderNo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	details := []OrderDetailDB{}
	for rows.Next() {
		var d OrderDetailDB
		if err := rows.Scan(&d.ID, &d.OrderNo, &d.TerminalNo, &d.OrderStatus, &d.ItemNo, &d.MenuName, &d.UnitPrice, &d.Quantity, &d.Subtotal, &d.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		details = append(details, d)
	}

	if len(details) == 0 {
		http.Error(w, "Order Not Found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(details)
}

// 4. PUT /api/orders/{orderNo}/status
func UpdateOrderStatusHandler(w http.ResponseWriter, r *http.Request) {
	orderNo := r.PathValue("orderNo")

	var req OrderStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	validStatuses := map[string]bool{"オーダー受信": true, "クッキング終了": true, "受け渡し終了": true}
	if !validStatuses[req.OrderStatus] {
		http.Error(w, "Invalid Status Value", http.StatusBadRequest)
		return
	}

	query := `UPDATE order_items SET order_status = ? WHERE order_no = ?`
	result, err := db.Exec(query, req.OrderStatus, orderNo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Order Not Found", http.StatusNotFound)
		return
	}

	log.Printf("[DB Update Status] OrderNo: %s updated to Status: %s", orderNo, req.OrderStatus)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"result":"OK", "message":"Status updated to %s"}`, req.OrderStatus)
}
