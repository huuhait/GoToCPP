// phase: 09

// Package store has must functions, methods and generic functions that other packages call: their
// signature (the extra error result) must reach the importing package.
package store

import (
	"errors"
	"fmt"
	"strconv"
)

var ErrMissing = errors.New("missing")

// Opened lists the stores that were opened, in order; a package-level initializer with a must call
// fills Default before main runs.
var Opened []string

var Default = open!("default")

func open!(name string) *DB {
	if name == "" {
		return nil, fmt.Errorf("open: empty name: %w", ErrMissing)
	}
	Opened = append(Opened, name)
	return &DB{name: name, rows: map[string]string{"one": "1", "two": "2", "word": "w"}}
}

// A DB is a tiny key-value store.
type DB struct {
	name string
	rows map[string]string
}

// Open is a must function: callers see func Open(name string) (*DB, error).
func Open!(name string) *DB {
	db := open!(name)
	fmt.Println("store: opened", db.name)
	return db
}

// Get returns the number stored under key.
func (d *DB) Get!(key string) int {
	v, ok := d.rows[key]
	if !ok {
		return 0, fmt.Errorf("get %s/%s: %w", d.name, key, ErrMissing)
	}
	n := strconv.Atoi(v)
	return n
}

// Close fails for the store named "bad".
func (d *DB) Close() error {
	fmt.Println("store: close", d.name)
	if d.name == "bad" {
		return fmt.Errorf("close %s: %w", d.name, ErrMissing)
	}
	return nil
}

// A Getter is satisfied by *DB: its must method has the real signature.
type Getter interface {
	Get(key string) (int, error)
}

// Pair gets two keys with one must call each.
func Pair!(g Getter, a, b string) (int, int) {
	x := g.Get!(a)
	y := g.Get!(b)
	return x, y
}

// Parse is generic; a caller picks T.
func Parse![T int | int64](s string) T {
	n := strconv.Atoi(s)
	return T(n)
}

// Sum is a must function with no result but the error.
func Sum!(db *DB, keys ...string) int {
	total := 0
	for _, k := range keys {
		total += db.Get!(k)
	}
	return total
}

// Touch only reports.
func Touch!(db *DB) {
	db.Get("one")
	db.Close()
}

func init!() {
	fmt.Println("store: init", len(Opened), Default.name)
}
