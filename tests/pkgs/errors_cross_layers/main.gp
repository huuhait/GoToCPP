// phase: 06

// A request travels three packages: each layer wraps the error in its own type (Unwrap() error, Unwrap() []error and an Is method); main inspects the whole tree.

package main

import (
	"errors"
	"fmt"

	"example.com/layers/db"
	"example.com/layers/handler"
	"example.com/layers/service"
)

type statuser interface{ Status() handler.Status }

func main() {
	for _, id := range []int{1, 0, -5} {
		err := handler.Handle(id)
		if err == nil {
			fmt.Println(id, "ok")
			continue
		}
		var st statuser
		errors.As(err, &st)
		fmt.Println(id, st.Status(), errors.Is(err, service.ErrInvalid), errors.Is(err, db.ErrNoRows))
		var se service.Error
		if errors.As(err, &se) {
			fmt.Println("  service op:", se.Op, "|", se.Error())
		}
		var qe *db.QueryError
		if errors.As(err, &qe) {
			fmt.Println("  query:", qe.Query, errors.Unwrap(qe) == db.ErrNoRows)
		}
		fmt.Println("  text:", err.Error())
	}
}

// Output:
// 1 ok
// 0 400 true false
//   service op: validate | validate failed: zero id
//   text: http error
// -5 500 false true
//   service op: load | load failed: query find: db: no rows
//   query: find true
//   text: http error
