package main

import (
	"fmt"
	"regexp"
	"strings"
)

// A failure cluster is a short key that groups the tests that fail for the
// same reason, so that the report can say "47 tests wait for package
// reflect" instead of listing 47 messages. clusterOf derives the key from
// the stage where the test failed and the output of the failing command;
// clusters.txt maps keys to the plan step that owns the missing feature.

var (
	notAvailableRx = regexp.MustCompile(`package (\S+) is not available yet`)
	notSupportedRx = regexp.MustCompile(`(?:^|: )(.*?) not supported yet \(Phase (\d+)\)`)
	cxxErrorRx     = regexp.MustCompile(`(?m)^\S+:\d+:\d+: (?:error|fatal error): (.*)$`)
	posPrefixRx    = regexp.MustCompile(`^(?:\./)?\S+\.(?:gp|go|cpp|hpp):\d+(?::\d+)?: `)
	diagRx         = regexp.MustCompile(`(?m)^(?:\./)?[^\s:]+\.(?:gp|go):\d+(?::\d+)?: (.*)$`)
	panicRx        = regexp.MustCompile(`(?m)^(?:panic|fatal error): (.*)$`)
	quotedRx       = regexp.MustCompile("\"[^\"\\n]*\"|`[^`\\n]*`|'[^'\\n]*'")
	numRx          = regexp.MustCompile(`\b\d+(?:\.\d+)?\b|0x[0-9a-fA-F]+`)
	spaceRx        = regexp.MustCompile(`\s+`)
	workPathRx     = regexp.MustCompile(`(?:/[\w.\-+]+){2,}`)
)

// normalizeMessage reduces a message to what is the same across tests:
// quoted text, numbers and long paths become placeholders.
func normalizeMessage(s string) string {
	s = quotedRx.ReplaceAllString(s, "Q")
	s = workPathRx.ReplaceAllString(s, "PATH")
	s = numRx.ReplaceAllString(s, "N")
	s = spaceRx.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if len(s) > 90 {
		s = s[:90]
	}
	return s
}

// clusterOf returns the cluster key of a failure.
func clusterOf(stage, detail string) string {
	if m := notAvailableRx.FindStringSubmatch(detail); m != nil {
		return "package not available: " + m[1]
	}
	if m := notSupportedRx.FindStringSubmatch(posPrefixRx.ReplaceAllString(firstLineWith(detail, "not supported yet"), "")); m != nil {
		return fmt.Sprintf("not supported yet (Phase %s): %s", m[2], normalizeMessage(m[1]))
	}
	switch stage {
	case "timeout-build":
		return "timeout: build"
	case "timeout-run":
		return "timeout: run"
	case "timeout-check":
		return "timeout: check"
	case "check", "build":
		if strings.Contains(detail, "internal error: generated C++ did not compile") ||
			strings.Contains(detail, "internal error") {
			if m := cxxErrorRx.FindStringSubmatch(detail); m != nil {
				return "C++ does not compile: " + normalizeMessage(m[1])
			}
			msg := posPrefixRx.ReplaceAllString(firstLineWith(detail, "internal error"), "")
			return normalizeMessage(msg)
		}
		if m := diagRx.FindStringSubmatch(detail); m != nil {
			return "gocpp rejects: " + normalizeMessage(m[1])
		}
		return stage + ": " + normalizeMessage(firstLine(detail))
	case "run":
		if m := panicRx.FindStringSubmatch(detail); m != nil {
			return "run: " + normalizeMessage(m[0])
		}
		return "run: " + normalizeMessage(firstLine(detail))
	case "compare":
		return "output differs"
	case "errorcheck":
		return errorcheckCluster(detail)
	case "go":
		return "real Go failed: " + normalizeMessage(firstLine(detail))
	case "runner":
		return "runner: " + normalizeMessage(firstLine(detail))
	}
	return stage + ": " + normalizeMessage(firstLine(detail))
}

// errorcheckCluster classifies a text mismatch of an errorcheck test: the
// detail starts with the kind of the first mismatch (see failErrorcheck).
func errorcheckCluster(detail string) string {
	kind, rest, _ := strings.Cut(detail, "\n")
	switch kind {
	case "succeeded":
		return "errorcheck: gocpp accepts what Go rejects"
	case "failed":
		return "errorcheck: gocpp rejects a valid program: " + normalizeMessage(firstDiag(rest))
	case "missing":
		return "errorcheck: error not reported"
	case "nomatch":
		// the lines of Mismatch.String: "f.go:3: no match for `re` in:" then the messages
		// of that line, tab-indented
		if _, got, ok := strings.Cut(rest, "\n\t"); ok {
			if msg := posPrefixRx.ReplaceAllString(firstLine(got), ""); msg != "" {
				return "errorcheck: message text differs: got " + normalizeMessage(msg)
			}
		}
		return "errorcheck: message text differs"
	case "unmatched":
		if msg, ok := strings.CutPrefix(firstLine(rest), "unmatched error: "); ok {
			if msg = posPrefixRx.ReplaceAllString(msg, ""); msg != "" {
				return "errorcheck: extra error: " + normalizeMessage(msg)
			}
		}
		return "errorcheck: extra error"
	}
	return "errorcheck: " + normalizeMessage(kind)
}

func firstDiag(s string) string {
	if m := diagRx.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return firstLine(s)
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			return l
		}
	}
	return ""
}

func firstLineWith(s, sub string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	return ""
}
