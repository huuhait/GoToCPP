// phase: 09

// Package conf adds context with errors.Wrapf in a must function that declares its error, and
// uses the must functions of store.
package conf

import (
	"errors"
	"fmt"

	"example.com/mustx/store"
)

// A Config holds two numbers.
type Config struct{ Port, Workers int }

// Load reads the keys of a store; the error says which store it was.
func Load!(db *store.DB, keys ...string) (cfg Config, err error) {
	defer errors.Wrapf(&err, "load config")
	cfg.Port = db.Get!(keys[0])
	cfg.Workers = db.Get!(keys[1])
	return cfg, nil
}

// Must is a normal function that uses must calls.
func Check(db *store.DB) (int, error) {
	n := store.Parse![int]("40") + db.Get!("two")
	fmt.Println("conf: check", n)
	return n, nil
}

// Fresh opens a store and closes it with a deferred must call: a failing Close is returned.
func Fresh(name string) (n int, err error) {
	db := store.Open!(name)
	defer db.Close!()
	return db.Get!("one"), nil
}
