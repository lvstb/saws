package appjson

import (
	"encoding/json"
	"time"
)

type Profile struct {
	Name        string `json:"name"`
	AccountName string `json:"accountName,omitempty"`
	AccountID   string `json:"accountId"`
	RoleName    string `json:"roleName"`
	Region      string `json:"region"`
}

type SessionStatusPayload struct {
	HasProfiles     bool       `json:"hasProfiles"`
	SelectedProfile *Profile   `json:"selectedProfile,omitempty"`
	CacheExpiresAt  *time.Time `json:"cacheExpiresAt,omitempty"`
}

type profilesResponse struct {
	Status  string `json:"status"`
	Payload struct {
		Profiles []Profile `json:"profiles"`
	} `json:"payload"`
}

type sessionStatusResponse struct {
	Status  string               `json:"status"`
	Payload SessionStatusPayload `json:"payload"`
}

type errorResponse struct {
	Status  string      `json:"status"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Payload interface{} `json:"payload,omitempty"`
}

func MarshalProfilesResponse(profiles []Profile) ([]byte, error) {
	var response profilesResponse
	response.Status = "success"
	response.Payload.Profiles = profiles
	return json.Marshal(response)
}

func MarshalSessionStatusResponse(payload SessionStatusPayload) ([]byte, error) {
	return json.Marshal(sessionStatusResponse{
		Status:  "success",
		Payload: payload,
	})
}

func MarshalErrorResponse(code, message string, payload interface{}) ([]byte, error) {
	return json.Marshal(errorResponse{
		Status:  "recoverable_error",
		Code:    code,
		Message: message,
		Payload: payload,
	})
}
