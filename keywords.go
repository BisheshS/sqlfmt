package sqlfmt

import "strings"

// This file is the keyword registry. To support a new keyword, add one line to
// the table in init. The printer never compares keyword strings itself: it
// looks the word up here and runs whatever behaviour the entry declares.

// keyword describes how the printer treats one SQL keyword. Every field is
// optional; an empty keyword{} is only upper-cased.
type keyword struct {
	// Head makes the keyword start a clause (own line, indented body). It
	// reports the clause and how many tokens the head uses, or n == 0 when
	// the tokens at i are not a clause head after all. Used only at
	// parenthesis depth 0.
	Head headFunc

	// Before runs just before the keyword is written, when it did not start a
	// clause. Use it to break the line or update the printer's state.
	Before func(p *printer)

	// Check reports structural problems, such as a CASE that is never closed.
	Check func(c *checker, t Token)

	FuncLike  bool // can also be a function name, so no space before "(": LEFT(x, 3)
	EndsValue bool // can end an operand, so a following + or - is a binary operator
	JoinMod   bool // can come before JOIN: INNER, LEFT, OUTER, ...
}

// clause is the clause the printer is currently in.
type clause struct {
	Name  string // head text: "WHERE", "LEFT OUTER JOIN", ...; "" before the first clause
	Logic bool   // AND / OR start a new line (conditions)
	Join  bool   // ON starts a new line
}

type headFunc func(toks []Token, i int) (c clause, n int)

// keywords is filled in init because the join head looks keywords up.
var keywords map[string]keyword

func init() {
	keywords = map[string]keyword{
		// Clause heads.
		"SELECT": {Head: simpleHead(clause{})},
		"FROM":   {Head: simpleHead(clause{})},
		"WHERE":  {Head: simpleHead(clause{Logic: true})},
		"HAVING": {Head: simpleHead(clause{Logic: true})},
		"LIMIT":  {Head: simpleHead(clause{})},
		"OFFSET": {Head: simpleHead(clause{})},
		"GROUP":  {Head: headThenBy},
		"ORDER":  {Head: headThenBy},

		// Joins: [INNER|LEFT|...] [OUTER] JOIN.
		"JOIN":    {Head: joinHead},
		"INNER":   {Head: joinHead, JoinMod: true},
		"LEFT":    {Head: joinHead, JoinMod: true, FuncLike: true},
		"RIGHT":   {Head: joinHead, JoinMod: true, FuncLike: true},
		"FULL":    {Head: joinHead, JoinMod: true},
		"OUTER":   {Head: joinHead, JoinMod: true},
		"CROSS":   {Head: joinHead, JoinMod: true},
		"NATURAL": {Head: joinHead, JoinMod: true},

		// Keywords with line-break or state behaviour.
		"ON":      {Before: breakBeforeOn},
		"AND":     {Before: beforeAnd},
		"OR":      {Before: breakBeforeLogic},
		"BETWEEN": {Before: beforeBetween},
		"CASE": {Before: openCase, Check: func(c *checker, t Token) {
			c.opened("CASE", t.Pos, "CASE without END")
		}},
		"END": {Before: closeCase, EndsValue: true, Check: func(c *checker, t Token) {
			c.closed("CASE", t.Pos, "END without CASE")
		}},

		// Operand-ending values.
		"NULL":  {EndsValue: true},
		"TRUE":  {EndsValue: true},
		"FALSE": {EndsValue: true},

		// Upper-case only.
		"BY": {}, "USING": {}, "AS": {}, "NOT": {}, "IN": {}, "IS": {},
		"LIKE": {}, "ILIKE": {}, "EXISTS": {}, "DISTINCT": {}, "ALL": {},
		"ASC": {}, "DESC": {}, "WHEN": {}, "THEN": {}, "ELSE": {},
	}
}

// --- clause heads ---

// simpleHead is a one-word clause head such as WHERE.
func simpleHead(c clause) headFunc {
	return func(toks []Token, i int) (clause, int) {
		out := c
		out.Name, _ = wordAt(toks, i)
		return out, 1
	}
}

// headThenBy is a two-word head: GROUP BY, ORDER BY.
func headThenBy(toks []Token, i int) (clause, int) {
	first, _ := wordAt(toks, i)
	if next, ok := wordAt(toks, i+1); ok && next == "BY" {
		return clause{Name: first + " BY"}, 2
	}
	return clause{}, 0
}

// joinHead is JOIN with optional modifiers before it. It does not match the
// function LEFT(x, 3), because no JOIN follows.
func joinHead(toks []Token, i int) (clause, int) {
	var words []string
	for j := i; ; j++ {
		w, ok := wordAt(toks, j)
		if !ok {
			return clause{}, 0
		}
		words = append(words, w)
		if w == "JOIN" {
			return clause{Name: strings.Join(words, " "), Logic: true, Join: true}, j - i + 1
		}
		if !keywords[w].JoinMod {
			return clause{}, 0
		}
	}
}

// wordAt returns the upper-cased text of toks[i] when it is a Word.
func wordAt(toks []Token, i int) (string, bool) {
	if i < len(toks) && toks[i].Kind == Word {
		return strings.ToUpper(toks[i].Text), true
	}
	return "", false
}

// --- behaviour hooks ---

func breakBeforeOn(p *printer) {
	if p.parens == 0 && p.clause.Join {
		p.flush()
	}
}

// breakBeforeLogic starts a new line for AND / OR in a condition clause,
// unless they are nested in parentheses or CASE.
func breakBeforeLogic(p *printer) {
	if p.parens == 0 && p.cases == 0 && p.clause.Logic {
		p.flush()
	}
}

func beforeAnd(p *printer) {
	if p.between { // BETWEEN a AND b: this AND stays inline
		p.between = false
		return
	}
	breakBeforeLogic(p)
}

func beforeBetween(p *printer) {
	if p.parens == 0 {
		p.between = true
	}
}

func openCase(p *printer) { p.cases++ }

func closeCase(p *printer) {
	if p.cases > 0 {
		p.cases--
	}
}
