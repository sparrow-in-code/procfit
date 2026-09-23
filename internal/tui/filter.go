package tui

import "strings"

// filterCtx is the autocomplete context at the cursor while editing a filter.
type filterCtx int

const (
	// ctxSubstring is the first token of the whole buffer: a bare word here is a
	// case-insensitive target substring, so column names are only *hints* and
	// Enter APPLIES (does not autofill).
	ctxSubstring filterCtx = iota
	// ctxColumn is the first token of a clause after && / ||: it must be a column,
	// so Enter AUTOFILLS the first suggestion.
	ctxColumn
	// ctxOperator is the second token of a clause: operator suggestions, Enter
	// AUTOFILLS.
	ctxOperator
	// ctxValue is the third+ token: a literal value, no suggestions, Enter APPLIES.
	ctxValue
)

// filterState describes the autocomplete situation at the cursor.
type filterState struct {
	ctx        filterCtx
	tokenStart int      // rune index in editBuf where the current partial token begins
	token      string   // the partial token being typed
	suggest    []string // prefix-filtered suggestions (columns or operators)
}

// autofills reports whether Enter should autofill the first suggestion (rather
// than apply the filter) in this context.
func (s filterState) autofills() bool {
	return (s.ctx == ctxColumn || s.ctx == ctxOperator) && len(s.suggest) > 0
}

// filterState computes the autocomplete context from the buffer up to the cursor.
func (m *Model) filterState() filterState {
	runes := []rune(m.editBuf)
	pos := clampInt(m.editPos, 0, len(runes))
	head := string(runes[:pos])

	clauseStart, firstClause := lastClauseStart(head)
	clause := head[clauseStart:]

	lastSpace := strings.LastIndexByte(clause, ' ')
	tokenByteStart := clauseStart + lastSpace + 1
	token := head[tokenByteStart:]
	fs := filterState{tokenStart: len([]rune(head[:tokenByteStart])), token: token}

	prior := strings.Fields(clause[:lastSpace+1]) // completed tokens before the current one
	switch len(prior) {
	case 0:
		fs.ctx = ctxSubstring
		if !firstClause {
			fs.ctx = ctxColumn
		}
		fs.suggest = prefixFilter(m.filterableList(), token)
	case 1:
		fs.ctx = ctxOperator
		fs.suggest = prefixFilter(m.opsFor(prior[0]), token)
	default:
		fs.ctx = ctxValue
	}
	return fs
}

// lastClauseStart returns the byte index just after the last && / || in head (0 if
// none), and whether head is still the first clause (no boolean chain yet).
func lastClauseStart(head string) (int, bool) {
	i := strings.LastIndex(head, "&&")
	if j := strings.LastIndex(head, "||"); j > i {
		i = j
	}
	if i < 0 {
		return 0, true
	}
	return i + 2, false
}

// opsFor returns the operators offered for a field (numeric vs string), via the
// injected classifier; a generous default when none is installed.
func (m *Model) opsFor(field string) []string {
	if m.filterOpsFn != nil {
		return m.filterOpsFn(field)
	}
	return []string{"==", "!=", "~=", "contains", "<", ">", "<=", ">=", "in"}
}

// prefixFilter keeps entries whose lowercase form starts with token (all when the
// token is empty).
func prefixFilter(list []string, token string) []string {
	if token == "" {
		return list
	}
	lt := strings.ToLower(token)
	out := make([]string, 0, len(list))
	for _, s := range list {
		if strings.HasPrefix(strings.ToLower(s), lt) {
			out = append(out, s)
		}
	}
	return out
}

// filterAutofill replaces the current partial token with the first suggestion
// when Enter lands in an autofilling context (column after &&/||, or operator);
// returns true if it did (so Enter does not also apply).
func (m *Model) filterAutofill() bool {
	fs := m.filterState()
	if !fs.autofills() {
		return false
	}
	m.applySuggestion(fs)
	return true
}

// filterTab autofills the first suggestion in ANY suggesting context (including
// the first-word substring position, where Enter applies but Tab completes a
// column name); a no-op when there is nothing to suggest.
func (m *Model) filterTab() {
	fs := m.filterState()
	if len(fs.suggest) == 0 {
		return
	}
	m.applySuggestion(fs)
}

// applySuggestion replaces the current partial token with the first suggestion
// plus a trailing space, moving the cursor after it.
func (m *Model) applySuggestion(fs filterState) {
	runes := []rune(m.editBuf)
	fill := []rune(fs.suggest[0] + " ")
	next := append([]rune{}, runes[:fs.tokenStart]...)
	next = append(next, fill...)
	next = append(next, runes[clampInt(m.editPos, 0, len(runes)):]...)
	m.editBuf = string(next)
	m.editPos = fs.tokenStart + len(fill)
}

// suggestHint renders the inline autocomplete hint shown in the filter status
// line: the field/operator candidates, with the Enter-autofill target bracketed.
func (m *Model) suggestHint() string {
	fs := m.filterState()
	if len(fs.suggest) == 0 {
		return ""
	}
	const maxShown = 8
	shown := append([]string{}, fs.suggest...)
	if len(shown) > maxShown {
		shown = shown[:maxShown]
	}
	if fs.autofills() {
		shown[0] = "[" + shown[0] + "]" // Enter autofills this one
	}
	label := "cols"
	if fs.ctx == ctxOperator {
		label = "ops"
	}
	return label + ": " + strings.Join(shown, " ")
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
