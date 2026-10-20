#!/bin/sh
# Prints the names (without .gp, joined with |) of the programs in tests/run that start a goroutine
# or use a channel or a select: the concurrency family that make ci-race runs with -race. A program
# with a data race on purpose says "// race: intentional" in its header and is left out.
cd "$(dirname "$0")/../run" || exit 1
grep -lE '^[[:space:]]*go |go func|make\(chan|select \{|"sync|"time"' ./*.gp | xargs grep -L '^// race: intentional$' | sed 's|^\./||; s|\.gp$||' | sort | paste -sd'|'
