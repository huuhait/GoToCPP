package conf

import (
	"fmt"

	"example.com/mustx/store"
)

type wrapfError struct {
	msg string
	err error
}

func (e *wrapfError) Error() string { return e.msg + ": " + e.err.Error() }
func (e *wrapfError) Unwrap() error { return e.err }

// Wrapf is errors.Wrapf of gocpp, in plain Go.
func Wrapf(errp *error, format string, args ...any) {
	if *errp == nil {
		return
	}
	*errp = &wrapfError{msg: fmt.Sprintf(format, args...), err: *errp}
}

type Config struct{ Port, Workers int }

func Load(db *store.DB, keys ...string) (cfg Config, err error) {
	defer Wrapf(&err, "load config")
	cfg.Port, err = db.Get(keys[0])
	if err != nil {
		return Config{}, err
	}
	cfg.Workers, err = db.Get(keys[1])
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Check(db *store.DB) (int, error) {
	p, err := store.Parse[int]("40")
	if err != nil {
		return 0, err
	}
	g, err := db.Get("two")
	if err != nil {
		return 0, err
	}
	n := p + g
	fmt.Println("conf: check", n)
	return n, nil
}

func Fresh(name string) (n int, err error) {
	db, err := store.Open(name)
	if err != nil {
		return 0, err
	}
	defer func() {
		if cerr := db.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	one, err := db.Get("one")
	if err != nil {
		return 0, err
	}
	return one, nil
}
