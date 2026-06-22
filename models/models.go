package models

import "time"

const (
	FenPerYuan = 100
)

type Member struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
}

type StoredCard struct {
	ID         string    `json:"id"`
	MemberID   string    `json:"member_id"`
	BalanceFen int64     `json:"-"`
	Balance    float64   `json:"balance"`
	Currency   string    `json:"currency"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Transaction struct {
	ID                 string    `json:"id"`
	CardID             string    `json:"card_id"`
	MemberID           string    `json:"member_id"`
	Type               string    `json:"type"`
	AmountFen          int64     `json:"-"`
	Amount             float64   `json:"amount"`
	BeforeBalanceFen   int64     `json:"-"`
	BeforeBalance      float64   `json:"before_balance"`
	AfterBalanceFen    int64     `json:"-"`
	AfterBalance       float64   `json:"after_balance"`
	Description        string    `json:"description"`
	CreatedAt          time.Time `json:"created_at"`
}

type BalanceResponse struct {
	MemberID   string     `json:"member_id"`
	MemberName string     `json:"member_name"`
	Cards      []CardInfo `json:"cards"`
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
	RequestID   string  `json:"request_id,omitempty"`
	Description string  `json:"description"`
}

type CardDeductDetail struct {
	CardID       string  `json:"card_id"`
	BeforeAmount float64 `json:"before_amount"`
	DeductAmount float64 `json:"deduct_amount"`
	AfterAmount  float64 `json:"after_amount"`
}

type DeductResponse struct {
	Success       bool               `json:"success"`
	Message       string             `json:"message"`
	RequestID     string             `json:"request_id,omitempty"`
	TotalDeduct   float64            `json:"total_deduct,omitempty"`
	TransactionID string             `json:"transaction_id,omitempty"`
	Details       []CardDeductDetail `json:"details,omitempty"`
}

func YuanToFen(yuan float64) int64 {
	return int64(yuan*float64(FenPerYuan) + 0.5)
}

func FenToYuan(fen int64) float64 {
	return float64(fen) / float64(FenPerYuan)
}

func (c *StoredCard) SyncBalanceFromFen() {
	c.Balance = FenToYuan(c.BalanceFen)
}

func (t *Transaction) SyncAmountsFromFen() {
	t.Amount = FenToYuan(t.AmountFen)
	t.BeforeBalance = FenToYuan(t.BeforeBalanceFen)
	t.AfterBalance = FenToYuan(t.AfterBalanceFen)
}
