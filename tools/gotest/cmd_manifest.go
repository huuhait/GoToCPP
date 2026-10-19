package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
)

func cmdFetch(args []string, stdout, stderr io.Writer) error {
	var c commonFlags
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitError{2, ""}
	}
	pin, err := c.loadPin()
	if err != nil {
		return err
	}
	root, err := c.sourceRoot(pin, stderr)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, root)
	return nil
}

// loadClassifierInputs reads overrides.txt and expect.txt.
func (c *commonFlags) loadClassifierInputs() (overrides, needs map[string]string, err error) {
	t, err := c.readConfig("overrides.txt", true)
	if err != nil {
		return nil, nil, err
	}
	if overrides, err = ParseOverrides(t); err != nil {
		return nil, nil, err
	}
	t, err = c.readConfig("expect.txt", true)
	if err != nil {
		return nil, nil, err
	}
	if needs, err = ParseNeeds(t); err != nil {
		return nil, nil, err
	}
	return overrides, needs, nil
}

func cmdManifest(args []string, stdout, stderr io.Writer) error {
	var c commonFlags
	fs := flag.NewFlagSet("manifest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c.register(fs)
	check := fs.Bool("check", false, "compare with manifest.txt instead of writing it (exit 1 when it differs)")
	verbose := fs.Bool("v", false, "print the reason detail of every skipped test to standard error")
	print := fs.Bool("print", false, "write the manifest to standard output instead of the file")
	if err := fs.Parse(args); err != nil {
		return exitError{2, ""}
	}
	pin, err := c.loadPin()
	if err != nil {
		return err
	}
	root, err := c.sourceRoot(pin, stderr)
	if err != nil {
		return err
	}
	overrides, needs, err := c.loadClassifierInputs()
	if err != nil {
		return err
	}
	ts, err := BuildManifest(root, defaultTarget(pin), overrides, needs)
	if err != nil {
		return err
	}
	if *verbose {
		for _, t := range ts {
			if t.Skipped() {
				fmt.Fprintf(stderr, "%s\t%s\t%s\n", t.Path, t.Status, t.Detail)
			}
		}
	}
	text := FormatManifest(pin, ts)
	if *print {
		_, err := io.WriteString(stdout, text)
		return err
	}
	dir, err := c.configDir()
	if err != nil {
		return err
	}
	file := filepath.Join(dir, "manifest.txt")
	if *check {
		old, err := os.ReadFile(file)
		if err != nil || string(old) != text {
			return exitError{1, "manifest.txt is out of date: run go run ./tools/gotest manifest"}
		}
		fmt.Fprintln(stdout, "manifest.txt is up to date")
		return nil
	}
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		return err
	}
	fmt.Fprint(stdout, summarize(ts))
	return nil
}

// summarize prints the counts of a manifest.
func summarize(ts []Test) string {
	by := map[string]int{}
	app := 0
	dirs := map[string]int{}
	for _, t := range ts {
		if t.Skipped() {
			by[t.Status]++
		} else {
			app++
			dirs[t.Directive]++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d tests, %d applicable\n", len(ts), app)
	tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	for _, k := range sortedKeys(by) {
		fmt.Fprintf(tw, "  %s\t%d\n", k, by[k])
	}
	tw.Flush()
	fmt.Fprintln(&b, "applicable by directive:")
	for _, k := range sortedKeys(dirs) {
		fmt.Fprintf(&b, "  %-22s %d\n", k, dirs[k])
	}
	return b.String()
}

// loadManifest reads manifest.txt of the configuration folder.
func (c *commonFlags) loadManifest() ([]Test, error) {
	text, err := c.readConfig("manifest.txt", false)
	if err != nil {
		return nil, fmt.Errorf("%v: make it with: go run ./tools/gotest manifest", err)
	}
	return ParseManifest(text)
}

func cmdList(args []string, stdout, stderr io.Writer) error {
	var c commonFlags
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c.register(fs)
	filter := fs.String("filter", "", "only paths matching this regexp")
	status := fs.String("status", "", "only this status (applicable, skip, or skip:<code>)")
	directive := fs.String("directive", "", "only this directive")
	if err := fs.Parse(args); err != nil {
		return exitError{2, ""}
	}
	ts, err := c.loadManifest()
	if err != nil {
		return err
	}
	var rx *regexp.Regexp
	if *filter != "" {
		if rx, err = regexp.Compile(*filter); err != nil {
			return usageError("-filter: %v", err)
		}
	}
	for _, t := range ts {
		if rx != nil && !rx.MatchString(t.Path) {
			continue
		}
		if *directive != "" && t.Directive != *directive {
			continue
		}
		if s := *status; s != "" && t.Status != s && !(s == "skip" && t.Skipped()) {
			continue
		}
		n := t.Needs
		if n == "" {
			n = "-"
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", t.Path, t.Directive, t.Status, n)
	}
	return nil
}
