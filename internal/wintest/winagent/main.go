// Command winagent is the program that internal/wintest copies to the machine that runs the test
// programs of the Windows suite (ADR-103). It is built with GOOS=windows by the test driver and is never
// part of a gocpp release.
package main

import (
	"os"

	"github.com/huuhait/gocpp/internal/wintest/agent"
)

func main() { os.Exit(agent.Main(os.Args[1:], os.Stdout, os.Stderr)) }
