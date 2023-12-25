package models

import "time"

type APIRegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type APIAuthRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type APIAddOrderRequest struct {
	UserID      string
	OrderNumber string
}

type APIGetOrderResponse struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    *int      `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type APIGetBonusesAmountResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type APIUseBonusesRequest struct {
	OrderNumber string  `json:"order"`
	Sum         float64 `json:"sum"`
}
