package store

import (
	"errors"
	"fmt"
	"strconv"
)

var ErrMissing = errors.New("missing")

var Opened []string

var Default = mustOpen("default")

// mustOpen is what a package-level initializer with a must call becomes: the error is a panic.
func mustOpen(name string) *DB {
	db, err := open(name)
	if err != nil {
		panic(err)
	}
	return db
}

func open(name string) (*DB, error) {
	if name == "" {
		return nil, fmt.Errorf("open: empty name: %w", ErrMissing)
	}
	Opened = append(Opened, name)
	return &DB{name: name, rows: map[string]string{"one": "1", "two": "2", "word": "w"}}, nil
}

type DB struct {
	name string
	rows map[string]string
}

func Open(name string) (*DB, error) {
	db, err := open(name)
	if err != nil {
		return nil, err
	}
	fmt.Println("store: opened", db.name)
	return db, nil
}

func (d *DB) Get(key string) (int, error) {
	v, ok := d.rows[key]
	if !ok {
		return 0, fmt.Errorf("get %s/%s: %w", d.name, key, ErrMissing)
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (d *DB) Close() error {
	fmt.Println("store: close", d.name)
	if d.name == "bad" {
		return fmt.Errorf("close %s: %w", d.name, ErrMissing)
	}
	return nil
}

type Getter interface {
	Get(key string) (int, error)
}

func Pair(g Getter, a, b string) (int, int, error) {
	x, err := g.Get(a)
	if err != nil {
		return 0, 0, err
	}
	y, err := g.Get(b)
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

func Parse[T int | int64](s string) (T, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return T(n), nil
}

func Sum(db *DB, keys ...string) (int, error) {
	total := 0
	for _, k := range keys {
		n, err := db.Get(k)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func Touch(db *DB) error {
	if _, err := db.Get("one"); err != nil {
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}
	return nil
}

func init() {
	if err := initErr(); err != nil {
		panic(err)
	}
}

func initErr() error {
	fmt.Println("store: init", len(Opened), Default.name)
	return nil
}
