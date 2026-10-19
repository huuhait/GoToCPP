// Docsite builds the documentation site: a static site made from the Markdown files of docs/
// and the standard library reference, with no network and no dependency.
//
//	go run ./tools/docsite                 # docs/ into site/ (git-ignored)
//	go run ./tools/docsite -check          # only check the links, write nothing
//	go run ./tools/docsite -out /tmp/site
//
// What it does:
//
//   - every docs/**/*.md becomes a page (README.md becomes index.html of its folder), rendered
//     by markdown.go; the sidebar and the order of the pages come from docs/site/nav.txt;
//   - docs/stdlib is not a folder: the standard library reference is generated from the .gp
//     sources of stdlib/ with internal/doc, one page for each package (stdlib.go);
//   - every link between pages is checked, with its anchor; a link that leaves docs/ must lead
//     to a file of the repository and is written as a link to that file on the web (-repo,
//     -branch); the errors are listed and make the exit status 1;
//   - search is a script over an index of the titles and headings (assets/search.js), so the
//     site works from a folder (file://) and from any static host.
//
// The code blocks of the tour and the guides are checked by the tests of this package
// (snippets.go): they compile with gocpp check, and the tour runs.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	var (
		docs   = flag.String("docs", "docs", "the docs folder")
		out    = flag.String("out", "site", "the folder to write the site into")
		check  = flag.Bool("check", false, "only check the links; write nothing")
		repo   = flag.String("repo", "https://github.com/huuhait/gocpp", "the URL of the repository, for the links to files outside docs/")
		branch = flag.String("branch", "dev", "the branch of the repository that those links name")
	)
	flag.Parse()
	s, err := build(*docs, *repo, *branch, embeddedStdlib())
	if err != nil {
		fmt.Fprintln(os.Stderr, "docsite:", err)
		os.Exit(1)
	}
	if len(s.errs) > 0 {
		sort.Strings(s.errs)
		for _, e := range s.errs {
			fmt.Fprintln(os.Stderr, "docsite:", e)
		}
		fmt.Fprintf(os.Stderr, "docsite: %d errors\n", len(s.errs))
		os.Exit(1)
	}
	if *check {
		fmt.Printf("docsite: %d pages, no broken link\n", len(s.pages))
		return
	}
	if err := s.write(*out); err != nil {
		fmt.Fprintln(os.Stderr, "docsite:", err)
		os.Exit(1)
	}
	fmt.Printf("docsite: %d pages written to %s\n", len(s.pages), *out)
}

// build reads docs, generates the standard library reference and renders and checks everything.
func build(docs, repo, branch string, std fs.FS) (*site, error) {
	abs, err := filepath.Abs(docs)
	if err != nil {
		return nil, err
	}
	s := &site{docs: abs, repo: filepath.Dir(abs), repoURL: strings.TrimRight(repo, "/"), branch: branch}
	if err := s.loadKnown(); err != nil {
		return nil, err
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	if err := s.addStdlib(std); err != nil {
		return nil, err
	}
	if err := s.render(); err != nil {
		return nil, err
	}
	if err := s.loadNav(); err != nil {
		return nil, err
	}
	s.check()
	s.checkKnown()
	return s, nil
}
