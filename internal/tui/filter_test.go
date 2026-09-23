package tui

import (
	"strings"
	"testing"

	"github.com/netikras/procfit/internal/queryspec"
)

func hasStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func TestBareSearch_WordVsQuery(t *testing.T) {
	if got := bareSearch("idea"); got != `target ~= "(?i)idea"` {
		t.Fatalf("lone word → case-insensitive substring, got %q", got)
	}
	// A metacharacter stays literal (escaped).
	if got := bareSearch("a.b"); got != `target ~= "(?i)a\.b"` {
		t.Fatalf("metachar must be escaped, got %q", got)
	}
	// A space means "you're writing a query" → passed through unchanged.
	if got := bareSearch("cpu > 5"); got != "cpu > 5" {
		t.Fatalf("query with a space must pass through, got %q", got)
	}
}

func TestFilterAutocomplete(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3) // filterable fields: target, cpu
	m.SetResult(res, cols)
	m.SetFilterOps(func(f string) []string {
		if f == "cpu" {
			return []string{"==", "!=", ">", "<", ">=", "<=", "in"}
		}
		return []string{"==", "!=", "~=", "contains", "in"}
	})
	set := func(buf string) { m.editBuf, m.editPos = buf, len([]rune(buf)) }

	// First word: substring position — column hints, but Enter APPLIES (no autofill).
	set("cp")
	fs := m.filterState()
	if fs.ctx != ctxSubstring || !hasStr(fs.suggest, "cpu") {
		t.Fatalf("first-word ctx/suggest wrong: %+v", fs)
	}
	if m.filterAutofill() {
		t.Fatal("Enter must not autofill in the substring position")
	}
	// ...but Tab completes the column even there.
	set("cp")
	m.filterTab()
	if m.editBuf != "cpu " {
		t.Fatalf("Tab should complete the column, got %q", m.editBuf)
	}

	// Operator position: numeric ops for cpu; Enter AUTOFILLS the first.
	set("cpu ")
	if fs = m.filterState(); fs.ctx != ctxOperator || fs.suggest[0] != "==" {
		t.Fatalf("operator ctx/suggest wrong: %+v", fs)
	}
	if !m.filterAutofill() || m.editBuf != "cpu == " {
		t.Fatalf("Enter should autofill the operator, got %q", m.editBuf)
	}

	// Operator prefix filters (> → >, >=; not ==).
	set("cpu >")
	if fs = m.filterState(); !hasStr(fs.suggest, ">=") || hasStr(fs.suggest, "==") {
		t.Fatalf("operator prefix filter wrong: %v", fs.suggest)
	}

	// Value position: no suggestions, Enter APPLIES.
	set("cpu > 5")
	if fs = m.filterState(); fs.ctx != ctxValue || len(fs.suggest) != 0 {
		t.Fatalf("value ctx wrong: %+v", fs)
	}
	if m.filterAutofill() {
		t.Fatal("value position must not autofill")
	}

	// After && : column context (no substring shortcut), Enter AUTOFILLS.
	set("cpu > 5 && ta")
	if fs = m.filterState(); fs.ctx != ctxColumn || !hasStr(fs.suggest, "target") {
		t.Fatalf("post-&& ctx/suggest wrong: %+v", fs)
	}
	if !m.filterAutofill() || m.editBuf != "cpu > 5 && target " {
		t.Fatalf("column after && should autofill, got %q", m.editBuf)
	}
}

func TestSuggestHint_MarksAutofillTarget(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3)
	m.SetResult(res, cols)
	// Operator context: the Enter target is bracketed, labelled "ops".
	m.editBuf, m.editPos = "cpu ", len("cpu ")
	hint := m.suggestHint()
	if !strings.HasPrefix(hint, "ops: [") {
		t.Fatalf("operator hint should bracket the autofill target: %q", hint)
	}
	// Substring context: columns are hints only (no bracket, labelled "cols").
	m.editBuf, m.editPos = "cp", 2
	if hint := m.suggestHint(); !strings.HasPrefix(hint, "cols: ") || strings.Contains(hint, "[") {
		t.Fatalf("substring hint must not bracket (Enter applies): %q", hint)
	}
}
