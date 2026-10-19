// phase: 09
// ExitCode: 1
// Stderr: (?m)^[^\n]*: port is missing$

// The must-function spike of the plan (spikes/2026-10-13/must-functions), as a gocpp program (step 10.13): loadConfig!
// reads a file and parses it, every error it drops is returned, and main! prints "<program>: <error>" and exits
// with 1. This run: a config without a port (the explicit error, exit 1). The twin (testdata/must_twin/must_spike_noport.go) is the same program with if err != nil.

package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Name string
	Port int
}

func loadConfig!(path string) Config {
	var cfg Config
	data := os.ReadFile(path)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			cfg.Name = strings.TrimSpace(value)
		case "port":
			cfg.Port = strconv.Atoi(strings.TrimSpace(value))
		}
	}
	if cfg.Port == 0 {
		return Config{}, errors.New("port is missing")
	}
	return cfg
}

func main!() {
	cfg := loadConfig("testdata/spike/noport.conf")
	fmt.Println("listening on", cfg.Port, cfg.Name)
}

// Output:
