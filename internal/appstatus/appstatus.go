package appstatus

import (
	"time"

	"github.com/lvstb/saws/internal/appjson"
	"github.com/lvstb/saws/internal/config"
)

type SessionStatus struct {
	HasProfiles     bool
	SelectedProfile *appjson.Profile
	CacheExpiresAt  *time.Time
}

func LoadSessionStatus() (*SessionStatus, error) {
	profiles, err := config.LoadProfiles()
	if err != nil {
		return nil, err
	}

	status := &SessionStatus{
		HasProfiles: len(profiles) > 0,
	}
	if len(profiles) == 0 {
		return status, nil
	}

	selected := profiles[0]
	status.SelectedProfile = &appjson.Profile{
		Name:        selected.Name,
		AccountName: selected.AccountName,
		AccountID:   selected.AccountID,
		RoleName:    selected.RoleName,
		Region:      selected.Region,
	}

	if cached := config.ReadSSOCache(selected.StartURL); cached != nil {
		expiresAt := cached.ExpiresAt
		status.CacheExpiresAt = &expiresAt
	}

	return status, nil
}
