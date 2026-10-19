package main

import (
	"errors"
	"fmt"
	"os"

	"example.com/mustx/conf"
	"example.com/mustx/store"
)

func report(what string, v any, err error) {
	fmt.Println(what, v, err, errors.Is(err, store.ErrMissing))
}

func total(name string, keys ...string) (int, error) {
	db, err := store.Open(name)
	if err != nil {
		return 0, err
	}
	pair1, pair2, err := store.Pair(db, keys[0], keys[1])
	if err != nil {
		return 0, err
	}
	s, err := store.Sum(db, keys...)
	if err != nil {
		return 0, err
	}
	return pair1 + pair2 + s, nil
}

func run() error {
	n, err := total("one-store", "one", "two")
	if err != nil {
		return err
	}
	fmt.Println("run: total", n)
	cfg, err := conf.Load(store.Default, "one", "word")
	report("run: load", cfg, err)
	cfg, err = conf.Load(store.Default, "one", "two")
	if err != nil {
		return err
	}
	fmt.Println("run: cfg", cfg)
	fmt.Println(conf.Check(store.Default))
	fmt.Println(conf.Fresh("fresh"))
	n64, err := store.Parse[int64]("77")
	if err != nil {
		return err
	}
	fmt.Println("run: parse", n64)
	if err := store.Touch(store.Default); err != nil {
		return err
	}
	fmt.Println("run: not reached by the failing call below")
	fmt.Println(conf.Fresh("bad"))
	if _, err := store.Parse[int]("x"); err != nil {
		return err
	}
	fmt.Println("run: not reached")
	return nil
}

func main() {
	fmt.Println("main: opened", store.Opened)
	n, err := total("a", "one", "nokey")
	report("main: total", n, err)
	err = run()
	fmt.Println("main: run gives", err)
	fmt.Println("main: opened", store.Opened)
	_ = os.Args
}
