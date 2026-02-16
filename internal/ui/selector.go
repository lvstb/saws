package ui

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/lvstb/saws/internal/profile"
)

// matchesFilter returns true if the item's FilterValue contains the term
// (case-insensitive substring match).
func matchesFilter(item list.Item, term string) bool {
	if term == "" {
		return true
	}
	return strings.Contains(strings.ToLower(item.FilterValue()), strings.ToLower(term))
}

// filterItems returns only items matching the filter term.
func filterItems(all []list.Item, term string) []list.Item {
	if term == "" {
		return all
	}
	var out []list.Item
	for _, item := range all {
		if matchesFilter(item, term) {
			out = append(out, item)
		}
	}
	return out
}

// isFilterRune returns true for printable characters that should go to the filter.
func isFilterRune(msg tea.KeyMsg) (rune, bool) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		r := msg.Runes[0]
		if unicode.IsPrint(r) {
			return r, true
		}
	}
	return 0, false
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

// selectorItem implements list.Item for the profile selector.
type selectorItem struct {
	kind    itemKind
	account *profile.AccountGroup // set for kindAccount
	profile *profile.SSOProfile   // set for kindRole
}

func (i selectorItem) FilterValue() string {
	switch i.kind {
	case kindAccount:
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
// It manages its own filter text so that typing filters the list while
// arrow keys simultaneously navigate the filtered results.
type selectorModel struct {
	list       list.Model
	groups     []profile.AccountGroup
	allItems   []list.Item // unfiltered items for current level
	filterText string
	level      selectorLevel
	selected   *profile.AccountGroup // the account we drilled into
	choice     *profile.SSOProfile
	isNew      bool
	quitting   bool
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

// applyFilter updates the list items based on the current filter text.
func (m *selectorModel) applyFilter() {
	filtered := filterItems(m.allItems, m.filterText)
	m.list.SetItems(filtered)
	m.list.Select(0)
}

// setLevel switches to a new level with the given items and title, clearing the filter.
func (m *selectorModel) setLevel(level selectorLevel, items []list.Item, title string) {
	m.level = level
	m.allItems = items
	m.filterText = ""
	m.list.SetItems(items)
	m.list.Title = title
	m.list.Select(0)
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle filter input: printable runes
		if r, ok := isFilterRune(msg); ok {
			// 'q' quits when filter is empty
			if r == 'q' && m.filterText == "" {
				m.quitting = true
				return m, tea.Quit
			}
			m.filterText += string(r)
			m.applyFilter()
			return m, nil
		}

		switch msg.Type {
		case tea.KeyBackspace:
			if len(m.filterText) > 0 {
				m.filterText = m.filterText[:len(m.filterText)-1]
				m.applyFilter()
				return m, nil
			}
		case tea.KeyEnter:
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
				m.selected = nil
				m.setLevel(levelAccounts, m.accountItems(), "Select an AWS Account")
				return m, nil
			case kindAccount:
				if len(item.account.Roles) == 1 {
					p := item.account.Roles[0]
					m.choice = &p
					m.quitting = true
					return m, tea.Quit
				}
				m.selected = item.account
				accountLabel := item.account.AccountID
				if item.account.AccountName != "" {
					accountLabel = item.account.AccountName
				}
				m.setLevel(levelRoles, m.roleItems(item.account), fmt.Sprintf("Select a Role — %s", accountLabel))
				return m, nil
			case kindRole:
				p := *item.profile
				m.choice = &p
				m.quitting = true
				return m, tea.Quit
			}
		case tea.KeyEscape:
			// If there's filter text, clear it first
			if m.filterText != "" {
				m.filterText = ""
				m.applyFilter()
				return m, nil
			}
			// If in roles view, go back to accounts
			if m.level == levelRoles {
				m.selected = nil
				m.setLevel(levelAccounts, m.accountItems(), "Select an AWS Account")
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 4)
	}

	// Pass through to list for arrow key navigation, page up/down, etc.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectorModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")

	// Render filter input line
	prompt := lipgloss.NewStyle().Foreground(ColorPrimary).Render("> ")
	cursor := lipgloss.NewStyle().Foreground(ColorPrimary).Render("█")
	filterStyle := lipgloss.NewStyle().Foreground(ColorWhite)

	if m.filterText != "" {
		b.WriteString("  " + prompt + filterStyle.Render(m.filterText) + cursor + "\n\n")
	} else {
		placeholder := lipgloss.NewStyle().Foreground(ColorMuted).Render("Type to filter...")
		b.WriteString("  " + prompt + placeholder + "\n\n")
	}

	b.WriteString(m.list.View())

	// Help line at bottom
	help := lipgloss.NewStyle().Foreground(ColorMuted).PaddingLeft(2).
		Render("enter: select  esc: back  q: quit")
	b.WriteString("\n" + help)

	return b.String()
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

// RunProfileSelector displays a searchable list of profiles,
// grouped by AWS account. Selecting an account expands to show its roles.
// Typing filters the list; arrow keys navigate simultaneously.
func RunProfileSelector(profiles []profile.SSOProfile) (*SelectionResult, error) {
	groups := profile.GroupByAccount(profiles)

	delegate := selectorDelegate{}
	items := make([]list.Item, 0, len(groups)+1)
	for i := range groups {
		items = append(items, selectorItem{kind: kindAccount, account: &groups[i]})
	}
	items = append(items, selectorItem{kind: kindNew})

	l := list.New(items, delegate, 60, 14)
	l.Title = "Select an AWS Account"
	l.Styles.Title = TitleStyle
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)

	m := selectorModel{
		list:     l,
		groups:   groups,
		allItems: items,
		level:    levelAccounts,
	}

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
// Typing filters the list; arrow keys navigate simultaneously.
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
	l.Title = "Select profiles to import"
	l.Styles.Title = TitleStyle
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)

	m := importModel{
		list:       l,
		allItems:   items,
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
// Like selectorModel, it manages its own filter so typing and navigation
// work simultaneously.
type importModel struct {
	list       list.Model
	allItems   []list.Item // unfiltered items
	filterText string
	checked    map[int]bool
	discovered []DiscoveredProfile
	confirmed  bool
	cancelled  bool
}

func (m importModel) Init() tea.Cmd {
	return nil
}

// applyFilter updates the list items based on the current filter text.
func (m *importModel) applyFilter() {
	filtered := filterItems(m.allItems, m.filterText)
	m.list.SetItems(filtered)
	m.list.Select(0)
}

func (m importModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle filter input: printable runes
		if r, ok := isFilterRune(msg); ok {
			switch r {
			case 'a':
				if m.filterText == "" {
					for i := range m.discovered {
						m.checked[i] = true
					}
					m.list.SetDelegate(importDelegate{checked: m.checked})
					return m, nil
				}
			case 'n':
				if m.filterText == "" {
					for i := range m.discovered {
						m.checked[i] = false
					}
					m.list.SetDelegate(importDelegate{checked: m.checked})
					return m, nil
				}
			case 'q':
				if m.filterText == "" {
					m.cancelled = true
					return m, tea.Quit
				}
			}
			m.filterText += string(r)
			m.applyFilter()
			return m, nil
		}

		switch msg.Type {
		case tea.KeySpace:
			// Space toggles checkbox on current item
			item, ok := m.list.SelectedItem().(importItem)
			if ok {
				m.checked[item.index] = !m.checked[item.index]
				m.list.SetDelegate(importDelegate{checked: m.checked})
			}
			return m, nil
		case tea.KeyBackspace:
			if len(m.filterText) > 0 {
				m.filterText = m.filterText[:len(m.filterText)-1]
				m.applyFilter()
				return m, nil
			}
		case tea.KeyEnter:
			m.confirmed = true
			return m, tea.Quit
		case tea.KeyEscape:
			if m.filterText != "" {
				m.filterText = ""
				m.applyFilter()
				return m, nil
			}
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyCtrlC:
			m.cancelled = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 4)
	}

	// Pass through to list for arrow key navigation
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m importModel) View() string {
	if m.confirmed || m.cancelled {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")

	// Render filter input line
	prompt := lipgloss.NewStyle().Foreground(ColorPrimary).Render("> ")
	cursor := lipgloss.NewStyle().Foreground(ColorPrimary).Render("█")
	filterStyle := lipgloss.NewStyle().Foreground(ColorWhite)

	if m.filterText != "" {
		b.WriteString("  " + prompt + filterStyle.Render(m.filterText) + cursor + "\n\n")
	} else {
		placeholder := lipgloss.NewStyle().Foreground(ColorMuted).Render("Type to filter...")
		b.WriteString("  " + prompt + placeholder + "\n\n")
	}

	b.WriteString(m.list.View())

	// Show count of selected items
	count := 0
	for _, v := range m.checked {
		if v {
			count++
		}
	}
	status := lipgloss.NewStyle().Foreground(ColorMuted).PaddingLeft(2).
		Render(fmt.Sprintf("%d of %d selected  •  space: toggle  a: all  n: none  enter: confirm", count, len(m.discovered)))
	b.WriteString("\n" + status)

	return b.String()
}
