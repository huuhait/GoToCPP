// phase: 09

// Must-errors reads small "key = value" config files and shows ! (must). A
// must function (func loadConfig!) returns every error that its code drops; a
// must call (F!(...)) returns the error of one call; func main!() prints an
// error and exits with 1. This run succeeds: the broken configs are loaded
// without !, so their errors come back as values and are printed.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Name    string
	Port    int
	Workers int
}

// parseLine is plain Go: it splits one "key = value" line.
func parseLine(line string) (key, value string, err error) {
	k, v, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", fmt.Errorf("line %q has no '='", line)
	}
	return strings.TrimSpace(k), strings.TrimSpace(v), nil
}

// loadConfig is a must function: the errors of os.Open, parseLine,
// strconv.Atoi and sc.Err are returned for you. It declares its error
// result, so a defer can add the file name to it, and a failing f.Close is
// returned too. Callers see
//
//	func loadConfig(path string) (cfg Config, err error)
func loadConfig!(path string) (cfg Config, err error) {
	defer errors.Wrapf(&err, "load %s", path)
	f := os.Open(path)
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value := parseLine(line)
		switch key {
		case "name":
			cfg.Name = value
		case "port":
			cfg.Port = strconv.Atoi(value)
		case "workers":
			cfg.Workers = strconv.Atoi(value)
		default:
			return Config{}, fmt.Errorf("unknown key %q", key)
		}
	}
	sc.Err() // a call as a statement: its error is returned
	if cfg.Port == 0 {
		return Config{}, errors.New("port is missing")
	}
	return cfg, nil
}

// port is a must function without a declared error, so it gets one: callers
// see func port(path string) (int, error). return p means success, and one
// more value fails on purpose.
func port!(path string) int {
	cfg := loadConfig(path)
	if cfg.Port < 1024 {
		return 0, fmt.Errorf("port %d is below 1024", cfg.Port)
	}
	return cfg.Port
}

// endpoint is a normal function: ! on one call returns that call's error.
func endpoint(path string) (string, error) {
	cfg := loadConfig!(path)
	return cfg.Name + ":" + strconv.Itoa(cfg.Port), nil
}

// lookup is a normal function too. a, b := F!(...) keeps two results.
func lookup(path, want string) (string, error) {
	data := os.ReadFile!(path)
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value := parseLine!(line)
		if key == want {
			return value, nil
		}
	}
	return "", fmt.Errorf("%s: no key %q", path, want)
}

// workers has no error result, so there F! panics on an error, like Go's
// Must functions (regexp.MustCompile).
func workers(path string) int {
	return loadConfig!(path).Workers
}

// In main!, an error that the code drops is printed as "<program>: <error>"
// and the program exits with 1, after main's deferred calls ran.
func main!() {
	defer fmt.Println("main: done")

	// No !: plain Go, the errors are values.
	for _, path := range []string{"noport.conf", "badport.conf", "badline.conf", "unknown.conf"} {
		_, err := loadConfig(path)
		fmt.Println("error:", err)
	}
	_, err := loadConfig("missing.conf")
	fmt.Println("missing.conf: not found:", errors.Is(err, os.ErrNotExist))
	_, err = port("lowport.conf")
	fmt.Println("error:", err)

	// Dropped errors: each one would end the program.
	cfg := loadConfig("app.conf")
	fmt.Printf("config: %+v\n", cfg)
	fmt.Println("port:", port("app.conf"))
	fmt.Println("endpoint:", endpoint("app.conf"))
	fmt.Println("name:", lookup("app.conf", "name"), "workers:", workers("app.conf"))
}
