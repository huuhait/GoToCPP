#!/bin/sh
# Prints the names (without .gp, joined with |) of the programs in tests/run that start a goroutine
# or use a channel or a select: the concurrency family that make ci-race runs with -race.
cd "$(dirname "$0")/../run" || exit 1
grep -lE '^[[:space:]]*go |go func|make\(chan|select \{|"sync|"time"' ./*.gp | sed 's|^\./||; s|\.gp$||' | sort | paste -sd'|'
