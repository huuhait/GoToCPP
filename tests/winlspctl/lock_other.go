//go:build !windows

package main

import "errors"

func lockFor(string, int) error { return errors.New("lockfor is for Windows") }
