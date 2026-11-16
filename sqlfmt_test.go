package sqlfmt

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// splitBlocks splits text into blocks separated by one or more blank lines.
func splitBlocks(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var blocks []string
	for _, b := range regexp.MustCompile(`\n\s*\n`).Split(strings.TrimSpace(text), -1) {
		if b = strings.TrimSpace(b); b != "" {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

// TestGolden checks every testdata/<name>.sql against testdata/<name>.golden.
// Both files hold queries separated by blank lines; the Nth query in the .sql
// file must format to the Nth block in the .golden file. When a query is
// rejected, the block is "error: " followed by the error text.
func TestGolden(t *testing.T) {
	files, err := filepath.Glob("testdata/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no testdata/*.sql files found (err: %v)", err)
	}
	for _, sqlFile := range files {
		name := strings.TrimSuffix(filepath.Base(sqlFile), ".sql")
		goldenFile := strings.TrimSuffix(sqlFile, ".sql") + ".golden"

		in, err := os.ReadFile(sqlFile)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(goldenFile)
		if err != nil {
			t.Fatalf("%s needs a matching %s: %v", sqlFile, goldenFile, err)
		}
		queries, wants := splitBlocks(string(in)), splitBlocks(string(want))
		if len(queries) != len(wants) {
			t.Fatalf("%s has %d queries but %s has %d outputs", sqlFile, len(queries), goldenFile, len(wants))
		}

		for i, q := range queries {
			title, _, _ := strings.Cut(q, "\n")
			t.Run(fmt.Sprintf("%s/%d %s", name, i+1, title), func(t *testing.T) {
				got, err := Format(q)
				failing := err != nil
				if failing {
					got = "error: " + err.Error()
				}
				if got != wants[i] {
					t.Errorf("query %d in %s: wrong output\n--- input\n%s\n--- got\n%s\n--- want (%s)\n%s",
						i+1, sqlFile, q, got, goldenFile, wants[i])
				}
				if failing {
					return // error cases have no formatted output to re-check
				}
				if again, _ := Format(wants[i]); again != wants[i] {
					t.Errorf("query %d in %s: formatting the expected output changes it\n--- got\n%s", i+1, sqlFile, again)
				}
			})
		}
	}
}

func TestTokenize(t *testing.T) {
	type tok struct {
		kind Kind
		text string
	}
	tests := []struct {
		name string
		in   string
		want []tok
	}{
		{"words and punct", "a.b, c", []tok{{Word, "a"}, {Punct, "."}, {Word, "b"}, {Punct, ","}, {Word, "c"}}},
		{"escaped quote", `'it''s'`, []tok{{String, `'it''s'`}}},
		{"quoted identifiers", "\"a\" `b`", []tok{{QuotedIdent, `"a"`}, {QuotedIdent, "`b`"}}},
		{"numbers", "1 2.5 .5 1e-3", []tok{{Number, "1"}, {Number, "2.5"}, {Number, ".5"}, {Number, "1e-3"}}},
		{"comments", "-- x\n/* y */", []tok{{LineComment, "-- x"}, {BlockComment, "/* y */"}}},
		{"nested comment", "/* a /* b */ c */ x", []tok{{BlockComment, "/* a /* b */ c */"}, {Word, "x"}}},
		{"operators", "a<=b::int", []tok{{Word, "a"}, {Op, "<="}, {Word, "b"}, {Op, "::"}, {Word, "int"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []tok
			toks, err := Tokenize(tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, x := range toks {
				got = append(got, tok{x.Kind, x.Text})
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestTokenizeErrors(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"string", "select 'abc", "1:8: unterminated string"},
		{"quoted identifier", "select a\nfrom \"t", "2:6: unterminated quoted identifier"},
		{"backtick identifier", "select `a", "1:8: unterminated quoted identifier"},
		{"block comment", "select 1 /* abc", "1:10: unterminated block comment"},
		{"nested comment closed once", "select 1 /* a /* b */", "1:10: unterminated block comment"},
		{"escaped quote is not the end", "select 'it''s", "1:8: unterminated string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Tokenize(tt.in)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("got error %v, want %q", err, tt.want)
			}
		})
	}
}
