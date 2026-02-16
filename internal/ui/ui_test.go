package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/lvstb/saws/internal/profile"
)

func TestBanner(t *testing.T) {
	banner := Banner()
	if banner == "" {
		t.Error("Banner() returned empty string")
	}
	if !containsStr(banner, "AWS SSO Credential Helper") {
		t.Error("Banner() does not contain subtitle")
	}
}

func TestFormatKeyValue(t *testing.T) {
	result := FormatKeyValue("Key:", "Value")
	if result == "" {
		t.Error("FormatKeyValue() returned empty string")
	}
	if !containsStr(result, "Key:") {
		t.Error("FormatKeyValue() missing key")
	}
	if !containsStr(result, "Value") {
		t.Error("FormatKeyValue() missing value")
	}
}

func TestSelectorItemFilterValue(t *testing.T) {
	t.Run("account item includes account name, ID, and profile names", func(t *testing.T) {
		g := profile.AccountGroup{
			AccountID:   "123456789012",
			AccountName: "Development",
			Region:      "us-east-1",
			Roles: []profile.SSOProfile{
				{Name: "dev-admin", RoleName: "Admin"},
				{Name: "dev-readonly", RoleName: "ReadOnly"},
			},
		}
		item := selectorItem{kind: kindAccount, account: &g}
		got := item.FilterValue()
		if !containsStr(got, "123456789012") {
			t.Error("FilterValue() missing account ID")
		}
		if !containsStr(got, "Development") {
			t.Error("FilterValue() missing account name")
		}
		if !containsStr(got, "dev-admin") {
			t.Error("FilterValue() missing profile name")
		}
		if !containsStr(got, "us-east-1") {
			t.Error("FilterValue() missing region")
		}
	})

	t.Run("role item includes role name and profile name", func(t *testing.T) {
		p := profile.SSOProfile{Name: "dev-admin", RoleName: "AdminAccess"}
		item := selectorItem{kind: kindRole, profile: &p}
		got := item.FilterValue()
		if !containsStr(got, "AdminAccess") {
			t.Error("FilterValue() missing role name")
		}
		if !containsStr(got, "dev-admin") {
			t.Error("FilterValue() missing profile name")
		}
	})

	t.Run("new item", func(t *testing.T) {
		item := selectorItem{kind: kindNew}
		if item.FilterValue() != addNewProfileLabel {
			t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), addNewProfileLabel)
		}
	})

	t.Run("back item", func(t *testing.T) {
		item := selectorItem{kind: kindBack}
		if item.FilterValue() != backLabel {
			t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), backLabel)
		}
	})
}

func TestSelectorDelegateDimensions(t *testing.T) {
	d := selectorDelegate{}
	if d.Height() != 2 {
		t.Errorf("Height() = %d, want 2", d.Height())
	}
	if d.Spacing() != 0 {
		t.Errorf("Spacing() = %d, want 0", d.Spacing())
	}
}

func TestSawsTheme(t *testing.T) {
	theme := sawsTheme()
	if theme == nil {
		t.Fatal("sawsTheme() returned nil")
	}
}

func TestSelectorModelAccountItems(t *testing.T) {
	groups := []profile.AccountGroup{
		{AccountID: "111111111111", Region: "us-east-1", Roles: []profile.SSOProfile{{Name: "a"}}},
		{AccountID: "222222222222", Region: "eu-west-1", Roles: []profile.SSOProfile{{Name: "b"}, {Name: "c"}}},
	}
	m := selectorModel{groups: groups}
	items := m.accountItems()

	// 2 accounts + 1 "add new"
	if len(items) != 3 {
		t.Fatalf("accountItems() returned %d items, want 3", len(items))
	}

	// Last item should be "add new"
	last := items[2].(selectorItem)
	if last.kind != kindNew {
		t.Error("last item should be kindNew")
	}
}

func TestSelectorModelRoleItems(t *testing.T) {
	g := &profile.AccountGroup{
		AccountID: "111111111111",
		Roles: []profile.SSOProfile{
			{Name: "dev-admin", RoleName: "Admin"},
			{Name: "dev-readonly", RoleName: "ReadOnly"},
		},
	}
	m := selectorModel{}
	items := m.roleItems(g)

	// 1 "back" + 2 roles
	if len(items) != 3 {
		t.Fatalf("roleItems() returned %d items, want 3", len(items))
	}

	first := items[0].(selectorItem)
	if first.kind != kindBack {
		t.Error("first item should be kindBack")
	}

	second := items[1].(selectorItem)
	if second.kind != kindRole {
		t.Error("second item should be kindRole")
	}
	if second.profile.RoleName != "Admin" {
		t.Errorf("second item role = %q, want Admin", second.profile.RoleName)
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSuggestProfileName(t *testing.T) {
	tests := []struct {
		name        string
		accountName string
		roleName    string
		want        string
	}{
		{"basic", "Production", "AdministratorAccess", "production-administratoraccess"},
		{"with spaces", "My Account", "Power User", "my-account-power-user"},
		{"empty account name", "", "Admin", "aws-admin"},
		{"uppercase", "DEV", "ReadOnly", "dev-readonly"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestProfileName(tt.accountName, tt.roleName)
			if got != tt.want {
				t.Errorf("SuggestProfileName(%q, %q) = %q, want %q", tt.accountName, tt.roleName, got, tt.want)
			}
		})
	}
}

func TestGenerateUniqueProfileNames(t *testing.T) {
	t.Run("no duplicates", func(t *testing.T) {
		profiles := []profile.SSOProfile{
			{AccountName: "Production", RoleName: "Admin"},
			{AccountName: "Staging", RoleName: "Admin"},
			{AccountName: "Production", RoleName: "ReadOnly"},
		}
		names := GenerateUniqueProfileNames(profiles)
		if len(names) != 3 {
			t.Fatalf("got %d names, want 3", len(names))
		}
		if names[0] != "production-admin" {
			t.Errorf("names[0] = %q, want %q", names[0], "production-admin")
		}
		if names[1] != "staging-admin" {
			t.Errorf("names[1] = %q, want %q", names[1], "staging-admin")
		}
		if names[2] != "production-readonly" {
			t.Errorf("names[2] = %q, want %q", names[2], "production-readonly")
		}
	})

	t.Run("duplicates get suffix", func(t *testing.T) {
		profiles := []profile.SSOProfile{
			{AccountName: "Development", RoleName: "Admin"},
			{AccountName: "Development", RoleName: "Admin"},
			{AccountName: "Development", RoleName: "Admin"},
		}
		names := GenerateUniqueProfileNames(profiles)
		if len(names) != 3 {
			t.Fatalf("got %d names, want 3", len(names))
		}
		if names[0] != "development-admin" {
			t.Errorf("names[0] = %q, want %q", names[0], "development-admin")
		}
		if names[1] != "development-admin-2" {
			t.Errorf("names[1] = %q, want %q", names[1], "development-admin-2")
		}
		if names[2] != "development-admin-3" {
			t.Errorf("names[2] = %q, want %q", names[2], "development-admin-3")
		}
	})

	t.Run("mixed duplicates and unique", func(t *testing.T) {
		profiles := []profile.SSOProfile{
			{AccountName: "Prod", RoleName: "Admin"},
			{AccountName: "Staging", RoleName: "ReadOnly"},
			{AccountName: "Prod", RoleName: "Admin"},
		}
		names := GenerateUniqueProfileNames(profiles)
		if names[0] != "prod-admin" {
			t.Errorf("names[0] = %q, want %q", names[0], "prod-admin")
		}
		if names[1] != "staging-readonly" {
			t.Errorf("names[1] = %q, want %q", names[1], "staging-readonly")
		}
		if names[2] != "prod-admin-2" {
			t.Errorf("names[2] = %q, want %q", names[2], "prod-admin-2")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		names := GenerateUniqueProfileNames(nil)
		if len(names) != 0 {
			t.Errorf("got %d names for nil input, want 0", len(names))
		}
	})

	t.Run("single profile", func(t *testing.T) {
		profiles := []profile.SSOProfile{
			{AccountName: "Production", RoleName: "Admin"},
		}
		names := GenerateUniqueProfileNames(profiles)
		if len(names) != 1 {
			t.Fatalf("got %d names, want 1", len(names))
		}
		if names[0] != "production-admin" {
			t.Errorf("names[0] = %q, want %q", names[0], "production-admin")
		}
	})
}

func TestRunProfileImportSelector_Empty(t *testing.T) {
	_, err := RunProfileImportSelector(nil)
	if err == nil {
		t.Fatal("expected error for nil input, got nil")
	}

	_, err = RunProfileImportSelector([]DiscoveredProfile{})
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestMatchesFilter(t *testing.T) {
	item := selectorItem{kind: kindRole, profile: &profile.SSOProfile{Name: "dev-admin", RoleName: "AdminAccess"}}

	t.Run("empty term matches everything", func(t *testing.T) {
		if !matchesFilter(item, "") {
			t.Error("empty term should match")
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		if !matchesFilter(item, "adminaccess") {
			t.Error("should match case-insensitively")
		}
	})

	t.Run("no match", func(t *testing.T) {
		if matchesFilter(item, "nonexistent") {
			t.Error("should not match")
		}
	})
}

func TestFilterItems(t *testing.T) {
	items := []list.Item{
		importItem{index: 0, accountName: "Production", roleName: "Admin", profileName: "prod-admin", accountID: "111"},
		importItem{index: 1, accountName: "Staging", roleName: "ReadOnly", profileName: "staging-readonly", accountID: "222"},
		importItem{index: 2, accountName: "Pipeline", roleName: "Deploy", profileName: "pipeline-deploy", accountID: "333"},
		importItem{index: 3, accountName: "Development", roleName: "Admin", profileName: "dev-admin", accountID: "444"},
	}

	t.Run("exact substring match", func(t *testing.T) {
		got := filterItems(items, "Pipeline")
		if len(got) != 1 {
			t.Fatalf("got %d matches, want 1", len(got))
		}
		if got[0].(importItem).index != 2 {
			t.Errorf("matched index = %d, want 2", got[0].(importItem).index)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		got := filterItems(items, "pipeline")
		if len(got) != 1 {
			t.Fatalf("got %d matches, want 1", len(got))
		}
	})

	t.Run("multiple matches", func(t *testing.T) {
		got := filterItems(items, "admin")
		if len(got) != 2 {
			t.Fatalf("got %d matches, want 2", len(got))
		}
		if got[0].(importItem).index != 0 || got[1].(importItem).index != 3 {
			t.Errorf("matched indices = [%d, %d], want [0, 3]",
				got[0].(importItem).index, got[1].(importItem).index)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		got := filterItems(items, "nonexistent")
		if len(got) != 0 {
			t.Fatalf("got %d matches, want 0", len(got))
		}
	})

	t.Run("empty term matches all", func(t *testing.T) {
		got := filterItems(items, "")
		if len(got) != len(items) {
			t.Fatalf("got %d matches, want %d", len(got), len(items))
		}
	})

	t.Run("preserves original order", func(t *testing.T) {
		got := filterItems(items, "prod")
		if len(got) != 1 {
			t.Fatalf("got %d matches, want 1", len(got))
		}
		if got[0].(importItem).index != 0 {
			t.Errorf("matched index = %d, want 0", got[0].(importItem).index)
		}
	})
}
