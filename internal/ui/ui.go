package ui

import (
	"fmt"
	"io"
	"os"

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
