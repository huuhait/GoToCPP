// phase: 09

// Wc counts the lines, words, characters and bytes of files, like the Unix wc.
// With no file names, or with the name "-", it reads standard input.
//
//	wc [-l] [-w] [-m] [-c] [file ...]
//
// Without flags it prints lines, words and bytes.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// Counts are the numbers wc reports for one input.
type Counts struct {
	Lines, Words, Runes, Bytes int
}

func (c *Counts) Add(o Counts) {
	c.Lines += o.Lines
	c.Words += o.Words
	c.Runes += o.Runes
	c.Bytes += o.Bytes
}

// Count reads r to the end. Like wc, it counts a line for every newline, so
// a last line without one adds words and bytes but no line. A word is a run
// of characters that are not white space.
func Count(r io.Reader) (Counts, error) {
	var c Counts
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		c.Bytes += len(line)
		c.Runes += utf8.RuneCountInString(line)
		c.Words += len(strings.Fields(line))
		if strings.HasSuffix(line, "\n") {
			c.Lines++
		}
		if err == io.EOF {
			return c, nil
		}
		if err != nil {
			return c, err
		}
	}
}

// columns says which counts to print.
type columns struct {
	lines, words, runes, bytes bool
}

func (cols columns) format(c Counts, name string) string {
	var b strings.Builder
	for _, col := range []struct {
		on bool
		n  int
	}{{cols.lines, c.Lines}, {cols.words, c.Words}, {cols.runes, c.Runes}, {cols.bytes, c.Bytes}} {
		if col.on {
			fmt.Fprintf(&b, " %7d", col.n)
		}
	}
	if name != "" {
		b.WriteString(" " + name)
	}
	return b.String()
}

func countFile(name string, stdin io.Reader) (Counts, error) {
	if name == "-" {
		return Count(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return Counts{}, err
	}
	defer f.Close()
	return Count(f)
}

// run is the whole program; tests call it with their own arguments and
// streams. It returns the exit status.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("wc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var cols columns
	fs.BoolVar(&cols.lines, "l", false, "print the number of lines")
	fs.BoolVar(&cols.words, "w", false, "print the number of words")
	fs.BoolVar(&cols.runes, "m", false, "print the number of characters")
	fs.BoolVar(&cols.bytes, "c", false, "print the number of bytes")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if cols == (columns{}) {
		cols = columns{lines: true, words: true, bytes: true}
	}
	names := fs.Args()
	if len(names) == 0 {
		c, err := Count(stdin)
		if err != nil {
			fmt.Fprintln(stderr, "wc:", err)
			return 1
		}
		fmt.Fprintln(stdout, cols.format(c, ""))
		return 0
	}
	status := 0
	var total Counts
	for _, name := range names {
		c, err := countFile(name, stdin)
		if err != nil {
			fmt.Fprintln(stderr, "wc:", err)
			status = 1
			continue
		}
		total.Add(c)
		fmt.Fprintln(stdout, cols.format(c, name))
	}
	if len(names) > 1 {
		fmt.Fprintln(stdout, cols.format(total, "total"))
	}
	return status
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
