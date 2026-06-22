package store

import (
	"errors"
	"fmt"
	"sort"
	"storedvalue/models"
	"sync"
	"time"
)

var (
	ErrMemberNotFound      = errors.New("会员不存在")
	ErrCardNotFound    = errors.New("储值卡不存在")
	ErrCardNotActive = errors.New("储值卡未激活")
	ErrInsufficientBalance = errors.New("余额不足")
	ErrInvalidAmount  = errors.New("扣款金额必须大于0")
	ErrCardMemberMismatch = errors.New("储值卡与会员不匹配")
	ErrNegativeBalancePanic = errors.New("系统错误：扣款后余额为负数，已自动回滚")
	ErrDuplicateRequest = errors.New("重复请求")
)

const MaxOverdraftFen int64 = 0

type DeductResult struct {
	TransactionIDs []string
	TotalDeductFen  int64
	Details         []CardDeductResult
}

type CardDeductResult struct {
	CardID           string
	BeforeBalanceFen  int64
	DeductFen        int64
	AfterBalanceFen   int64
}

type MemoryStore struct {
	mu            sync.RWMutex
	members       map[string]*models.Member
	cards         map[string]*models.StoredCard
	transactions  map[string]*models.Transaction
	idempotency   map[string]string
	cardSeq       int64
	txSeq         int64
}

func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		members:      make(map[string]*models.Member),
		cards:        make(map[string]*models.StoredCard),
		transactions: make(map[string]*models.Transaction),
		idempotency:  make(map[string]string),
	}
	store.initMockData()
	return store
}

func (s *MemoryStore) initMockData() {
	now := time.Now()

	members := []models.Member{
		{ID: "M001", Name: "张三", Phone: "13800138001", CreatedAt: now},
		{ID: "M002", Name: "李四", Phone: "13800138002", CreatedAt: now},
		{ID: "M003", Name: "王五", Phone: "13800138003", CreatedAt: now},
	}

	for i := range members {
		s.members[members[i].ID] = &members[i]
	}

	cardsData := []struct {
		id       string
		memberID string
		balanceFen int64
		status    string
	}{
		{"C001", "M001", 500000, "active"},
		{"C002", "M001", 30050, "active"},
		{"C003", "M002", 150000, "active"},
		{"C004", "M003", 5000, "frozen"},
	}

	for _, cd := range cardsData {
		card := &models.StoredCard{
			ID:         cd.id,
			MemberID:   cd.memberID,
			BalanceFen: cd.balanceFen,
			Currency:   "CNY",
			Status:     cd.status,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		card.SyncBalanceFromFen()
		s.cards[card.ID] = card
	}

	s.cardSeq = 4
	s.txSeq = 0
}

func (s *MemoryStore) GetMember(memberID string) (*models.Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	member, ok := s.members[memberID]
	if !ok {
		return nil, ErrMemberNotFound
	}
	return member, nil
}

func (s *MemoryStore) GetCardsByMember(memberID string) ([]*models.StoredCard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.members[memberID]; !ok {
		return nil, ErrMemberNotFound
	}

	var cards []*models.StoredCard
	for _, card := range s.cards {
		if card.MemberID == memberID {
			card.SyncBalanceFromFen()
			cards = append(cards, card)
		}
	}
	return cards, nil
}

func (s *MemoryStore) GetCard(cardID string) (*models.StoredCard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	card, ok := s.cards[cardID]
	if !ok {
		return nil, ErrCardNotFound
	}
	card.SyncBalanceFromFen()
	return card, nil
}

func (s *MemoryStore) DeductBalance(memberID, cardID, requestID string, amountFen int64, description string) (*DeductResult, error) {
	if amountFen <= 0 {
		return nil, ErrInvalidAmount
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if requestID != "" {
		if existingTxID, exists := s.idempotency[requestID]; exists {
			return nil, fmt.Errorf("%w: request_id=%s, first_tx=%s", ErrDuplicateRequest, requestID, existingTxID)
		}
	}

	if _, ok := s.members[memberID]; !ok {
		return nil, ErrMemberNotFound
	}

	var targetCards = make([]*models.StoredCard, 0)

	if cardID != "" {
		card, ok := s.cards[cardID]
		if !ok {
			return nil, ErrCardNotFound
		}
		if card.MemberID != memberID {
			return nil, ErrCardMemberMismatch
		}
		if card.Status != "active" {
			return nil, ErrCardNotActive
		}
		targetCards = append(targetCards, card)
	} else {
		for _, card := range s.cards {
			if card.MemberID == memberID && card.Status == "active" {
				targetCards = append(targetCards, card)
			}
		}
		if len(targetCards) == 0 {
			return nil, ErrCardNotFound
		}
		sort.Slice(targetCards, func(i, j int) bool {
			return targetCards[i].BalanceFen > targetCards[j].BalanceFen
		})
	}

	var totalAvailable int64
	for _, c := range targetCards {
		totalAvailable += c.BalanceFen
	}
	if totalAvailable < amountFen {
		return nil, fmt.Errorf("%w: 可用总余额 %s，需扣款 %s",
			ErrInsufficientBalance,
			fmt.Sprintf("%.2f", models.FenToYuan(totalAvailable)),
			fmt.Sprintf("%.2f", models.FenToYuan(amountFen)))
	}

	result := &DeductResult{
		Details: make([]CardDeductResult, 0, len(targetCards)),
	}

	remaining := amountFen
	snapshot := make(map[string]int64)
	for _, c := range targetCards {
		snapshot[c.ID] = c.BalanceFen
	}

	rollbackAll := func() {
		for cid, prev := range snapshot {
			if card, ok := s.cards[cid]; ok {
				card.BalanceFen = prev
				card.SyncBalanceFromFen()
				card.UpdatedAt = time.Now()
			}
		}
	}

	now := time.Now()
	var createdTxIDs = make([]string, 0, len(targetCards))

	for _, card := range targetCards {
		if remaining <= 0 {
			break
		}
		if card.BalanceFen <= 0 {
			continue
		}

		deductThisCard := card.BalanceFen
		if deductThisCard > remaining {
			deductThisCard = remaining
		}

		beforeFen := card.BalanceFen
		card.BalanceFen -= deductThisCard

		if card.BalanceFen < MaxOverdraftFen {
			rollbackAll()
			return nil, ErrNegativeBalancePanic
		}

		card.SyncBalanceFromFen()
		card.UpdatedAt = now

		if card.BalanceFen < 0 {
			rollbackAll()
			return nil, fmt.Errorf("%w: 卡 %s 余额为负(%.2f)", ErrNegativeBalancePanic, card.ID, card.Balance)
		}

		afterFen := card.BalanceFen

		s.txSeq++
		txID := fmt.Sprintf("TX%010d", s.txSeq)
		tx := &models.Transaction{
			ID:               txID,
			CardID:           card.ID,
			MemberID:         memberID,
			Type:             "deduct",
			AmountFen:        deductThisCard,
			BeforeBalanceFen: beforeFen,
			AfterBalanceFen:  afterFen,
			Description:      description,
			CreatedAt:      now,
		}
		tx.SyncAmountsFromFen()
		s.transactions[txID] = tx
		createdTxIDs = append(createdTxIDs, txID)

		result.TransactionIDs = append(result.TransactionIDs, txID)
		result.TotalDeductFen += deductThisCard
		result.Details = append(result.Details, CardDeductResult{
			CardID:           card.ID,
			BeforeBalanceFen:  beforeFen,
			DeductFen:        deductThisCard,
			AfterBalanceFen:   afterFen,
		})

		remaining -= deductThisCard
	}

	if remaining > 0 {
		rollbackAll()
		for _, txID := range createdTxIDs {
			delete(s.transactions, txID)
		}
		return nil, fmt.Errorf("%w: 多卡扣款后仍有剩余未扣 %s，已回滚",
			ErrInsufficientBalance,
			fmt.Sprintf("%.2f", models.FenToYuan(remaining)))
	}

	for _, detail := range result.Details {
		if c, ok := s.cards[detail.CardID]; ok {
			if c.BalanceFen < 0 {
				rollbackAll()
				return nil, fmt.Errorf("%w: 卡 %s 最终余额为负", ErrNegativeBalancePanic, c.ID)
			}
		}
	}

	if requestID != "" {
		if len(result.TransactionIDs) > 0 {
			s.idempotency[requestID] = result.TransactionIDs[0]
		}
	}

	return result, nil
}
