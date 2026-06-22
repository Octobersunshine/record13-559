package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"storedvalue/models"
	"storedvalue/store"
	"strings"
)

type Handler struct {
	store *store.MemoryStore
}

func NewHandler(s *store.MemoryStore) *Handler {
	return &Handler{store: s}
}

type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp *apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("write json error: %v", err)
	}
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, &apiResponse{
			Code:    http.StatusMethodNotAllowed,
			Message: "方法不允许，请使用 GET 请求",
		})
		return
	}

	memberID := r.URL.Query().Get("member_id")
	if strings.TrimSpace(memberID) == "" {
		writeJSON(w, http.StatusBadRequest, &apiResponse{
			Code:    http.StatusBadRequest,
			Message: "缺少 member_id 参数",
		})
		return
	}

	member, err := h.store.GetMember(memberID)
	if err != nil {
		if errors.Is(err, store.ErrMemberNotFound) {
			writeJSON(w, http.StatusNotFound, &apiResponse{
				Code:    http.StatusNotFound,
				Message: err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, &apiResponse{
			Code:    http.StatusInternalServerError,
			Message: "查询失败: " + err.Error(),
		})
		return
	}

	cards, err := h.store.GetCardsByMember(memberID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, &apiResponse{
			Code:    http.StatusInternalServerError,
			Message: "查询储值卡失败: " + err.Error(),
		})
		return
	}

	cardInfos := make([]models.CardInfo, 0, len(cards))
	for _, c := range cards {
		cardInfos = append(cardInfos, models.CardInfo{
			CardID:   c.ID,
			Balance:  c.Balance,
			Currency: c.Currency,
			Status:   c.Status,
		})
	}

	resp := &models.BalanceResponse{
		MemberID:   member.ID,
		MemberName: member.Name,
		Cards:      cardInfos,
	}

	writeJSON(w, http.StatusOK, &apiResponse{
		Code:    0,
		Message: "查询成功",
		Data:    resp,
	})
}

func (h *Handler) Deduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, &apiResponse{
			Code:    http.StatusMethodNotAllowed,
			Message: "方法不允许，请使用 POST 请求",
		})
		return
	}

	var req models.DeductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, &apiResponse{
			Code:    http.StatusBadRequest,
			Message: "请求体格式错误: " + err.Error(),
		})
		return
	}

	if strings.TrimSpace(req.MemberID) == "" {
		writeJSON(w, http.StatusBadRequest, &apiResponse{
			Code:    http.StatusBadRequest,
			Message: "member_id 不能为空",
		})
		return
	}

	if req.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, &apiResponse{
			Code:    http.StatusBadRequest,
			Message: "扣款金额必须大于 0",
		})
		return
	}

	amountFen := models.YuanToFen(req.Amount)
	if amountFen <= 0 {
		writeJSON(w, http.StatusBadRequest, &apiResponse{
			Code:    http.StatusBadRequest,
			Message: "扣款金额过小（不足 0.01 元）",
		})
		return
	}

	result, err := h.store.DeductBalance(req.MemberID, req.CardID, req.RequestID, amountFen, req.Description)
	if err != nil {
		var statusCode int
		switch {
		case errors.Is(err, store.ErrMemberNotFound):
			statusCode = http.StatusNotFound
		case errors.Is(err, store.ErrCardNotFound):
			statusCode = http.StatusNotFound
		case errors.Is(err, store.ErrCardNotActive):
			statusCode = http.StatusBadRequest
		case errors.Is(err, store.ErrInvalidAmount):
			statusCode = http.StatusBadRequest
		case errors.Is(err, store.ErrCardMemberMismatch):
			statusCode = http.StatusBadRequest
		case errors.Is(err, store.ErrInsufficientBalance):
			statusCode = http.StatusPaymentRequired
		case errors.Is(err, store.ErrDuplicateRequest):
			statusCode = http.StatusConflict
		case errors.Is(err, store.ErrNegativeBalancePanic):
			statusCode = http.StatusInternalServerError
			log.Printf("FATAL 余额负数防护触发: %v", err)
		default:
			statusCode = http.StatusInternalServerError
		}

		deductResp := models.DeductResponse{
			Success:   false,
			Message:   err.Error(),
			RequestID: req.RequestID,
		}

		writeJSON(w, statusCode, &apiResponse{
			Code:    statusCode,
			Message: err.Error(),
			Data:    deductResp,
		})
		return
	}

	details := make([]models.CardDeductDetail, 0, len(result.Details))
	for _, d := range result.Details {
		details = append(details, models.CardDeductDetail{
			CardID:       d.CardID,
			BeforeAmount: models.FenToYuan(d.BeforeBalanceFen),
			DeductAmount: models.FenToYuan(d.DeductFen),
			AfterAmount:  models.FenToYuan(d.AfterBalanceFen),
		})
	}

	var firstTxID string
	if len(result.TransactionIDs) > 0 {
		firstTxID = result.TransactionIDs[0]
	}

	deductResp := models.DeductResponse{
		Success:       true,
		Message:       "扣款成功",
		RequestID:     req.RequestID,
		TotalDeduct:   models.FenToYuan(result.TotalDeductFen),
		TransactionID: firstTxID,
		Details:       details,
	}

	writeJSON(w, http.StatusOK, &apiResponse{
		Code:    0,
		Message: "扣款成功",
		Data:    deductResp,
	})
}
