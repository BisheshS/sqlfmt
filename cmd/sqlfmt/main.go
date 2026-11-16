// Command sqlfmt formats SQL from a file argument or from stdin.
//
//	echo 'select a,b from t' | go run ./cmd/sqlfmt
//	go run ./cmd/sqlfmt query.sql
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/BisheshS/sqlfmt"
)

func main() {
	var in []byte
	var err error
	if len(os.Args) > 1 {
		in, err = os.ReadFile(os.Args[1])
	} else {
		in, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "sqlfmt:", err)
		os.Exit(1)
	}
	out, err := sqlfmt.Format(string(in))
	if err != nil {
		fmt.Fprintln(os.Stderr, "sqlfmt:", err)
		os.Exit(1)
	}
	fmt.Println(out)
}
