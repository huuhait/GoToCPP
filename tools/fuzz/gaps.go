package fuzz

import (
	"regexp"
	"strings"
)

// knownGaps are the messages of the emitter for a checked program that are known and listed
// (TODO.md or the phase STATE.md): the fuzzers skip them, so that a campaign finds new ones.
//
// Every entry says where it is listed. A gap is a message of the emitter for a program that the
// checker accepts; the right fix is a checker error or the feature.
var knownGaps = []string{
	// found by the 15.8 campaign: the checker accepts "func f()" without a body (Go's compiler says
	// "missing function body"; bodiless declarations are for //gocpp:native and linkname), and the
	// emitter says it is not supported. Reported in the 15.8 handoff.
	"not supported yet: a function without a body",
	// A conversion between a string and a slice of a named byte type (type MB byte; []MB(s),
	// string([]MB{1})): lower.sliceConversion knows only []byte and []rune. Found by the 15.8
	// round 2 generator (Options.Gaps turns these constructs on); in the 15.8 report.
	"not supported yet: the conversion from string to []MyByte",
	"not supported yet: the conversion from []MyByte",
}

// unportedCallRx matches the emitter's refusal of a call into a standard package whose functions
// are not lowered yet (Phase 9: tools/gotest/clusters.txt gives these packages to the steps 8.7,
// 8.8, 9.3 to 9.9; stdlib/PORTED.txt lists what exists). It names the packages: a refusal of a
// call into another package (unsafe, which clusters.txt lists as a gocpp bug, a package of
// the program) is a finding.
var unportedCallRx = regexp.MustCompile(`not supported yet: the call of (fmt|strings|bytes|strconv|utf8|utf16|unicode|os|io|bufio|errors|sort|math|bits|rand|atomic|sync|time|context|regexp|reflect|slices|maps|cmp|iter|flag|testing|path|filepath|runtime|log|syscall|exec|signal|big|binary|json|hex|base64|fs|ioutil|heap|list|ring|tabwriter|template|html|url|http|net|sha256|sha1|md5|crc32|fnv|gzip|zlib|bzip2|flate|tar|zip|csv|xml|gob|pem|x509|tls|rsa|ecdsa|aes|cipher|hmac|subtle|debug|pprof|trace|race|unique|weak|synctest|embed)\.[A-Za-z_]`)

// knownGap returns the known gap that msg is, or "".
func knownGap(msg string) string {
	for _, g := range knownGaps {
		if strings.Contains(msg, g) {
			return g
		}
	}
	if m := unportedCallRx.FindString(msg); m != "" {
		return m
	}
	return ""
}

// knownCheckGap returns the known gap of the checker that one of the errors is, or "".
func knownCheckGap(errs []string) string {
	for _, e := range errs {
		if g := knownGap(e); g != "" {
			return g
		}
	}
	return ""
}

// knownInternal are the "internal error" diagnostics that are known upstream bugs, not gocpp's:
// go/types of Go 1.25 panics on "type T T.x.y" (a named type declared from a selector of itself),
// and gocpp reports it as an error instead of crashing (internal/check, safeTypesCheck). Found by
// the 15.8 fuzzer; the report to the Go project is the manager's.
var knownInternal = []string{
	"the type checker failed on this package: nil underlying",
}

// newInternal returns the internal errors of msgs that are not known.
func newInternal(msgs []string) []string {
	var out []string
	for _, m := range msgs {
		known := false
		for _, k := range knownInternal {
			known = known || strings.Contains(m, k)
		}
		if !known {
			out = append(out, m)
		}
	}
	return out
}
