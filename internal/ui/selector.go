package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/lvstb/saws/internal/profile"
)

// substringFilter is a case-insensitive substring filter that replaces the
// default fuzzy filter. It keeps items whose FilterValue contains the search
// term and preserves their original order.
func substringFilter(term string, targets []string) []list.Rank {
	term = strings.ToLower(term)
	var ranks []list.Rank
	for i, t := range targets {
		if strings.Contains(strings.ToLower(t), term) {
			ranks = append(ranks, list.Rank{Index: i})
		}
	}
	return ranks
}

const (
	addNewProfileLabel = "+ Configure new profile"
	backLabel          = "< Back to accounts"
)

// itemKind distinguishes the type of list item.
type itemKind int

const (
	kindAccount itemKind = iota
	kindRole
	kindNew
	kindBack
)

// selectorItem implements list.Item for the fuzzy finder.
type selectorItem struct {
	kind    itemKind
	account *profile.AccountGroup // set for kindAccount
	profile *profile.SSOProfile   // set for kindRole
}

func (i selectorItem) FilterValue() string {
	switch i.kind {
	case kindAccount:
		// Include account name, ID, and profile names so fuzzy search matches broadly
		parts := ""
		if i.account.AccountName != "" {
			parts += i.account.AccountName + " "
		}
		parts += i.account.AccountID + " " + i.account.Region
		for _, r := range i.account.Roles {
			parts += " " + r.Name
		}
		return parts
	case kindRole:
		return i.profile.RoleName + " " + i.profile.Name
	case kindNew:
		return addNewProfileLabel
	case kindBack:
		return backLabel
	default:
		return ""
	}
}

// selectorDelegate renders each item in the list.
type selectorDelegate struct{}

func (d selectorDelegate) Height() int                             { return 2 }
func (d selectorDelegate) Spacing() int                            { return 0 }
func (d selectorDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d selectorDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(selectorItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	var title, desc string
	switch item.kind {
	case kindAccount:
		g := item.account
		if g.AccountName != "" {
			title = g.AccountName
		} else {
			title = g.AccountID
		}
		roleCount := len(g.Roles)
		if roleCount == 1 {
			desc = fmt.Sprintf("%s | %s | %s", g.AccountID, g.Region, g.Roles[0].RoleName)
		} else {
			desc = fmt.Sprintf("%s | %s | %d roles", g.AccountID, g.Region, roleCount)
		}
	case kindRole:
		p := item.profile
		title = p.RoleName
		desc = p.Name
	case kindNew:
		title = addNewProfileLabel
		desc = "Set up a new SSO profile"
	case kindBack:
		title = backLabel
		desc = "Return to account list"
	}

	titleStyle := lipgloss.NewStyle().PaddingLeft(2)
	descStyle := lipgloss.NewStyle().PaddingLeft(2).Foreground(ColorMuted)

	if isSelected {
		titleStyle = titleStyle.Foreground(ColorPrimary).Bold(true)
		descStyle = descStyle.Foreground(ColorPrimary)
		title = "> " + title
	} else {
		titleStyle = titleStyle.Foreground(ColorWhite)
		title = "  " + title
	}

	fmt.Fprintf(w, "%s\n%s", titleStyle.Render(title), descStyle.Render("  "+desc))
}

// selectorLevel tracks whether we're showing accounts or roles.
type selectorLevel int

const (
	levelAccounts selectorLevel = iota
	levelRoles
)

// selectorModel is the bubbletea model for profile selection.
type selectorModel struct {
	list     list.Model
	groups   []profile.AccountGroup
	level    selectorLevel
	selected *profile.AccountGroup // the account we drilled into
	choice   *profile.SSOProfile
	isNew    bool
	quitting bool
}

func (m selectorModel) Init() tea.Cmd {
	// Start in filtering mode immediately so the user can type to search
	// without pressing "/" first.
	return tea.Sequence(
		func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}} },
	)
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Always select the highlighted item on enter, even while filtering.
			// This eliminates the "double enter" problem where the first enter
			// only applies the filter.
			item, ok := m.list.SelectedItem().(selectorItem)
			if !ok {
				break
			}
			switch item.kind {
			case kindNew:
				m.isNew = true
				m.quitting = true
				return m, tea.Quit
			case kindBack:
				m.level = levelAccounts
				m.selected = nil
				m.list.SetItems(m.accountItems())
				m.list.Title = "Select an AWS Account"
				m.list.ResetFilter()
				// Re-enter filtering mode
				return m, func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}} }
			case kindAccount:
				// If only one role, auto-select it
				if len(item.account.Roles) == 1 {
					p := item.account.Roles[0]
					m.choice = &p
					m.quitting = true
					return m, tea.Quit
				}
				// Drill into roles
				m.level = levelRoles
				m.selected = item.account
				m.list.SetItems(m.roleItems(item.account))
				accountLabel := item.account.AccountID
				if item.account.AccountName != "" {
					accountLabel = item.account.AccountName
				}
				m.list.Title = fmt.Sprintf("Select a Role — %s", accountLabel)
				m.list.ResetFilter()
				m.list.Select(0)
				return m, nil
			case kindRole:
				p := *item.profile
				m.choice = &p
				m.quitting = true
				return m, tea.Quit
			}
		case "esc":
			// If filtering is active, let the list handle esc to clear filter first
			if m.list.FilterState() == list.Filtering {
				break
			}
			// If in roles view, go back to accounts instead of quitting
			if m.level == levelRoles {
				m.level = levelAccounts
				m.selected = nil
				m.list.SetItems(m.accountItems())
				m.list.Title = "Select an AWS Account"
				m.list.ResetFilter()
				return m, func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}} }
			}
			m.quitting = true
			return m, tea.Quit
		case "q":
			// Only quit if not filtering (otherwise typing "q" in search)
			if m.list.FilterState() != list.Filtering {
				m.quitting = true
				return m, tea.Quit
			}
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 4)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectorModel) View() string {
	if m.quitting {
		return ""
	}
	return "\n" + m.list.View()
}

func (m selectorModel) accountItems() []list.Item {
	items := make([]list.Item, 0, len(m.groups)+1)
	for i := range m.groups {
		items = append(items, selectorItem{kind: kindAccount, account: &m.groups[i]})
	}
	items = append(items, selectorItem{kind: kindNew})
	return items
}

func (m selectorModel) roleItems(g *profile.AccountGroup) []list.Item {
	items := make([]list.Item, 0, len(g.Roles)+1)
	items = append(items, selectorItem{kind: kindBack})
	for i := range g.Roles {
		items = append(items, selectorItem{kind: kindRole, profile: &g.Roles[i]})
	}
	return items
}

// SelectionResult holds the result of the profile selection.
type SelectionResult struct {
	Profile *profile.SSOProfile // non-nil if an existing profile was selected
	IsNew   bool                // true if user wants to create a new profile
}

// RunProfileSelector displays a fuzzy-searchable list of profiles,
// grouped by AWS account. Selecting an account expands to show its roles.
func RunProfileSelector(profiles []profile.SSOProfile) (*SelectionResult, error) {
	groups := profile.GroupByAccount(profiles)

	delegate := selectorDelegate{}
	m := selectorModel{groups: groups, level: levelAccounts}

	items := m.accountItems()
	l := list.New(items, delegate, 60, 14)
	l.Title = "Select an AWS Account"
	l.Filter = substringFilter
	l.Styles.Title = TitleStyle
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(ColorPrimary)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(ColorPrimary)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.SetShowStatusBar(true)
	l.FilterInput.Placeholder = "Type to filter..."

	m.list = l

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(Output))
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("selector failed: %w", err)
	}

	result := finalModel.(selectorModel)
	if result.choice == nil && !result.isNew {
		return nil, fmt.Errorf("no profile selected")
	}

	return &SelectionResult{
		Profile: result.choice,
		IsNew:   result.isNew,
	}, nil
}

// Confirm displays a yes/no confirmation prompt.
func Confirm(message string) (bool, error) {
	var result bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(message).
				Affirmative("Yes").
				Negative("No").
				Value(&result),
		),
	).WithTheme(sawsTheme()).WithOutput(Output)

	if err := form.Run(); err != nil {
		return false, err
	}
	return result, nil
}

// --- Multi-select import selector ---

// RunProfileImportSelector displays a multi-select list showing all discovered
// account/role combinations. All are pre-selected by default. The user can
// toggle items with space, select/deselect all with a/n, and confirm with enter.
// Uses the same bubbles/list fuzzy-filtering UI as the profile selector.
func RunProfileImportSelector(discovered []DiscoveredProfile) ([]DiscoveredProfile, error) {
	if len(discovered) == 0 {
		return nil, fmt.Errorf("no profiles to import")
	}

	// Build items and pre-select all
	checked := make(map[int]bool, len(discovered))
	items := make([]list.Item, len(discovered))
	for i, d := range discovered {
		checked[i] = true
		accountLabel := d.Profile.AccountName
		if accountLabel == "" {
			accountLabel = d.Profile.AccountID
		}
		items[i] = importItem{
			index:       i,
			accountName: accountLabel,
			roleName:    d.Profile.RoleName,
			profileName: d.Name,
			accountID:   d.Profile.AccountID,
		}
	}

	delegate := importDelegate{checked: checked}
	l := list.New(items, delegate, 60, min(len(discovered)*2+6, 20))
	l.Title = "Select profiles to import  (space: toggle, a: all, n: none)"
	l.Filter = substringFilter
	l.Styles.Title = TitleStyle
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(ColorPrimary)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(ColorPrimary)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.SetShowStatusBar(true)
	l.FilterInput.Placeholder = "Type to filter..."

	m := importModel{
		list:       l,
		checked:    checked,
		discovered: discovered,
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(Output))
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("import selector failed: %w", err)
	}

	result := finalModel.(importModel)
	if result.cancelled {
		return nil, fmt.Errorf("import selection cancelled")
	}

	// Collect selected profiles
	var selected []DiscoveredProfile
	for i, d := range discovered {
		if result.checked[i] {
			selected = append(selected, d)
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no profiles selected")
	}

	return selected, nil
}

// importItem implements list.Item for the import multi-selector.
type importItem struct {
	index       int
	accountName string
	roleName    string
	profileName string
	accountID   string
}

func (i importItem) FilterValue() string {
	return i.accountName + " " + i.roleName + " " + i.profileName + " " + i.accountID
}

// importDelegate renders each item with a checkbox.
type importDelegate struct {
	checked map[int]bool
}

func (d importDelegate) Height() int                             { return 2 }
func (d importDelegate) Spacing() int                            { return 0 }
func (d importDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d importDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(importItem)
	if !ok {
		return
	}

	isCursor := index == m.Index()

	checkbox := "[ ]"
	if d.checked[item.index] {
		checkbox = "[x]"
	}

	title := fmt.Sprintf("%s %s / %s", checkbox, item.accountName, item.roleName)
	desc := item.profileName

	titleStyle := lipgloss.NewStyle().PaddingLeft(2)
	descStyle := lipgloss.NewStyle().PaddingLeft(2).Foreground(ColorMuted)

	if isCursor {
		titleStyle = titleStyle.Foreground(ColorPrimary).Bold(true)
		descStyle = descStyle.Foreground(ColorPrimary)
		title = "> " + title
	} else {
		titleStyle = titleStyle.Foreground(ColorWhite)
		title = "  " + title
	}

	fmt.Fprintf(w, "%s\n%s", titleStyle.Render(title), descStyle.Render("    "+desc))
}

// importModel is the bubbletea model for multi-select import.
type importModel struct {
	list       list.Model
	checked    map[int]bool
	discovered []DiscoveredProfile
	confirmed  bool
	cancelled  bool
}

func (m importModel) Init() tea.Cmd {
	// Start in filtering mode immediately.
	return tea.Sequence(
		func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}} },
	)
}

func (m importModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case " ":
			// Toggle current item (only when not actively filtering to avoid
			// interfering with search input — but space in filter is unusual)
			if m.list.FilterState() != list.Filtering {
				item, ok := m.list.SelectedItem().(importItem)
				if ok {
					m.checked[item.index] = !m.checked[item.index]
					m.list.SetDelegate(importDelegate{checked: m.checked})
				}
				return m, nil
			}
		case "a":
			if m.list.FilterState() != list.Filtering {
				for i := range m.discovered {
					m.checked[i] = true
				}
				m.list.SetDelegate(importDelegate{checked: m.checked})
				return m, nil
			}
		case "n":
			if m.list.FilterState() != list.Filtering {
				for i := range m.discovered {
					m.checked[i] = false
				}
				m.list.SetDelegate(importDelegate{checked: m.checked})
				return m, nil
			}
		case "enter":
			// Always confirm on enter, even while filtering
			m.confirmed = true
			return m, tea.Quit
		case "q":
			if m.list.FilterState() != list.Filtering {
				m.cancelled = true
				return m, tea.Quit
			}
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "esc":
			// If filtering, let list handle esc to clear filter
			if m.list.FilterState() == list.Filtering {
				break
			}
			m.cancelled = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 4)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m importModel) View() string {
	if m.confirmed || m.cancelled {
		return ""
	}
	// Show count of selected items
	count := 0
	for _, v := range m.checked {
		if v {
			count++
		}
	}
	status := lipgloss.NewStyle().Foreground(ColorMuted).PaddingLeft(2).
		Render(fmt.Sprintf("%d of %d selected — enter to confirm", count, len(m.discovered)))
	return "\n" + m.list.View() + "\n" + status
}
