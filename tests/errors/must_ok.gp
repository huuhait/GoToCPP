// phase: 09

// Programs with ! that check without errors: every shape of Phase 10 that is allowed.
package main

import (
	"errors"
	"fmt"
	"os"
)

type Config struct{ Port int }

func parse(s string) (int, error) { return len(s), nil }

func pair(s string) (int, string, error) { return len(s), s, nil }

func load!(path string) (cfg Config, err error) {
	defer errors.Wrapf(&err, "load %s", path)
	cfg.Port = parse(path)
	_, name := pair(path)
	fmt.Println(name)
	return cfg, nil
}

type Server struct{ port int }

func (s *Server) Start!() {
	s.port = parse("x")
}

func apply[T any](v T, f func(T) (T, error)) (T, error) {
	return f!(v), nil
}

func generic![T any](v T) T {
	return v
}

func kinds(x int) (int, error) {
	if!(x > 0) {
		return 0, nil
	}
	switch {
	case !(x < 0):
		return 1, nil
	}
	f, err := os.Open("x")
	if err == nil {
		f.Close!()
	}
	return parse!("a") + parse!("b"), nil
}

func main!() {
	cfg := load("a.conf")
	s := &Server{}
	s.Start()
	fmt.Println(cfg, s.port)
}
