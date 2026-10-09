package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	faint  = "\033[2m"
	green  = "\033[32m"
	red    = "\033[31m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	blue   = "\033[34m"
)

//

var (
	Out io.Writer = os.Stdout
	Err io.Writer = os.Stderr
)

var forcedOff bool

func DisableColor() { forcedOff = true }

func colorEnabled() bool {
	if forcedOff || os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := Out.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

func wrap(code, s string) string {
	if !colorEnabled() {
		return s
	}
	return code + s + reset
}

// Check prints one line of a report with a severity marker: ✓ when the
// check passed, ! when it found something.
//
// A report goes to stdout even when it contains problems, so redirecting
// one still gives a readable document. The verdict about the run — what
// to do, and whether it passed — is a diagnostic, and stays on stderr.
func Check(ok bool, format string, a ...interface{}) {
	if ok {
		fmt.Fprint(Out, wrap(green+bold, "✓ "))
	} else {
		fmt.Fprint(Out, wrap(yellow+bold, "! "))
	}
	fmt.Fprintf(Out, format+"\n", a...)
}

func Success(format string, a ...interface{}) {
	fmt.Fprint(Out, wrap(green+bold, "✓ "))
	fmt.Fprintf(Out, format+"\n", a...)
}

func Error(format string, a ...interface{}) {
	fmt.Fprint(Err, wrap(red+bold, "✗ "))
	fmt.Fprintf(Err, format+"\n", a...)
}

func Warn(format string, a ...interface{}) {
	fmt.Fprint(Err, wrap(yellow+bold, "! "))
	fmt.Fprintf(Err, format+"\n", a...)
}

func Info(format string, a ...interface{}) {
	fmt.Fprint(Out, wrap(cyan, "→ "))
	fmt.Fprintf(Out, format+"\n", a...)
}

func Dim(format string, a ...interface{}) {
	fmt.Fprintln(Out, wrap(faint, fmt.Sprintf(format, a...)))
}

func Step(format string, a ...interface{}) {
	fmt.Fprint(Out, wrap(blue, "  • "))
	fmt.Fprintf(Out, format+"\n", a...)
}

func Bold(s string) string { return wrap(bold, s) }

func Field(label string, format string, a ...interface{}) {
	fmt.Fprintf(Out, "  %s  %s\n", wrap(faint, fmt.Sprintf("%-10s", label)), fmt.Sprintf(format, a...))
}

// Table prints aligned columns with an optional header. Widths come from
// the content itself, so the layout is deterministic and doesn't depend
// on the terminal. Useful where rows have more than one attribute to
// compare, like branch state across every branch.
func Table(header []string, rows [][]string) {
	if len(header) == 0 && len(rows) == 0 {
		return
	}

	cols := 0
	if len(header) > 0 {
		cols = len(header)
	}
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	if cols == 0 {
		return
	}

	widths := make([]int, cols)
	measure := func(cells []string) {
		for i, cell := range cells {
			if n := len([]rune(cell)); n > widths[i] {
				widths[i] = n
			}
		}
	}
	if len(header) > 0 {
		measure(header)
	}
	for _, r := range rows {
		measure(r)
	}

	if len(header) > 0 {
		fmt.Fprintln(Out, wrap(faint, strings.TrimRight(padRow(header, widths), " ")))
	}
	for _, r := range rows {
		fmt.Fprintln(Out, padRow(r, widths))
	}
}

func padRow(cells []string, widths []int) string {
	parts := make([]string, len(widths))
	for i := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		parts[i] = cell + strings.Repeat(" ", widths[i]-len([]rune(cell)))
	}
	return strings.TrimRight(strings.Join(parts, "  "), " ")
}

type TreeNode struct {
	Label    string
	Current  bool
	Missing  bool
	Note     string
	Children []TreeNode
}

// Tree renders nodes as an indented tree, git-flow style, e.g.:
//
//	main
//	└── develop
//	     ├── feature/api-auth  (current)
//	     └── feature/ui-polish

func Tree(nodes []TreeNode) {
	for i, n := range nodes {
		printTreeNode(n, "", i == len(nodes)-1, true)
	}
}

func printTreeNode(n TreeNode, prefix string, last, root bool) {
	connector := "├── "
	nextPrefix := prefix + "│    "
	if last {
		connector = "└── "
		nextPrefix = prefix + "     "
	}
	if root {
		connector = ""
	}

	label := n.Label
	switch {
	case n.Missing:
		label = wrap(red+bold, label) + wrap(faint, "  (missing)")
	case n.Current:
		label = wrap(green+bold, label) + wrap(faint, "  (current)")
	}
	if n.Note != "" {
		label += wrap(faint, "  "+n.Note)
	}
	fmt.Fprintf(Out, "%s%s%s\n", prefix, connector, label)

	for i, c := range n.Children {
		printTreeNode(c, nextPrefix, i == len(n.Children)-1, false)
	}
}
