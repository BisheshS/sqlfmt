// Package sqlfmt formats flat SQL SELECT statements.
//
// Layout: each clause keyword (SELECT, FROM, JOIN, WHERE, ...) sits on its own
// line, in upper case. Its body is indented by two spaces, one comma-separated
// item per line. Parentheses stay inline, and only the clause keywords at
// parenthesis depth 0 start a new line, so subqueries are not laid out yet.
package sqlfmt

import "strings"

const indentUnit = "  "

// Format returns sql laid out with one clause per line.
//
// It returns an error, and no output, for input that is broken at the text
// level: an unterminated string, quoted identifier or block comment, an
// unmatched parenthesis, or a CASE without END. It does not check the SQL
// grammar: input it does not understand is passed through with normal spacing.
func Format(sql string) (string, error) {
	toks, err := Tokenize(sql)
	if err != nil {
		return "", err
	}
	if err := validate(sql, toks); err != nil {
		return "", err
	}
	p := &printer{}
	for i := 0; i < len(toks); i++ {
		i = p.token(toks, i)
	}
	p.flush()
	for len(p.lines) > 0 && p.lines[len(p.lines)-1] == "" {
		p.lines = p.lines[:len(p.lines)-1]
	}
	return strings.Join(p.lines, "\n"), nil
}

type printer struct {
	lines []string
	cur   strings.Builder // current line, without indentation
	level int             // indent level of the current line

	pendingBlank bool // a statement just ended: put a blank line before the next line

	prev      *Token // last token written, nil at the start of a statement or clause
	noSpace   bool   // no space before the next token (after "(", ".", "::" or a unary sign)
	breakNext bool   // start a new line before the next token

	clause  clause // current clause; zero value before the first one
	parens  int    // open parenthesis depth
	cases   int    // open CASE depth
	between bool   // the next AND belongs to a BETWEEN
}

// token handles toks[i] and returns the index of the last token it used.
func (p *printer) token(toks []Token, i int) int {
	t := toks[i]
	switch t.Kind {
	case LineComment:
		p.lineComment(t)
	case Word:
		return p.word(toks, i)
	case Punct:
		p.punct(t.Text)
	default: // Number, String, QuotedIdent, BlockComment, Op
		p.write(t.Kind, t.Text)
	}
	return i
}

func (p *printer) word(toks []Token, i int) int {
	t := toks[i]

	// A word after "." is a column name (t.order), never a keyword.
	if p.prev != nil && p.prev.Kind == Punct && p.prev.Text == "." {
		p.write(Word, t.Text)
		return i
	}

	up := strings.ToUpper(t.Text)
	kw, isKeyword := keywords[up]
	if kw.Head != nil && p.parens == 0 {
		if c, n := kw.Head(toks, i); n > 0 {
			p.startClause(c)
			return i + n - 1
		}
	}
	if kw.Before != nil {
		kw.Before(p)
	}

	text := t.Text
	if isKeyword {
		text = up
	}
	p.write(Word, text)
	return i
}

func (p *printer) punct(text string) {
	p.write(Punct, text)
	switch text {
	case "(":
		p.parens++
	case ")":
		p.parens = max(p.parens-1, 0)
	case ",":
		if p.clause.Name != "" && p.parens == 0 && p.cases == 0 {
			p.breakNext = true
		}
	case ";":
		p.endStatement()
	}
}

// startClause puts the clause head on its own line and moves to the body.
func (p *printer) startClause(c clause) {
	p.flush()
	p.level = 0
	p.cur.WriteString(c.Name)
	p.flush()
	p.level = 1
	p.clause = c
	p.between = false
	p.breakNext = false
	p.noSpace = false
	p.prev = nil
}

func (p *printer) endStatement() {
	p.flush()
	p.pendingBlank = true // added by flush, so a trailing comment can still join the ";" line
	p.level = 0
	p.clause = clause{}
	p.parens, p.cases = 0, 0
	p.between = false
	p.breakNext = false
	p.noSpace = false
	p.prev = nil
}

// lineComment keeps a trailing comment on its line (after a "," or ";" too)
// and puts an own-line comment on a new line. Either way the next token
// starts a new line.
func (p *printer) lineComment(t Token) {
	if t.NewlineBefore {
		p.flush()
	}
	switch {
	case p.cur.Len() > 0:
		p.cur.WriteByte(' ')
		p.cur.WriteString(t.Text)
	case p.pendingBlank && !t.NewlineBefore && len(p.lines) > 0:
		p.lines[len(p.lines)-1] += " " + t.Text // trailing comment after ";"
	default:
		p.cur.WriteString(t.Text)
	}
	p.breakNext = true
}

func (p *printer) write(k Kind, text string) {
	if p.breakNext {
		p.flush()
		p.breakNext = false
	}
	if p.cur.Len() > 0 && p.spaceBefore(k, text) {
		p.cur.WriteByte(' ')
	}
	unary := p.isUnary(k, text)
	p.cur.WriteString(text)
	p.prev = &Token{Kind: k, Text: text}
	p.noSpace = k == Punct && (text == "(" || text == ".") || k == Op && text == "::" || unary
}

func (p *printer) spaceBefore(k Kind, text string) bool {
	if p.noSpace {
		return false
	}
	switch {
	case k == Punct && (text == "," || text == ")" || text == ";" || text == "."):
		return false
	case k == Op && text == "::":
		return false
	case k == Punct && text == "(":
		// Function call: count(x), LEFT(x, 3). Otherwise: IN (1, 2).
		pv := p.prev
		if pv != nil && pv.Kind == Word {
			if kw, isKeyword := keywords[pv.Text]; !isKeyword || kw.FuncLike {
				return false
			}
		}
	}
	return true
}

// isUnary reports whether the token is a sign, as in -1 or (-x), not a minus.
func (p *printer) isUnary(k Kind, text string) bool {
	if k != Op || (text != "-" && text != "+") {
		return false
	}
	pv := p.prev
	switch {
	case pv == nil, pv.Kind == Op:
		return true
	case pv.Kind == Punct:
		return pv.Text == "(" || pv.Text == ","
	case pv.Kind == Word:
		kw, isKeyword := keywords[pv.Text]
		return isKeyword && !kw.EndsValue
	}
	return false
}

func (p *printer) flush() {
	if p.cur.Len() == 0 {
		return
	}
	if p.pendingBlank {
		p.lines = append(p.lines, "") // blank line between statements
		p.pendingBlank = false
	}
	p.lines = append(p.lines, strings.Repeat(indentUnit, p.level)+p.cur.String())
	p.cur.Reset()
}
