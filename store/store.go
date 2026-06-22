package store

import (
	"errors"
	"fmt"
	"storedvalue/models"
	"sync"
	"time"
)

var (
	ErrMemberNotFound    = errors.New("会员不存在")
	ErrCardNotFound      = errors.New("储值卡不存在")
	ErrCardNotActive     = errors.New("储值卡未激活")
	ErrInsufficientBalance = errors.New("余额不足")
	ErrInvalidAmount     = errors.New("扣款金额必须大于0")
	ErrCardMemberMismatch = errors.New("储值卡与会员不匹配")
)

type MemoryStore struct {
	mu           sync.RWMutex
	members      map[string]*models.Member
	cards        map[string]*models.StoredCard
	transactions map[string]*models.Transaction
	cardSeq      int64
	txSeq        int64
}

func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		members:      make(map[string]*models.Member),
		cards:        make(map[string]*models.StoredCard),
		transactions: make(map[string]*models.Transaction),
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

	cards := []models.StoredCard{
		{ID: "C001", MemberID: "M001", Balance: 5000.00, Currency: "CNY", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "C002", MemberID: "M001", Balance: 300.50, Currency: "CNY", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "C003", MemberID: "M002", Balance: 1500.00, Currency: "CNY", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "C004", MemberID: "M003", Balance: 50.00, Currency: "CNY", Status: "frozen", CreatedAt: now, UpdatedAt: now},
	}

	for i := range cards {
		s.cards[cards[i].ID] = &cards[i]
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
	return card, nil
}

func (s *MemoryStore) DeductBalance(memberID, cardID string, amount float64, description string) (*models.Transaction, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.members[memberID]; !ok {
		return nil, ErrMemberNotFound
	}

	var targetCard *models.StoredCard

	if cardID != "" {
		card, ok := s.cards[cardID]
		if !ok {
			return nil, ErrCardNotFound
		}
		if card.MemberID != memberID {
			return nil, ErrCardMemberMismatch
		}
		targetCard = card
	} else {
		for _, card := range s.cards {
			if card.MemberID == memberID && card.Status == "active" {
				if targetCard == nil || card.Balance > targetCard.Balance {
					targetCard = card
				}
			}
		}
		if targetCard == nil {
			return nil, ErrCardNotFound
		}
	}

	if targetCard.Status != "active" {
		return nil, ErrCardNotActive
	}

	if targetCard.Balance < amount {
		return nil, fmt.Errorf("%w: 当前余额 %.2f，需扣款 %.2f", ErrInsufficientBalance, targetCard.Balance, amount)
	}

	beforeBalance := targetCard.Balance
	targetCard.Balance -= amount
	targetCard.UpdatedAt = time.Now()

	s.txSeq++
	tx := &models.Transaction{
		ID:            fmt.Sprintf("TX%010d", s.txSeq),
		CardID:        targetCard.ID,
		MemberID:      memberID,
		Type:          "deduct",
		Amount:        amount,
		BeforeBalance: beforeBalance,
		AfterBalance:  targetCard.Balance,
		Description:   description,
		CreatedAt:     time.Now(),
	}
	s.transactions[tx.ID] = tx

	return tx, nil
}
