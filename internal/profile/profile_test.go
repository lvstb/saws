package profile

import (
	"testing"
)

func TestValidateStartURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid awsapps URL", "https://my-org.awsapps.com/start", false},
		{"valid awsapps URL with trailing slash", "https://my-org.awsapps.com/start/", false},
		{"valid custom URL", "https://sso.example.com/start", false},
		{"empty string", "", true},
		{"no scheme", "my-org.awsapps.com/start", true},
		{"http scheme", "http://my-org.awsapps.com/start", false}, // http is allowed (loose validation)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStartURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStartURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAccountID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid 12 digits", "123456789012", false},
		{"empty string", "", true},
		{"too short", "12345", true},
		{"too long", "1234567890123", true},
		{"contains letters", "12345678901a", true},
		{"contains dashes", "123-456-7890", true},
		{"all zeros", "000000000000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAccountID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAccountID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRoleName(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		wantErr bool
	}{
		{"valid role", "AdministratorAccess", false},
		{"valid with dashes", "my-role-name", false},
		{"empty string", "", true},
		{"whitespace only", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRoleName(tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRoleName(%q) error = %v, wantErr %v", tt.role, err, tt.wantErr)
			}
		})
	}
}

func TestValidateProfileName(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		wantErr bool
	}{
		{"valid simple name", "my-profile", false},
		{"valid default", "default", false},
		{"empty string", "", true},
		{"contains bracket", "my[profile]", true},
		{"whitespace only", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfileName(tt.profile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfileName(%q) error = %v, wantErr %v", tt.profile, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRegion(t *testing.T) {
	tests := []struct {
		name    string
		region  string
		wantErr bool
	}{
		{"valid us-east-1", "us-east-1", false},
		{"valid eu-west-1", "eu-west-1", false},
		{"valid ap-southeast-1", "ap-southeast-1", false},
		{"empty string", "", true},
		{"invalid region", "us-invalid-1", true},
		{"made up region", "mars-west-1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegion(tt.region)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRegion(%q) error = %v, wantErr %v", tt.region, err, tt.wantErr)
			}
		})
	}
}

func TestSSOProfile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		profile SSOProfile
		wantErr bool
	}{
		{
			name: "all fields valid",
			profile: SSOProfile{
				Name:      "my-profile",
				StartURL:  "https://my-org.awsapps.com/start",
				Region:    "us-east-1",
				AccountID: "123456789012",
				RoleName:  "AdministratorAccess",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			profile: SSOProfile{
				StartURL:  "https://my-org.awsapps.com/start",
				Region:    "us-east-1",
				AccountID: "123456789012",
				RoleName:  "AdministratorAccess",
			},
			wantErr: true,
		},
		{
			name: "invalid account ID",
			profile: SSOProfile{
				Name:      "my-profile",
				StartURL:  "https://my-org.awsapps.com/start",
				Region:    "us-east-1",
				AccountID: "bad",
				RoleName:  "AdministratorAccess",
			},
			wantErr: true,
		},
		{
			name: "invalid region",
			profile: SSOProfile{
				Name:      "my-profile",
				StartURL:  "https://my-org.awsapps.com/start",
				Region:    "invalid-region",
				AccountID: "123456789012",
				RoleName:  "AdministratorAccess",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SSOProfile.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSSOProfile_DisplayName(t *testing.T) {
	t.Run("without account name", func(t *testing.T) {
		p := SSOProfile{
			Name:      "dev",
			AccountID: "123456789012",
			RoleName:  "ReadOnly",
		}
		got := p.DisplayName()
		want := "dev (123456789012 / ReadOnly)"
		if got != want {
			t.Errorf("DisplayName() = %q, want %q", got, want)
		}
	})

	t.Run("with account name", func(t *testing.T) {
		p := SSOProfile{
			Name:        "dev",
			AccountID:   "123456789012",
			AccountName: "Development",
			RoleName:    "ReadOnly",
		}
		got := p.DisplayName()
		want := "dev (Development / ReadOnly)"
		if got != want {
			t.Errorf("DisplayName() = %q, want %q", got, want)
		}
	})
}

func TestGroupByAccount(t *testing.T) {
	profiles := []SSOProfile{
		{Name: "dev-admin", StartURL: "https://org.awsapps.com/start", Region: "us-east-1", AccountID: "111111111111", AccountName: "Development", RoleName: "Admin"},
		{Name: "dev-readonly", StartURL: "https://org.awsapps.com/start", Region: "us-east-1", AccountID: "111111111111", AccountName: "Development", RoleName: "ReadOnly"},
		{Name: "prod-admin", StartURL: "https://org.awsapps.com/start", Region: "us-east-1", AccountID: "222222222222", AccountName: "Production", RoleName: "Admin"},
		{Name: "staging", StartURL: "https://other.awsapps.com/start", Region: "eu-west-1", AccountID: "111111111111", RoleName: "Admin"},
	}

	groups := GroupByAccount(profiles)

	if len(groups) != 3 {
		t.Fatalf("GroupByAccount() returned %d groups, want 3", len(groups))
	}

	// First group: org/111111111111 with 2 roles, should have account name
	if groups[0].AccountID != "111111111111" {
		t.Errorf("group[0].AccountID = %q, want 111111111111", groups[0].AccountID)
	}
	if groups[0].AccountName != "Development" {
		t.Errorf("group[0].AccountName = %q, want Development", groups[0].AccountName)
	}
	if len(groups[0].Roles) != 2 {
		t.Errorf("group[0] has %d roles, want 2", len(groups[0].Roles))
	}

	// Second group: org/222222222222 with 1 role
	if groups[1].AccountID != "222222222222" {
		t.Errorf("group[1].AccountID = %q, want 222222222222", groups[1].AccountID)
	}
	if groups[1].AccountName != "Production" {
		t.Errorf("group[1].AccountName = %q, want Production", groups[1].AccountName)
	}
	if len(groups[1].Roles) != 1 {
		t.Errorf("group[1] has %d roles, want 1", len(groups[1].Roles))
	}

	// Third group: different start URL, same account ID, no account name
	if groups[2].StartURL != "https://other.awsapps.com/start" {
		t.Errorf("group[2].StartURL = %q, want https://other.awsapps.com/start", groups[2].StartURL)
	}
	if groups[2].AccountID != "111111111111" {
		t.Errorf("group[2].AccountID = %q, want 111111111111", groups[2].AccountID)
	}
	if groups[2].AccountName != "" {
		t.Errorf("group[2].AccountName = %q, want empty", groups[2].AccountName)
	}
	if len(groups[2].Roles) != 1 {
		t.Errorf("group[2] has %d roles, want 1", len(groups[2].Roles))
	}
}

func TestGroupByAccountEmpty(t *testing.T) {
	groups := GroupByAccount(nil)
	if len(groups) != 0 {
		t.Errorf("GroupByAccount(nil) returned %d groups, want 0", len(groups))
	}
}

func TestGroupByAccountSingleProfile(t *testing.T) {
	profiles := []SSOProfile{
		{Name: "only", StartURL: "https://org.awsapps.com/start", Region: "us-east-1", AccountID: "111111111111", RoleName: "Admin"},
	}
	groups := GroupByAccount(profiles)
	if len(groups) != 1 {
		t.Fatalf("GroupByAccount() returned %d groups, want 1", len(groups))
	}
	if len(groups[0].Roles) != 1 {
		t.Errorf("group has %d roles, want 1", len(groups[0].Roles))
	}
}

func TestAccountGroup_DisplayName(t *testing.T) {
	t.Run("with account name", func(t *testing.T) {
		g := AccountGroup{AccountID: "123456789012", AccountName: "Production", Region: "us-east-1"}
		got := g.DisplayName()
		want := "Production (123456789012)"
		if got != want {
			t.Errorf("DisplayName() = %q, want %q", got, want)
		}
	})

	t.Run("without account name", func(t *testing.T) {
		g := AccountGroup{AccountID: "123456789012", Region: "us-east-1"}
		got := g.DisplayName()
		want := "123456789012"
		if got != want {
			t.Errorf("DisplayName() = %q, want %q", got, want)
		}
	})
}
