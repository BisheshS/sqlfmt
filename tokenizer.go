package sqlfmt

import "strings"

// Kind is the type of a Token.
type Kind int

const (
	Word         Kind = iota // keyword or unquoted identifier: select, users, :param
	Number                   // 42, 2.5, .5, 1e10
	String                   // 'text' ('' is an escaped quote)
	QuotedIdent              // "name" or `name`
	LineComment              // -- text
	BlockComment             // /* text */
	Op                       // = <> || :: + - * ...
	Punct                    // , ( ) ; .
)

// Token is one lexical unit. Whitespace is dropped, but NewlineBefore records
// whether a line break came before the token (used to place comments).
type Token struct {
	Kind          Kind
	Text          string
	Pos           int // byte offset of the token in the input
	NewlineBefore bool
}

// Tokenize splits src into tokens. It returns a *SyntaxError for an
// unterminated string, quoted identifier or block comment; the tokens before
// the error are returned too.
func Tokenize(src string) ([]Token, error) {
	var toks []Token
	for i := 0; ; {
		start, newline := skipSpace(src, i)
		if start >= len(src) {
			return toks, nil
		}
		kind, end, problem := scanToken(src, start, endsOperand(toks))
		if problem != "" {
			return toks, newSyntaxError(src, start, problem)
		}

		text := src[start:end]
		if kind == LineComment {
			text = strings.TrimRight(text, " \t\r")
		}
		toks = append(toks, Token{Kind: kind, Text: text, Pos: start, NewlineBefore: newline})
		i = end
	}
}

// skipSpace returns the index of the first non-space byte at or after i, and
// whether a newline was skipped on the way.
func skipSpace(s string, i int) (next int, newline bool) {
	for i < len(s) && isSpace(s[i]) {
		newline = newline || s[i] == '\n'
		i++
	}
	return i, newline
}

// scanToken reads the token that starts at s[i] and returns its kind and the
// index after it. afterOperand says the previous token ended an operand.
// problem is not empty when the token is not terminated.
func scanToken(s string, i int, afterOperand bool) (kind Kind, end int, problem string) {
	c, next := s[i], at(s, i+1)
	switch {
	case c == '-' && next == '-':
		return LineComment, scanWhile(s, i, func(c byte) bool { return c != '\n' }), ""
	case c == '/' && next == '*':
		end, ok := scanBlockComment(s, i)
		return BlockComment, end, unterminated(ok, "block comment")
	case c == '\'':
		end, ok := scanQuoted(s, i)
		return String, end, unterminated(ok, "string")
	case c == '"' || c == '`':
		end, ok := scanQuoted(s, i)
		return QuotedIdent, end, unterminated(ok, "quoted identifier")
	case isDigit(c), c == '.' && isDigit(next) && !afterOperand: // 42, .5 (but t.5 is not a number)
		return Number, scanNumber(s, i), ""
	case isIdentStart(c), c == ':' && isIdentStart(next): // name, :param
		return Word, scanWhile(s, i+1, isIdentPart), ""
	case strings.IndexByte(",().;", c) >= 0:
		return Punct, i + 1, ""
	}
	return Op, i + opLen(s, i), ""
}

func unterminated(ok bool, what string) string {
	if ok {
		return ""
	}
	return "unterminated " + what
}

// scanBlockComment returns the index after the /* */ comment at s[i], and
// false when the comment is not closed. Block comments nest, as in Postgres:
// /* a /* b */ c */ is one comment.
func scanBlockComment(s string, i int) (end int, ok bool) {
	depth := 0
	for i < len(s) {
		switch {
		case strings.HasPrefix(s[i:], "/*"):
			depth++
			i += 2
		case strings.HasPrefix(s[i:], "*/"):
			depth--
			i += 2
			if depth == 0 {
				return i, true
			}
		default:
			i++
		}
	}
	return len(s), false
}

// scanQuoted returns the index after the quoted run that starts at s[i], and
// false when the closing quote is missing. A doubled quote inside the run is
// an escape.
func scanQuoted(s string, i int) (end int, ok bool) {
	q := s[i]
	for i++; i < len(s); i++ {
		if s[i] != q {
			continue
		}
		if at(s, i+1) != q {
			return i + 1, true
		}
		i++ // skip the escaped quote
	}
	return len(s), false
}

// scanNumber returns the index after the number that starts at s[i]:
// digits, an optional fraction, and an optional exponent.
func scanNumber(s string, i int) int {
	i = scanWhile(s, i, isDigit)
	if at(s, i) == '.' && isDigit(at(s, i+1)) {
		i = scanWhile(s, i+1, isDigit)
	}
	if e := at(s, i); e == 'e' || e == 'E' {
		j := i + 1
		if sign := at(s, j); sign == '+' || sign == '-' {
			j++
		}
		if isDigit(at(s, j)) {
			i = scanWhile(s, j, isDigit)
		}
	}
	return i
}

// opLen returns the byte length of the operator at s[i].
func opLen(s string, i int) int {
	for _, op := range []string{"->>", "->", "<=", ">=", "<>", "!=", "||", "::", "=="} {
		if strings.HasPrefix(s[i:], op) {
			return len(op)
		}
	}
	return 1
}

// endsOperand reports whether the last token can end an operand, so that a
// following "." is a qualifier (t.col) and not the start of a number (.5).
func endsOperand(toks []Token) bool {
	if len(toks) == 0 {
		return false
	}
	last := toks[len(toks)-1]
	return last.Kind == Word || last.Kind == QuotedIdent || (last.Kind == Punct && last.Text == ")")
}

// scanWhile returns the index of the first byte at or after i that fails ok.
func scanWhile(s string, i int, ok func(byte) bool) int {
	for i < len(s) && ok(s[i]) {
		i++
	}
	return i
}

// at returns s[i], or 0 when i is out of range.
func at(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

// isSpace reports whether c is whitespace, including newlines.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isIdentStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '$' || c == '@' || c >= 0x80
}

func isIdentPart(c byte) bool { return isIdentStart(c) || isDigit(c) }
