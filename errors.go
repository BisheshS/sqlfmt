package sqlfmt

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// SyntaxError is a problem found in the input, with its 1-based position.
type SyntaxError struct {
	Pos       int // byte offset in the input
	Line, Col int
	Msg       string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Msg)
}

func newSyntaxError(src string, pos int, msg string) *SyntaxError {
	before := src[:pos]
	line := strings.Count(before, "\n") + 1
	col := utf8.RuneCountInString(before[strings.LastIndexByte(before, '\n')+1:]) + 1
	return &SyntaxError{Pos: pos, Line: line, Col: col, Msg: msg}
}

// checker collects structural problems: things that must be opened and closed.
// Keyword hooks (keyword.Check) and validate use it to report them.
type checker struct {
	open     map[string][]problem // currently open items, per kind
	problems []problem
}

type problem struct {
	pos int
	msg string
}

// opened records an open item of the given kind, such as "(" or "CASE".
// msgIfUnclosed is reported if nothing closes it.
func (c *checker) opened(kind string, pos int, msgIfUnclosed string) {
	if c.open == nil {
		c.open = map[string][]problem{}
	}
	c.open[kind] = append(c.open[kind], problem{pos, msgIfUnclosed})
}

// closed matches the latest open item of the given kind. When there is none,
// it reports msgIfUnmatched.
func (c *checker) closed(kind string, pos int, msgIfUnmatched string) {
	stack := c.open[kind]
	if len(stack) == 0 {
		c.problems = append(c.problems, problem{pos, msgIfUnmatched})
		return
	}
	c.open[kind] = stack[:len(stack)-1]
}

// validate checks the tokens for unbalanced parentheses and CASE / END. It
// returns every problem it finds, in input order, or nil.
func validate(src string, toks []Token) error {
	c := &checker{}
	for i, t := range toks {
		switch {
		case t.Kind == Punct && t.Text == "(":
			c.opened("(", t.Pos, "unclosed (")
		case t.Kind == Punct && t.Text == ")":
			c.closed("(", t.Pos, "unmatched )")
		case t.Kind == Word && !afterDot(toks, i):
			if kw := keywords[strings.ToUpper(t.Text)]; kw.Check != nil {
				kw.Check(c, t)
			}
		}
	}
	for _, stack := range c.open {
		c.problems = append(c.problems, stack...)
	}
	if len(c.problems) == 0 {
		return nil
	}
	sort.Slice(c.problems, func(i, j int) bool { return c.problems[i].pos < c.problems[j].pos })
	errs := make([]error, len(c.problems))
	for i, p := range c.problems {
		errs[i] = newSyntaxError(src, p.pos, p.msg)
	}
	return errors.Join(errs...)
}

// afterDot reports whether toks[i] follows a ".", so it is a name (t.end),
// not a keyword.
func afterDot(toks []Token, i int) bool {
	return i > 0 && toks[i-1].Kind == Punct && toks[i-1].Text == "."
}
