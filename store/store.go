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
	ErrCardNotFound        = errors.New("储值卡不存在")
	ErrCardNotActive       = errors.New("储值卡未激活")
	ErrInsufficientBalance = errors.New("余额不足")
	ErrInvalidAmount       = errors.New("扣款金额必须大于0")
	ErrCardMemberMismatch  = errors.New("储值卡与会员不匹配")
	ErrNegativeBalancePanic = errors.New("系统错误：扣款后余额为负数，已自动回滚")
	ErrDuplicateRequest    = errors.New("重复请求")
	ErrInvalidDiscountRate = errors.New("折扣率必须在 0.01 ~ 1.00 之间")
	ErrTierOverlap         = errors.New("折扣档位区间重叠")
)

const MaxOverdraftFen int64 = 0

type DiscountCalculation struct {
	TierID            string
	TierName          string
	DiscountRate      float64
	OriginalAmountFen int64
	DiscountAmountFen int64
	SavedAmountFen    int64
}

type CardDeductResult struct {
	CardID               string
	BeforeBalanceFen     int64
	OriginalDeductFen    int64
	AppliedDiscountRate  float64
	SavedFen             int64
	DiscountDeductFen    int64
	AfterBalanceFen      int64
	TierID               string
	TierName             string
}

type DeductResult struct {
	TransactionIDs       []string
	TotalOriginalFen     int64
	TotalDiscountRate    float64
	TotalSavedFen        int64
	TotalDeductFen       int64
	AppliedTier          *models.DiscountTier
	Details              []CardDeductResult
}

type MemoryStore struct {
	mu            sync.RWMutex
	members       map[string]*models.Member
	cards         map[string]*models.StoredCard
	transactions  map[string]*models.Transaction
	discountTiers []*models.DiscountTier
	idempotency   map[string]string
	cardSeq       int64
	txSeq         int64
	tierSeq       int64
}

func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		members:       make(map[string]*models.Member),
		cards:         make(map[string]*models.StoredCard),
		transactions:  make(map[string]*models.Transaction),
		discountTiers: make([]*models.DiscountTier, 0),
		idempotency:   make(map[string]string),
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
		id         string
		memberID   string
		balanceFen int64
		status     string
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
	s.tierSeq = 0

	defaultTiers := []struct {
		name           string
		minBalanceFen  int64
		maxBalanceFen  int64
		discountRate   float64
		desc           string
	}{
		{"普通会员", 0, 99999, 1.00, "无折扣，建议充值至 1000 元享 95 折"},
		{"白银会员", 100000, 299999, 0.95, "享 95 折优惠，再充 2000 元升级黄金会员享 90 折"},
		{"黄金会员", 300000, 499999, 0.90, "享 90 折优惠，再充 2000 元升级铂金会员享 85 折"},
		{"铂金会员", 500000, 999999, 0.85, "享 85 折优惠，再充 5000 元升级钻石会员享 80 折"},
		{"钻石会员", 1000000, 1 << 62, 0.80, "最高等级！享全场 80 折超值优惠"},
	}

	for _, t := range defaultTiers {
		s.tierSeq++
		tier := &models.DiscountTier{
			ID:             fmt.Sprintf("T%03d", s.tierSeq),
			TierName:       t.name,
			MinBalanceFen:  t.minBalanceFen,
			MaxBalanceFen:  t.maxBalanceFen,
			DiscountRate:   t.discountRate,
			Description:    t.desc,
		}
		tier.SyncFromFen()
		s.discountTiers = append(s.discountTiers, tier)
	}
	s.sortTiers()
}

func (s *MemoryStore) sortTiers() {
	sort.Slice(s.discountTiers, func(i, j int) bool {
		return s.discountTiers[i].MinBalanceFen < s.discountTiers[j].MinBalanceFen
	})
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

func (s *MemoryStore) GetDiscountTiers() []models.DiscountTier {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.DiscountTier, 0, len(s.discountTiers))
	for _, t := range s.discountTiers {
		cp := *t
		cp.SyncFromFen()
		result = append(result, cp)
	}
	return result
}

func (s *MemoryStore) getMaxActiveBalanceFen(memberID string, cards []*models.StoredCard) int64 {
	var maxBal int64
	for _, c := range cards {
		if c.Status == "active" && c.BalanceFen > maxBal {
			maxBal = c.BalanceFen
		}
	}
	return maxBal
}

func (s *MemoryStore) GetMemberTierInfo(memberID string) (current *models.DiscountTier, next *models.DiscountTier, amountToNextFen int64, maxBalanceFen int64, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.members[memberID]; !ok {
		return nil, nil, 0, 0, ErrMemberNotFound
	}

	var memberCards []*models.StoredCard
	for _, c := range s.cards {
		if c.MemberID == memberID {
			memberCards = append(memberCards, c)
		}
	}
	maxBalanceFen = s.getMaxActiveBalanceFen(memberID, memberCards)

	current = s.matchTierLocked(maxBalanceFen)
	if current != nil {
		next = s.findNextTierLocked(current)
		if next != nil {
			amountToNextFen = next.MinBalanceFen - maxBalanceFen
			if amountToNextFen < 0 {
				amountToNextFen = 0
			}
		}
		cp := *current
		cp.SyncFromFen()
		current = &cp
		if next != nil {
			ncp := *next
			ncp.SyncFromFen()
			next = &ncp
		}
	}
	return
}

func (s *MemoryStore) matchTierLocked(balanceFen int64) *models.DiscountTier {
	for _, t := range s.discountTiers {
		if balanceFen >= t.MinBalanceFen && balanceFen <= t.MaxBalanceFen {
			return t
		}
	}
	if len(s.discountTiers) > 0 {
		return s.discountTiers[0]
	}
	return nil
}

func (s *MemoryStore) findNextTierLocked(current *models.DiscountTier) *models.DiscountTier {
	for _, t := range s.discountTiers {
		if t.MinBalanceFen > current.MinBalanceFen {
			return t
		}
	}
	return nil
}

func calcDiscountFen(originalFen int64, rate float64) (discountFen int64, savedFen int64) {
	exact := float64(originalFen) * rate
	discountFen = int64(exact + 0.5)
	if discountFen < 1 && originalFen > 0 {
		discountFen = 1
	}
	if discountFen > originalFen {
		discountFen = originalFen
	}
	savedFen = originalFen - discountFen
	if savedFen < 0 {
		savedFen = 0
	}
	return
}

func (s *MemoryStore) DeductBalance(memberID, cardID, requestID string, originalAmountFen int64, description string) (*DeductResult, error) {
	if originalAmountFen <= 0 {
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

	maxBalanceForTier := s.getMaxActiveBalanceFen(memberID, targetCards)
	appliedTier := s.matchTierLocked(maxBalanceForTier)
	if appliedTier == nil {
		appliedTier = &models.DiscountTier{
			ID:           "T000",
			TierName:     "默认",
			DiscountRate: 1.00,
		}
	}

	discountRate := appliedTier.DiscountRate
	if discountRate <= 0 || discountRate > 1.0 {
		discountRate = 1.0
	}

	totalDiscountAmountFen, _ := calcDiscountFen(originalAmountFen, discountRate)
	totalSavedFen := originalAmountFen - totalDiscountAmountFen
	if totalSavedFen < 0 {
		totalSavedFen = 0
	}

	var totalAvailable int64
	for _, c := range targetCards {
		totalAvailable += c.BalanceFen
	}
	if totalAvailable < totalDiscountAmountFen {
		return nil, fmt.Errorf("%w: 折扣后应扣 %s(原价 %s, 折扣 %.0f%%, 省 %s)，可用总余额 %s",
			ErrInsufficientBalance,
			fmt.Sprintf("%.2f", models.FenToYuan(totalDiscountAmountFen)),
			fmt.Sprintf("%.2f", models.FenToYuan(originalAmountFen)),
			discountRate*100,
			fmt.Sprintf("%.2f", models.FenToYuan(totalSavedFen)),
			fmt.Sprintf("%.2f", models.FenToYuan(totalAvailable)))
	}

	result := &DeductResult{
		TotalOriginalFen:  originalAmountFen,
		TotalDiscountRate: discountRate,
		TotalSavedFen:     totalSavedFen,
		TotalDeductFen:    0,
		AppliedTier:       appliedTier,
		Details:           make([]CardDeductResult, 0, len(targetCards)),
	}

	remaining := totalDiscountAmountFen
	remainingOriginal := originalAmountFen
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

		cardRatio := float64(deductThisCard) / float64(totalDiscountAmountFen)
		origThisCard := int64(float64(remainingOriginal)*cardRatio + 0.5)
		if origThisCard < 0 {
			origThisCard = 0
		}
		savedThisCard := origThisCard - deductThisCard
		if savedThisCard < 0 {
			savedThisCard = 0
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
			ID:                  txID,
			CardID:              card.ID,
			MemberID:            memberID,
			Type:                "deduct",
			AmountFen:           deductThisCard,
			OriginalAmountFen:   origThisCard,
			AppliedDiscountRate: discountRate,
			SavedAmountFen:      savedThisCard,
			TierID:              appliedTier.ID,
			TierName:            appliedTier.TierName,
			BeforeBalanceFen:    beforeFen,
			AfterBalanceFen:     afterFen,
			Description:         description,
			CreatedAt:           now,
		}
		tx.SyncAmountsFromFen()
		s.transactions[txID] = tx
		createdTxIDs = append(createdTxIDs, txID)

		result.TransactionIDs = append(result.TransactionIDs, txID)
		result.TotalDeductFen += deductThisCard
		result.Details = append(result.Details, CardDeductResult{
			CardID:              card.ID,
			BeforeBalanceFen:    beforeFen,
			OriginalDeductFen:   origThisCard,
			AppliedDiscountRate: discountRate,
			SavedFen:            savedThisCard,
			DiscountDeductFen:   deductThisCard,
			AfterBalanceFen:     afterFen,
			TierID:              appliedTier.ID,
			TierName:            appliedTier.TierName,
		})

		remaining -= deductThisCard
		remainingOriginal -= origThisCard
		if remainingOriginal < 0 {
			remainingOriginal = 0
		}
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
