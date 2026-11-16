# sqlfmt

sqlfmt formats SQL `SELECT` queries. It is written in Go and uses only the standard library.

It puts each clause keyword on its own line, in upper case. It indents the clause body by 2 spaces.
It returns an error for broken input, such as an open quote or an unmatched parenthesis.

## Example

Input:

```sql
select u.id, count(*) as n from users u left join orders o on u.id=o.user_id where u.age>18 and o.total>=10 group by u.id order by n desc limit 5;
```

Output:

```sql
SELECT
  u.id,
  count(*) AS n
FROM
  users u
LEFT JOIN
  orders o
  ON u.id = o.user_id
WHERE
  u.age > 18
  AND o.total >= 10
GROUP BY
  u.id
ORDER BY
  n DESC
LIMIT
  5;
```

## Use it

You need Go. The tool reads a file, or stdin when you give no file.

```sh
go run ./cmd/sqlfmt query.sql
echo "select a,b from t" | go run ./cmd/sqlfmt
```

On an error, the tool prints the error to stderr and exits with code 1.

You can also call it from Go code:

```go
out, err := sqlfmt.Format("select a,b from t")
```

## Layout rules

- Clause keywords start a new line: `SELECT`, `FROM`, `WHERE`, `GROUP BY`, `HAVING`, `ORDER BY`, `LIMIT`, `OFFSET`, and all `JOIN` types.
- The body of a clause is indented by 2 spaces. Each item after a top-level comma goes on its own line.
- `ON` goes on its own line under its `JOIN`.
- `AND` and `OR` go on a new line in `WHERE`, `HAVING`, and `JOIN` conditions.
- `BETWEEN a AND b` stays on one line.
- Text in parentheses and `CASE ... END` stays on one line.
- Keywords become upper case. Names, strings, and comments keep their text.
- Block comments can nest: `/* a /* b */ c */` is one comment.
- A comment on the same line as a `,` or `;` stays on that line.
- A blank line separates statements.

## Errors

`Format` returns an error and no output for these problems:

| Problem | Message |
| --- | --- |
| Open string | `1:8: unterminated string` |
| Open `"name"` or `` `name` `` | `1:8: unterminated quoted identifier` |
| Open `/* comment` | `1:10: unterminated block comment` |
| `(` with no `)` | `1:13: unclosed (` |
| `)` with no `(` | `1:9: unmatched )` |
| `CASE` with no `END` | `1:8: CASE without END` |
| `END` with no `CASE` | `1:10: END without CASE` |

Each message starts with `line:column`. The error is a `*SyntaxError`. If there are many problems, the error lists all of them.

The tool does not check SQL grammar. It does not know if a table or column exists.

## Tests

Run all tests:

```sh
go test ./...
```

Run one group, with details:

```sh
go test -run 'TestGolden/where' -v ./
```

### Golden tests

Golden tests live in `testdata/`. Each group is a pair of files:

- `name.sql` holds the input queries.
- `name.golden` holds the expected output.

Put one blank line between queries in both files. Query number N must match output number N.
A query must not contain a blank line.

To add a test, add the query to the end of a `.sql` file. Then add the expected output to the end of the matching `.golden` file.
To add a group, create a new `.sql` and `.golden` pair.

For a query that must fail, write `error: ` and the error message as the expected output.
See `testdata/errors.golden`.

The test also formats each expected output again. The result must not change.

## Code map

| File | Job |
| --- | --- |
| `tokenizer.go` | Splits text into tokens. Reports open strings and comments. |
| `keywords.go` | The keyword table. Each keyword has its own behavior. |
| `printer.go` | Builds the output lines from the tokens. |
| `errors.go` | The error type and the check for unmatched parentheses and `CASE`. |
| `cmd/sqlfmt/main.go` | The command-line tool. |

### Add a keyword

Add one line to the table in `keywords.go`. A plain keyword needs only `"NAME": {}`.
A keyword with special behavior sets a field, such as `Head`, `Before`, or `Check`. The printer has no keyword names in its code.

## Limits

- Subqueries stay on one line.
- `UNION`, `WITH`, `INSERT`, `UPDATE`, `DELETE`, and window functions are not laid out.
- `[bracket]` names, `E'...'` strings, and `0x` numbers are not supported.
