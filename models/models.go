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

type DiscountTier struct {
	ID             string  `json:"id"`
	TierName       string  `json:"tier_name"`
	MinBalanceFen  int64   `json:"-"`
	MaxBalanceFen  int64   `json:"-"`
	MinBalance     float64 `json:"min_balance"`
	MaxBalance     float64 `json:"max_balance"`
	DiscountRate   float64 `json:"discount_rate"`
	Description    string  `json:"description"`
}

type Transaction struct {
	ID                  string    `json:"id"`
	CardID              string    `json:"card_id"`
	MemberID            string    `json:"member_id"`
	Type                string    `json:"type"`
	AmountFen           int64     `json:"-"`
	Amount              float64   `json:"amount"`
	OriginalAmountFen   int64     `json:"-"`
	OriginalAmount      float64   `json:"original_amount,omitempty"`
	AppliedDiscountRate float64   `json:"applied_discount_rate,omitempty"`
	SavedAmountFen      int64     `json:"-"`
	SavedAmount         float64   `json:"saved_amount,omitempty"`
	TierID              string    `json:"tier_id,omitempty"`
	TierName            string    `json:"tier_name,omitempty"`
	BeforeBalanceFen    int64     `json:"-"`
	BeforeBalance       float64   `json:"before_balance"`
	AfterBalanceFen     int64     `json:"-"`
	AfterBalance        float64   `json:"after_balance"`
	Description         string    `json:"description"`
	CreatedAt           time.Time `json:"created_at"`
}

type BalanceResponse struct {
	MemberID         string        `json:"member_id"`
	MemberName       string        `json:"member_name"`
	CurrentTier      *DiscountTier `json:"current_tier,omitempty"`
	NextTier         *DiscountTier `json:"next_tier,omitempty"`
	AmountToNextTier float64       `json:"amount_to_next_tier,omitempty"`
	Cards            []CardInfo    `json:"cards"`
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
	CardID            string  `json:"card_id"`
	BeforeAmount      float64 `json:"before_amount"`
	OriginalDeduct    float64 `json:"original_deduct,omitempty"`
	AppliedRate       float64 `json:"applied_rate,omitempty"`
	SavedAmount       float64 `json:"saved_amount,omitempty"`
	DiscountDeduct    float64 `json:"discount_deduct"`
	AfterAmount       float64 `json:"after_amount"`
}

type DeductResponse struct {
	Success           bool               `json:"success"`
	Message           string             `json:"message"`
	RequestID         string             `json:"request_id,omitempty"`
	TotalOriginal     float64            `json:"total_original,omitempty"`
	TotalDiscountRate float64            `json:"total_discount_rate,omitempty"`
	TotalSaved        float64            `json:"total_saved,omitempty"`
	TotalDeduct       float64            `json:"total_deduct,omitempty"`
	CurrentTier       *DiscountTier      `json:"current_tier,omitempty"`
	TransactionID     string             `json:"transaction_id,omitempty"`
	Details           []CardDeductDetail `json:"details,omitempty"`
}

type DiscountConfigResponse struct {
	Tiers []DiscountTier `json:"tiers"`
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
	t.OriginalAmount = FenToYuan(t.OriginalAmountFen)
	t.SavedAmount = FenToYuan(t.SavedAmountFen)
	t.BeforeBalance = FenToYuan(t.BeforeBalanceFen)
	t.AfterBalance = FenToYuan(t.AfterBalanceFen)
}

func (d *DiscountTier) SyncFromFen() {
	d.MinBalance = FenToYuan(d.MinBalanceFen)
	d.MaxBalance = FenToYuan(d.MaxBalanceFen)
}
