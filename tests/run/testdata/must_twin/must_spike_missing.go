package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Name string
	Port int
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			cfg.Name = strings.TrimSpace(value)
		case "port":
			port, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return Config{}, err
			}
			cfg.Port = port
		}
	}
	if cfg.Port == 0 {
		return Config{}, errors.New("port is missing")
	}
	return cfg, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, filepath.Base(os.Args[0])+": "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig("testdata/spike/missing.conf")
	if err != nil {
		return err
	}
	fmt.Println("listening on", cfg.Port, cfg.Name)
	return nil
}
