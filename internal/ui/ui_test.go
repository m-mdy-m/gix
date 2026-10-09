package ui

import (
	"bytes"
	"strings"
	"testing"
)

func capture(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	oldOut, oldErr := Out, Err
	Out, Err = &buf, &buf
	t.Cleanup(func() { Out, Err = oldOut, oldErr })

	fn()

	return buf.String()
}

func TestColorIsOffWhenNotWritingToAFile(t *testing.T) {
	out := capture(t, func() {
		Success("done")
		Error("bad")
		Warn("careful")
		Info("next")
	})

	for _, marker := range []string{"\033["} {
		if strings.Contains(out, marker) {
			t.Errorf("output contains an escape sequence: %q", out)
		}
	}
	for _, want := range []string{"✓ done", "✗ bad", "! careful", "→ next"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

func TestMessagesSplitAcrossOutAndErr(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	oldOut, oldErr := Out, Err
	Out, Err = &outBuf, &errBuf
	t.Cleanup(func() { Out, Err = oldOut, oldErr })

	Success("kept")
	Info("kept")
	Dim("kept")
	Step("kept")
	Field("label", "kept")
	Error("dropped")
	Warn("dropped")

	if !strings.Contains(outBuf.String(), "kept") {
		t.Errorf("Out = %q, want it to carry normal output", outBuf.String())
	}
	if strings.Contains(outBuf.String(), "dropped") {
		t.Errorf("Out = %q, want problems kept off stdout", outBuf.String())
	}
	if !strings.Contains(errBuf.String(), "dropped") {
		t.Errorf("Err = %q, want problems written to stderr", errBuf.String())
	}
}

func TestFieldLabelsLineUp(t *testing.T) {
	out := capture(t, func() {
		Field("main", "%s", "main")
		Field("develop", "%s", "develop")
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("output has %d lines, want 2:\n%s", len(lines), out)
	}
	if strings.Index(lines[0], "main") != strings.Index(lines[1], "develop") {
		t.Errorf("values are not aligned:\n%s", out)
	}
}

func TestTableAlignsColumns(t *testing.T) {
	out := capture(t, func() {
		Table(nil, [][]string{
			{"BRANCH", "KIND", "STATE"},
			{"feature/api-auth", "topic", "behind"},
			{"develop", "base", "tracked"},
		})
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%s", len(lines), out)
	}
	for _, line := range lines {
		if strings.HasSuffix(line, " ") {
			t.Errorf("line %q has trailing spaces", line)
		}
	}
	// The second column starts at the same offset on every row, sized by
	// the longest first-column value.
	first := strings.Index(lines[1], "topic")
	if first <= 0 {
		t.Fatalf("no second column in %q", lines[1])
	}
	if got := strings.Index(lines[2], "base"); got != first {
		t.Errorf("second column starts at %d on line 2 but %d on line 3:\n%s", first, got, out)
	}
	if got := strings.Index(lines[0], "KIND"); got != first {
		t.Errorf("header column starts at %d, want %d to match the rows:\n%s", got, first, out)
	}
}

func TestTableWithHeader(t *testing.T) {
	out := capture(t, func() {
		Table([]string{"BRANCH", "STATE"}, [][]string{
			{"main", "tracked"},
		})
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want a header and one row:\n%s", len(lines), out)
	}
	if !strings.Contains(lines[0], "BRANCH") || !strings.Contains(lines[0], "STATE") {
		t.Errorf("header = %q, want the column names", lines[0])
	}
	if strings.HasSuffix(lines[0], " ") {
		t.Errorf("header %q has trailing spaces", lines[0])
	}
}

func TestTableRaggedRows(t *testing.T) {
	out := capture(t, func() {
		Table([]string{"A", "B", "C"}, [][]string{
			{"only one"},
			{"a", "b", "c", "extra"},
		})
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%s", len(lines), out)
	}
	if !strings.Contains(lines[1], "only one") {
		t.Errorf("short row = %q, want its content kept", lines[1])
	}
	if !strings.Contains(lines[2], "extra") {
		t.Errorf("long row = %q, want its content kept", lines[2])
	}
}

func TestTableEmpty(t *testing.T) {
	if out := capture(t, func() { Table(nil, nil) }); out != "" {
		t.Errorf("Table(nil, nil) wrote %q, want nothing", out)
	}
}

// A row with fewer cells than the header still lines up, and the missing
// cell is left empty rather than invented.
func TestTableShortRowLinesUp(t *testing.T) {
	out := capture(t, func() {
		Table([]string{"BRANCH", "KIND", "STATE"}, [][]string{
			{"develop", "base"},
		})
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want a header and one row:\n%s", len(lines), out)
	}
	if lines[1] != "develop  base" {
		t.Errorf("row = %q, want %q with no trailing padding", lines[1], "develop  base")
	}
}

// A report line belongs on stdout: if it went to stderr, redirecting a
// report would leave the findings without their headings.
func TestCheckMarksOnStdout(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	oldOut, oldErr := Out, Err
	Out, Err = &outBuf, &errBuf
	t.Cleanup(func() { Out, Err = oldOut, oldErr })

	Check(true, "clean")
	Check(false, "needs work")

	if !strings.Contains(outBuf.String(), "✓ clean") {
		t.Errorf("a passing check should be marked on stdout: %q", outBuf.String())
	}
	if !strings.Contains(outBuf.String(), "! needs work") {
		t.Errorf("a failing check should be on stdout too, or a redirected report loses its headings: %q", outBuf.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("a report line belongs on stdout, got %q on stderr", errBuf.String())
	}
}

func TestTree(t *testing.T) {
	out := capture(t, func() {
		Tree([]TreeNode{{
			Label: "main",
			Children: []TreeNode{{
				Label: "develop",
				Children: []TreeNode{
					{Label: "feature/api-auth", Current: true},
					{Label: "feature/ui-polish"},
				},
			}},
		}})
	})

	want := []string{
		"main",
		"└── develop",
		"├── feature/api-auth",
		"(current)",
		"└── feature/ui-polish",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("tree output is missing %q:\n%s", w, out)
		}
	}
}

func TestTreeMarksMissingAndNoted(t *testing.T) {
	out := capture(t, func() {
		Tree([]TreeNode{
			{Label: "main"},
			{Label: "develop", Missing: true, Note: "(was integration)"},
		})
	})

	for _, want := range []string{"(missing)", "(was integration)"} {
		if !strings.Contains(out, want) {
			t.Errorf("tree output is missing %q:\n%s", want, out)
		}
	}
}

func TestTreeSingleRootHasNoConnector(t *testing.T) {
	out := capture(t, func() {
		Tree([]TreeNode{{Label: "main"}})
	})

	if strings.TrimSpace(out) != "main" {
		t.Errorf("single root rendered as %q, want just the label", out)
	}
}

func TestTreeEmpty(t *testing.T) {
	if out := capture(t, func() { Tree(nil) }); out != "" {
		t.Errorf("Tree(nil) wrote %q, want nothing", out)
	}
}

func TestDisableColorStripsSequences(t *testing.T) {
	DisableColor()
	t.Cleanup(func() { forcedOff = false })

	out := capture(t, func() {
		Info(Bold("main"))
	})

	if strings.Contains(out, "\033[") {
		t.Errorf("DisableColor did not strip escape sequences: %q", out)
	}
	if strings.TrimSpace(out) != "→ main" {
		t.Errorf("output = %q, want the text kept with colors stripped", out)
	}
}

func TestNoColorEnvIsHonored(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	out := capture(t, func() {
		if colorEnabled() {
			t.Error("colorEnabled() = true with NO_COLOR set")
		}
		Success("done")
	})

	if strings.Contains(out, "\033[") {
		t.Errorf("output contains escape sequences with NO_COLOR set: %q", out)
	}
}
