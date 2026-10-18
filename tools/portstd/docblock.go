package main

import (
	"fmt"
	"os"
	"strings"
)

// The generated table of docs/guides/porting-blockers.md sits between two
// marker lines; the rest of the file is written by hand.
const (
	docBegin = "<!-- portstd:begin: go run ./tools/portstd -list -tier tier1 -md -doc docs/guides/porting-blockers.md -->"
	docEnd   = "<!-- portstd:end -->"

	// defaultDoc is the file whose table the check mode compares, and the
	// list it holds.
	defaultDoc  = "docs/guides/porting-blockers.md"
	defaultTier = "tier1"
)

// replaceDocBlock returns text with the generated block replaced by md.
func replaceDocBlock(text, md string) (string, error) {
	i := strings.Index(text, "<!-- portstd:begin")
	j := strings.Index(text, docEnd)
	if i < 0 || j < i {
		return "", fmt.Errorf("the file has no %q ... %q block", docBegin, docEnd)
	}
	return text[:i] + docBegin + "\n\n" + strings.TrimRight(md, "\n") + "\n\n" + text[j:], nil
}

// updateDoc writes the table into the file.
func updateDoc(file, md string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	out, err := replaceDocBlock(string(b), md)
	if err != nil {
		return fmt.Errorf("%s: %v", file, err)
	}
	return writeFile(file, []byte(out))
}

// checkDoc reports whether the table in the file is what md says. A missing
// file is fine (a checkout without docs).
func checkDoc(file, md string) []string {
	b, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return []string{err.Error()}
	}
	want, err := replaceDocBlock(string(b), md)
	if err != nil {
		return []string{file + ": " + err.Error()}
	}
	if want != string(b) {
		return []string{fmt.Sprintf("%s: the table is out of date (go run ./tools/portstd -list -tier %s -md -doc %s); %s",
			file, defaultTier, file, firstDiff([]byte(want), b))}
	}
	return nil
}
