package models

import "time"

type Member struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
}

type StoredCard struct {
	ID        string    `json:"id"`
	MemberID  string    `json:"member_id"`
	Balance   float64   `json:"balance"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Transaction struct {
	ID            string    `json:"id"`
	CardID        string    `json:"card_id"`
	MemberID      string    `json:"member_id"`
	Type          string    `json:"type"`
	Amount        float64   `json:"amount"`
	BeforeBalance float64   `json:"before_balance"`
	AfterBalance  float64   `json:"after_balance"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
}

type BalanceResponse struct {
	MemberID  string      `json:"member_id"`
	MemberName string     `json:"member_name"`
	Cards     []CardInfo  `json:"cards"`
}

type CardInfo struct {
	CardID   string  `json:"card_id"`
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency"`
	Status   string  `json:"status"`
}

type DeductRequest struct {
	MemberID    string  `json:"member_id"`
	CardID      string  `json:"card_id,omitempty"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
}

type DeductResponse struct {
	Success      bool      `json:"success"`
	Message      string    `json:"message"`
	CardID       string    `json:"card_id,omitempty"`
	BeforeAmount float64   `json:"before_amount,omitempty"`
	DeductAmount float64   `json:"deduct_amount,omitempty"`
	AfterAmount  float64   `json:"after_amount,omitempty"`
	TransactionID string   `json:"transaction_id,omitempty"`
}
