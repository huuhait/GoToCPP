package main

import (
	"regexp"
	"strconv"
	"strings"
)

// A Kind is the sanitizer that made a report.
type Kind string

const (
	ASan  Kind = "asan"  // AddressSanitizer: out of bounds, use after free, double free, stack overflow, SEGV
	UBSan Kind = "ubsan" // UndefinedBehaviorSanitizer: "runtime error: ..." at a source position
	LSan  Kind = "lsan"  // LeakSanitizer: detected memory leaks
	TSan  Kind = "tsan"  // ThreadSanitizer, in its own form or in Go's ("WARNING: DATA RACE", gocpp -race)
)

// A Report is one sanitizer report found in a program's output.
type Report struct {
	Kind  Kind   `json:"kind"`
	Title string `json:"title"` // the first line of the report, for example "heap-buffer-overflow on address 0x..."
}

func (r Report) String() string { return string(r.Kind) + ": " + r.Title }

// addrRx matches an address, which differs between runs: reports are compared and deduplicated without it.
var addrRx = regexp.MustCompile(`0x[0-9a-f]+`)

// The first lines of the reports. Each sanitizer starts a report with a line that no program
// of gocpp prints by itself: gocpp's own panics are "panic: runtime error: ..." and have no
// "file:line:col:" in front, which UBSan's lines always have.
var (
	asanRx = regexp.MustCompile(`(?m)^==\d+==ERROR: AddressSanitizer: (.+)$`)
	asanDS = regexp.MustCompile(`(?m)^AddressSanitizer:DEADLYSIGNAL$`)
	// UBSan alone (clang) prints the signal and stack overflow reports of the runtime in its own name.
	ubsanErr = regexp.MustCompile(`(?m)^==\d+==ERROR: UndefinedBehaviorSanitizer: (.+)$`)
	// The catch-alls: a sanitizer that stops itself ("CHECK failed", a fatal signal in the sanitizer) or
	// that prints a SUMMARY line without any of the lines above. They add a report only for a kind that
	// has none yet, so one error is still one report.
	checkFailed = regexp.MustCompile(`(?m)^(?:==\d+==)?(Address|Thread|Leak|UndefinedBehavior)Sanitizer:? (?:CHECK failed|has encountered a fatal error)`)
	deadlySig   = regexp.MustCompile(`(?m)^(Address|Thread|Leak|UndefinedBehavior)Sanitizer:DEADLYSIGNAL$`)
	summaryLine = regexp.MustCompile(`(?m)^SUMMARY: (Address|Thread|Leak|UndefinedBehavior)Sanitizer: (.+)$`)
	lsanRx      = regexp.MustCompile(`(?m)^==\d+==ERROR: LeakSanitizer: (.+)$`)
	ubsanRx     = regexp.MustCompile(`(?m)^\S[^\n]*?:\d+:\d+: runtime error: (.+)$`)
	// Go's form is what gocpp -race prints; the raw form is TSan's (GOCPP_RACE_RAW=1, or a C++
	// library built with it). FATAL: is TSan giving up (for example "unexpected memory mapping"), which
	// is as much a failure of the run as a race.
	tsanGo  = regexp.MustCompile(`(?m)^WARNING: DATA RACE$`)
	tsanRaw = regexp.MustCompile(`(?m)^(?:WARNING|FATAL): ThreadSanitizer: (.+)$`)
	// SUMMARY: AddressSanitizer: 288 byte(s) leaked in 8 allocation(s).
	leakSummary = regexp.MustCompile(`(?m)^SUMMARY: (?:Address|Leak)Sanitizer: (\d+) byte\(s\) leaked in (\d+) allocation\(s\)\.?$`)
)

// Scan returns the sanitizer reports in text, in the order of their kinds (ASan, UBSan, LSan,
// TSan), one for each report line; equal lines are reported once. Text that has none returns nil.
//
// Not reports: LeakSanitizer's "Unable to get registers from thread" warning (a runtime thread
// ends while the leak check stops the threads; the check goes on and the exit status is unchanged),
// and the sanitizers' own banners.
func Scan(text string) []Report {
	var out []Report
	seen := map[Report]bool{}
	add := func(k Kind, title string) {
		r := Report{k, addrRx.ReplaceAllString(strings.TrimSpace(title), "0x...")}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	for _, m := range asanRx.FindAllStringSubmatch(text, -1) {
		add(ASan, m[1])
	}
	if asanDS.MatchString(text) {
		add(ASan, "DEADLYSIGNAL")
	}
	for _, m := range ubsanErr.FindAllStringSubmatch(text, -1) {
		add(UBSan, m[1])
	}
	for _, m := range ubsanRx.FindAllStringSubmatch(text, -1) {
		add(UBSan, m[1])
	}
	for _, m := range lsanRx.FindAllStringSubmatch(text, -1) {
		add(LSan, m[1])
	}
	if tsanGo.MatchString(text) {
		add(TSan, "data race")
	}
	for _, m := range tsanRaw.FindAllStringSubmatch(text, -1) {
		add(TSan, m[1])
	}
	// The catch-alls, for a kind that the lines above did not find.
	have := map[Kind]bool{}
	for _, r := range out {
		have[r.Kind] = true
	}
	catch := func(k Kind, title string) {
		if !have[k] {
			have[k] = true
			add(k, title)
		}
	}
	for _, m := range checkFailed.FindAllStringSubmatch(text, -1) {
		catch(sanitizerKind(m[1]), m[0])
	}
	for _, m := range deadlySig.FindAllStringSubmatch(text, -1) {
		catch(sanitizerKind(m[1]), "DEADLYSIGNAL")
	}
	for _, m := range summaryLine.FindAllStringSubmatch(text, -1) {
		k := sanitizerKind(m[1])
		if strings.Contains(m[2], "byte(s) leaked") {
			k = LSan // LeakSanitizer inside ASan says AddressSanitizer
		}
		catch(k, "SUMMARY: "+m[2])
	}
	return out
}

// A Leak is the size of what LeakSanitizer reports at the end of a run.
type Leak struct {
	Bytes  int `json:"bytes"`
	Allocs int `json:"allocs"`
}

// ParseLeak returns the leak summary in text, or false when LeakSanitizer reported none.
func ParseLeak(text string) (Leak, bool) {
	m := leakSummary.FindStringSubmatch(text)
	if m == nil {
		return Leak{}, false
	}
	b, err1 := strconv.Atoi(m[1])
	a, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		return Leak{}, false
	}
	return Leak{b, a}, true
}

// sanitizerKind maps the first word of a sanitizer's name (Address, Thread, Leak, UndefinedBehavior).
func sanitizerKind(name string) Kind {
	switch name {
	case "Thread":
		return TSan
	case "Leak":
		return LSan
	case "UndefinedBehavior":
		return UBSan
	}
	return ASan
}
