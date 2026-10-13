// The twin of ../main.gp: the same program with every ! written out by hand
// as the Go code that gocpp generates for it (Phase 10 §2.1 of the plan).
// Real Go runs this twin to make ../output.txt, and the gocpp program must
// print the same. Names that start with gp_ stand for names the compiler
// makes up; the comments show the gocpp line each block comes from.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Name    string
	Port    int
	Workers int
}

func parseLine(line string) (key, value string, err error) {
	k, v, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", fmt.Errorf("line %q has no '='", line)
	}
	return strings.TrimSpace(k), strings.TrimSpace(v), nil
}

// gp_wrapf is errors.Wrapf(&err, format, args...), a gocpp addition to the
// errors package: a non-nil *errp becomes "<message>: <*errp>", and it still
// unwraps to the old error, like fmt.Errorf with %w.
func gp_wrapf(errp *error, format string, args ...any) {
	if *errp != nil {
		*errp = fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), *errp)
	}
}

// gp_deferErr is what a deferred call does with its error in a must
// function: the error is returned when the function was returning nil.
func gp_deferErr(errp *error, err error) {
	if err != nil && *errp == nil {
		*errp = err
	}
}

// func loadConfig!(path string) (cfg Config, err error): the error result is
// declared, so the signature stays as it is.
func loadConfig(path string) (cfg Config, err error) {
	defer gp_wrapf(&err, "load %s", path)
	// f := os.Open(path)
	gp_t1, gp_e1 := os.Open(path)
	if gp_e1 != nil {
		return Config{}, gp_e1
	}
	f := gp_t1
	// defer f.Close(): the receiver is evaluated now, Close runs at the end
	gp_r2 := f
	defer func() { gp_deferErr(&err, gp_r2.Close()) }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// key, value := parseLine(line)
		gp_t3, gp_t4, gp_e3 := parseLine(line)
		if gp_e3 != nil {
			return Config{}, gp_e3
		}
		key, value := gp_t3, gp_t4
		switch key {
		case "name":
			cfg.Name = value
		case "port":
			// cfg.Port = strconv.Atoi(value)
			gp_t5, gp_e5 := strconv.Atoi(value)
			if gp_e5 != nil {
				return Config{}, gp_e5
			}
			cfg.Port = gp_t5
		case "workers":
			// cfg.Workers = strconv.Atoi(value)
			gp_t6, gp_e6 := strconv.Atoi(value)
			if gp_e6 != nil {
				return Config{}, gp_e6
			}
			cfg.Workers = gp_t6
		default:
			return Config{}, fmt.Errorf("unknown key %q", key)
		}
	}
	// sc.Err()
	if gp_e7 := sc.Err(); gp_e7 != nil {
		return Config{}, gp_e7
	}
	if cfg.Port == 0 {
		return Config{}, errors.New("port is missing")
	}
	return cfg, nil
}

// func port!(path string) int: it gets an error result, return p becomes
// return p, nil, and a return with one more value stays as it is.
func port(path string) (int, error) {
	// cfg := loadConfig(path)
	gp_t1, gp_e1 := loadConfig(path)
	if gp_e1 != nil {
		return 0, gp_e1
	}
	cfg := gp_t1
	if cfg.Port < 1024 {
		return 0, fmt.Errorf("port %d is below 1024", cfg.Port)
	}
	return cfg.Port, nil
}

func endpoint(path string) (string, error) {
	// cfg := loadConfig!(path)
	gp_t1, gp_e1 := loadConfig(path)
	if gp_e1 != nil {
		return "", gp_e1
	}
	cfg := gp_t1
	return cfg.Name + ":" + strconv.Itoa(cfg.Port), nil
}

func lookup(path, want string) (string, error) {
	// data := os.ReadFile!(path)
	gp_t1, gp_e1 := os.ReadFile(path)
	if gp_e1 != nil {
		return "", gp_e1
	}
	data := gp_t1
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// key, value := parseLine!(line)
		gp_t2, gp_t3, gp_e2 := parseLine(line)
		if gp_e2 != nil {
			return "", gp_e2
		}
		key, value := gp_t2, gp_t3
		if key == want {
			return value, nil
		}
	}
	return "", fmt.Errorf("%s: no key %q", path, want)
}

func workers(path string) int {
	// return loadConfig!(path).Workers: no error result, so the error panics
	gp_t1, gp_e1 := loadConfig(path)
	if gp_e1 != nil {
		panic(gp_e1)
	}
	return gp_t1.Workers
}

// func main!(): the body moves into gp_main, which returns the first error
// that the code drops. main prints it and exits with 1; main's own deferred
// calls have run by then, because they run when gp_main returns.
func main() {
	if err := gp_main(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", filepath.Base(os.Args[0]), err)
		os.Exit(1)
	}
}

func gp_main() (gp_err error) {
	// defer fmt.Println("main: done")
	defer func() {
		_, gp_e := fmt.Println("main: done")
		gp_deferErr(&gp_err, gp_e)
	}()

	for _, path := range []string{"noport.conf", "badport.conf", "badline.conf", "unknown.conf"} {
		_, err := loadConfig(path)
		// fmt.Println("error:", err): even this dropped error is handled
		if _, gp_e := fmt.Println("error:", err); gp_e != nil {
			return gp_e
		}
	}
	_, err := loadConfig("missing.conf")
	// fmt.Println("missing.conf: not found:", errors.Is(err, os.ErrNotExist))
	if _, gp_e := fmt.Println("missing.conf: not found:", errors.Is(err, os.ErrNotExist)); gp_e != nil {
		return gp_e
	}
	_, err = port("lowport.conf")
	// fmt.Println("error:", err)
	if _, gp_e := fmt.Println("error:", err); gp_e != nil {
		return gp_e
	}

	// cfg := loadConfig("app.conf")
	gp_t1, gp_e1 := loadConfig("app.conf")
	if gp_e1 != nil {
		return gp_e1
	}
	cfg := gp_t1
	// fmt.Printf("config: %+v\n", cfg)
	if _, gp_e := fmt.Printf("config: %+v\n", cfg); gp_e != nil {
		return gp_e
	}
	// fmt.Println("port:", port("app.conf"))
	gp_t2, gp_e2 := port("app.conf")
	if gp_e2 != nil {
		return gp_e2
	}
	if _, gp_e := fmt.Println("port:", gp_t2); gp_e != nil {
		return gp_e
	}
	// fmt.Println("endpoint:", endpoint("app.conf"))
	gp_t3, gp_e3 := endpoint("app.conf")
	if gp_e3 != nil {
		return gp_e3
	}
	if _, gp_e := fmt.Println("endpoint:", gp_t3); gp_e != nil {
		return gp_e
	}
	// fmt.Println("name:", lookup("app.conf", "name"), "workers:", workers("app.conf"))
	gp_t4, gp_e4 := lookup("app.conf", "name")
	if gp_e4 != nil {
		return gp_e4
	}
	if _, gp_e := fmt.Println("name:", gp_t4, "workers:", workers("app.conf")); gp_e != nil {
		return gp_e
	}
	return nil
}
