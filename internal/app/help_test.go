package app

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func flagSetForTest() *flag.FlagSet { return flag.NewFlagSet("t", flag.ContinueOnError) }

func TestTopLevelHelpListsCommonFlags(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"help"}); code != ExitOK {
		t.Fatalf("help exit %d", code)
	}
	for _, want := range []string{"Commands:", "Common view flags", "--human", "--target-width", "--number"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("top-level help missing %q:\n%s", want, out.String())
		}
	}
}

func TestPS_HelpListsFlags(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"ps", "--help"}); code != ExitOK {
		t.Fatalf("ps --help exit %d, want 0", code)
	}
	for _, want := range []string{"Usage: procfit ps", "-human", "-target-width", "-group-by"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("ps --help missing %q:\n%s", want, out.String())
		}
	}
}

func TestHelpForCommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"help", "stat"}); code != ExitOK {
		t.Fatalf("help stat exit %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: procfit stat") {
		t.Fatalf("help stat did not print stat usage:\n%s", out.String())
	}
}

func TestHumanToggleParses(t *testing.T) {
	// -h and --human both set the human flag (free-style units toggle).
	for _, arg := range []string{"-h", "--human"} {
		fs := flagSetForTest()
		qf := bindQueryFlags(fs)
		if err := fs.Parse([]string{arg}); err != nil {
			t.Fatalf("parse %q: %v", arg, err)
		}
		if !qf.toSpecFlags().Human {
			t.Fatalf("%q should enable human mode", arg)
		}
	}
	// Default (no flag) is raw.
	fs := flagSetForTest()
	qf := bindQueryFlags(fs)
	_ = fs.Parse(nil)
	if qf.toSpecFlags().Human {
		t.Fatal("human must be off by default")
	}
}
