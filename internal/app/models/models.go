package models

type APIRegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type APIAuthRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
