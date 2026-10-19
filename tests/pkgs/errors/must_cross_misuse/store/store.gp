// phase: 08

package store

import "errors"

type MyErr struct{}

func (*MyErr) Error() string { return "my" }

type DB struct{}

func (*DB) Close() error { return nil }

// Open is a must function: callers see func Open(name string) (*DB, error).
func Open!(name string) *DB {
	if name == "" {
		return nil, errors.New("empty")
	}
	return &DB{}
}

// Run has no result but the error.
func Run!() {}

// Typed returns a concrete error type.
func Typed() (int, *MyErr) { return 0, nil }

type Srv struct{}

func (Srv) Start!() {}
