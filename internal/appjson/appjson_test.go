package appjson

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMarshalProfilesResponse(t *testing.T) {
	response, err := MarshalProfilesResponse([]Profile{
		{
			Name:        "dev-admin",
			AccountName: "Development",
			AccountID:   "123456789012",
			RoleName:    "AdministratorAccess",
			Region:      "eu-west-1",
		},
	})
	if err != nil {
		t.Fatalf("MarshalProfilesResponse() error = %v", err)
	}

	var got struct {
		Status  string `json:"status"`
		Payload struct {
			Profiles []Profile `json:"profiles"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(response, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Status != "success" {
		t.Fatalf("status = %q, want success", got.Status)
	}
	if len(got.Payload.Profiles) != 1 {
		t.Fatalf("profiles length = %d, want 1", len(got.Payload.Profiles))
	}
	if got.Payload.Profiles[0].Name != "dev-admin" {
		t.Fatalf("profile name = %q, want dev-admin", got.Payload.Profiles[0].Name)
	}
}

func TestMarshalSessionStatusResponse(t *testing.T) {
	expiresAt := time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
	response, err := MarshalSessionStatusResponse(SessionStatusPayload{
		HasProfiles: true,
		SelectedProfile: &Profile{
			Name:        "prod-admin",
			AccountName: "Production",
			AccountID:   "210987654321",
			RoleName:    "AdministratorAccess",
			Region:      "us-east-1",
		},
		CacheExpiresAt: &expiresAt,
	})
	if err != nil {
		t.Fatalf("MarshalSessionStatusResponse() error = %v", err)
	}

	var got struct {
		Status  string               `json:"status"`
		Payload SessionStatusPayload `json:"payload"`
	}
	if err := json.Unmarshal(response, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Status != "success" {
		t.Fatalf("status = %q, want success", got.Status)
	}
	if !got.Payload.HasProfiles {
		t.Fatal("HasProfiles = false, want true")
	}
	if got.Payload.SelectedProfile == nil || got.Payload.SelectedProfile.Name != "prod-admin" {
		t.Fatal("SelectedProfile not marshaled correctly")
	}
	if got.Payload.CacheExpiresAt == nil || !got.Payload.CacheExpiresAt.Equal(expiresAt) {
		t.Fatal("CacheExpiresAt not marshaled correctly")
	}
}

func TestMarshalErrorResponse(t *testing.T) {
	response, err := MarshalErrorResponse("auth_expired", "Login required", map[string]string{
		"retry": "true",
	})
	if err != nil {
		t.Fatalf("MarshalErrorResponse() error = %v", err)
	}

	var got struct {
		Status  string            `json:"status"`
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Payload map[string]string `json:"payload"`
	}
	if err := json.Unmarshal(response, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Status != "recoverable_error" {
		t.Fatalf("status = %q, want recoverable_error", got.Status)
	}
	if got.Code != "auth_expired" {
		t.Fatalf("code = %q, want auth_expired", got.Code)
	}
	if got.Message != "Login required" {
		t.Fatalf("message = %q, want Login required", got.Message)
	}
	if got.Payload["retry"] != "true" {
		t.Fatalf("payload retry = %q, want true", got.Payload["retry"])
	}
}
