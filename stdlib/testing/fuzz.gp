// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import "fmt"

// Fuzzing, as far as `go test` does it without -fuzz: a fuzz target runs its seed corpus, the
// entries of F.Add and the files of testdata/fuzz/<FuzzName>, as subtests named FuzzName/seed#N and
// FuzzName/<file>. The fuzzing engine (-test.fuzz, generating and minimizing inputs) is not part
// of gocpp (PORT.md).
//
// Go calls the function that F.Fuzz receives with reflect (Value.Call), which gocpp's reflect has
// not got. Instead, gocpp test rewrites the call f.Fuzz(func(t *testing.T, a int, b string) {...})
// in the test files (internal/gotest, Rewrite) to
//
//	f.GocppFuzz(func(t *testing.T, vals []any) { (func(t *testing.T, a int, b string) {...})(t, vals[0].(int), vals[1].(string)) },
//		[]any{*new(int), *new(string)})
//
// where the second argument holds a zero value of each parameter type, for the checks that Go's F.Fuzz
// makes against the types of the corpus. A call of f.Fuzz that gocpp test cannot rewrite (the argument is
// not a function literal or a function of the package, or f is passed to another function) reaches
// F.Fuzz, which skips the target and says so.

// InternalFuzzTarget is an internal type but exported because it is cross-package;
// it is part of the implementation of the "gocpp test" command.
type InternalFuzzTarget struct {
	Name string
	Fn   func(f *F)
}

// F is a type passed to fuzz tests.
//
// A fuzz test runs the seed corpus by default, which includes entries provided
// by [F.Add] and entries in the testdata/fuzz/<FuzzTestName> directory. After
// any necessary setup and calls to [F.Add], the fuzz test must then call
// [F.Fuzz] to provide the fuzz target.
//
// *F methods can only be called before [F.Fuzz]. Once the test is
// executing the fuzz target, only [*T] methods can be used. The only *F methods
// that are allowed in the [F.Fuzz] function are [F.Failed] and [F.Name].
type F struct {
	common
	tstate *testState

	// inFuzzFn is true when the fuzz function is running. Most F methods cannot
	// be called when inFuzzFn is true.
	inFuzzFn bool

	// corpus is a set of seed corpus entries, added with F.Add and loaded
	// from testdata.
	corpus []corpusEntry

	fuzzCalled bool
}

var _ TB = (*F)(nil)

// corpusEntry is one input of the seed corpus.
type corpusEntry struct {
	Path   string // "seed#N" for F.Add, else the file in testdata/fuzz/<FuzzName>
	Values []any
}

// corpusDir is the parent directory of the fuzz test's seed corpus within the package.
const corpusDir = "testdata/fuzz"

// Helper marks the calling function as a test helper function.
// When printing file and line information, that function will be skipped.
func (f *F) Helper() {
	if f.inFuzzFn {
		panic("testing: f.Helper was called inside the fuzz target, use t.Helper instead")
	}
	f.common.Helper()
}

// Fail marks the function as having failed but continues execution.
func (f *F) Fail() {
	// (*F).Fail may be called by tRunner... we shouldn't allow direct (*F).Fail calls from inside
	// the (*F).Fuzz function.
	if f.inFuzzFn {
		panic("testing: f.Fail was called inside the fuzz target, use t.Fail instead")
	}
	f.common.Helper()
	f.common.Fail()
}

// Skipped reports whether the test was skipped.
func (f *F) Skipped() bool {
	if f.inFuzzFn {
		panic("testing: f.Skipped was called inside the fuzz target, use t.Skipped instead")
	}
	f.common.Helper()
	return f.common.Skipped()
}

// Add will add the arguments to the seed corpus for the fuzz test. This will be
// a no-op if called after or within the fuzz target, and args must match the
// arguments for the fuzz target.
func (f *F) Add(args ...any) {
	var values []any
	for i := range args {
		if tn := typeName(args[i]); !supportedType(tn) {
			panic(fmt.Sprintf("testing: unsupported type to Add %v", tn))
		}
		values = append(values, args[i])
	}
	f.corpus = append(f.corpus, corpusEntry{Values: values, Path: fmt.Sprintf("seed#%d", len(f.corpus))})
}

// typeName is the name of the type of v, as reflect prints it.
func typeName(v any) string {
	return fmt.Sprintf("%T", v)
}

// supportedType reports whether a fuzz target may have a parameter of the type (by its name).
func supportedType(name string) bool {
	switch name {
	case "[]uint8", "string", "bool", "uint8", "int32", "float32", "float64", "int", "int8", "int16",
		"int64", "uint", "uint16", "uint32", "uint64":
		return true
	}
	return false
}

// Fuzz is the call that gocpp test could not rewrite (GocppFuzz is the one it rewrites to): it
// skips the fuzz target.
func (f *F) Fuzz(ff any) {
	if f.fuzzCalled {
		panic("testing: F.Fuzz called more than once")
	}
	f.fuzzCalled = true
	if f.failed {
		return
	}
	f.Helper()
	f.Skip("gocpp test cannot run this fuzz target: the argument of F.Fuzz must be a function literal or a function of the package, and f must be used in the fuzz test function itself")
}

// GocppFuzz runs the seed corpus with the fuzz function that gocpp test made of the argument of
// F.Fuzz (see the top of this file): call runs it for the values of one entry, and zeros holds a
// zero value for each parameter after the *T.
func (f *F) GocppFuzz(call func(t *T, vals []any), zeros []any) {
	if f.fuzzCalled {
		panic("testing: F.Fuzz called more than once")
	}
	f.fuzzCalled = true
	if f.failed {
		return
	}
	f.Helper()

	// Save the types of the function to compare against the corpus.
	var types []string
	for _, z := range zeros {
		tn := typeName(z)
		if !supportedType(tn) {
			panic(fmt.Sprintf("testing: unsupported type for fuzzing %v", tn))
		}
		types = append(types, tn)
	}

	// Check types of entries declared with F.Add, and load the testdata seed corpus.
	for _, c := range f.corpus {
		if err := checkCorpus(c.Values, types); err != "" {
			f.Fatal(err)
		}
	}
	c, err := readCorpus(corpusDir+"/"+f.name, types)
	if err != "" {
		f.Fatal(err)
	}
	f.corpus = append(f.corpus, c...)

	// run calls the fuzz function on a given input, as a subtest with its own T.
	run := func(e corpusEntry) {
		testName := f.name
		if e.Path != "" {
			testName = f.name + "/" + baseName(e.Path)
		}
		t := &T{
			common: common{
				barrier: make(chan bool),
				signal:  make(chan bool, 1),
				name:    testName,
				parent:  &f.common,
				level:   f.level + 1,
				chatty:  f.chatty,
			},
			tstate: f.tstate,
		}
		t.w = indenter{&t.common}
		if t.chatty != nil {
			t.chatty.Updatef(t.name, "=== RUN   %s\n", t.name)
		}
		libRunningAdd(t.name, libNow())
		f.inFuzzFn = true
		go tRunner(t, func(t *T) {
			call(t, e.Values)
		})
		<-t.signal
		f.inFuzzFn = false
	}

	// Fuzzing is not enabled: only run the seed corpus.
	for _, e := range f.corpus {
		name := f.name + "/" + baseName(e.Path)
		if _, ok, _ := f.tstate.match.fullName(nil, name); ok {
			run(e)
		}
	}
}

// baseName is the last element of a path.
func baseName(p string) string {
	if i := lastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// listOf prints a list of type names as fmt prints a []reflect.Type.
func listOf(names []string) string {
	s := "["
	for i, n := range names {
		if i > 0 {
			s += " "
		}
		s += n
	}
	return s + "]"
}

// checkCorpus checks that the values of an entry have the types of the fuzz function.
func checkCorpus(vals []any, types []string) string {
	if len(vals) != len(types) {
		return fmt.Sprintf("wrong number of values in corpus entry: %d, want %d", len(vals), len(types))
	}
	valsT := make([]string, len(vals))
	for i, v := range vals {
		valsT[i] = typeName(v)
	}
	for i := range types {
		if valsT[i] != types[i] {
			return fmt.Sprintf("mismatched types in corpus entry: %v, want %v", listOf(valsT), listOf(types))
		}
	}
	return ""
}

// readCorpus reads the files of the folder dir, a corpus of entries that have the types.
func readCorpus(dir string, types []string) ([]corpusEntry, string) {
	names, errText := libReadFiles(dir)
	if errText != "" {
		return nil, "reading seed corpus from testdata: " + errText
	}
	var corpus []corpusEntry
	for _, name := range splitByte(names, '\n') {
		if name == "" {
			continue
		}
		filename := dir + "/" + name
		data, errText := libReadFile(filename)
		if errText != "" {
			return nil, "failed to read corpus file: " + errText
		}
		vals, msg := readCorpusData(data, types)
		if msg != "" {
			return nil, fmt.Sprintf("failed to read corpus file %s: %s", name, msg)
		}
		corpus = append(corpus, corpusEntry{Path: filename, Values: vals})
	}
	return corpus, ""
}

// encVersion1 is the first line of a corpus file.
const encVersion1 = "go test fuzz v1"

// readCorpusData parses the content of a corpus file and checks the types of its values.
func readCorpusData(data string, types []string) ([]any, string) {
	vals, msg := unmarshalCorpusFile(data)
	if msg != "" {
		return nil, msg
	}
	if len(vals) != len(types) {
		return nil, fmt.Sprintf("wrong number of values in corpus file: %d, want %d", len(vals), len(types))
	}
	valsT := make([]string, len(vals))
	for i, v := range vals {
		valsT[i] = typeName(v)
	}
	for i := range types {
		if valsT[i] != types[i] {
			return nil, fmt.Sprintf("mismatched types in corpus file: %v, want %v", listOf(valsT), listOf(types))
		}
	}
	return vals, ""
}

// unmarshalCorpusFile parses a corpus file: the version line and a value on each line.
func unmarshalCorpusFile(data string) ([]any, string) {
	lines := splitByte(data, '\n')
	if len(lines) < 2 {
		return nil, "must include version and at least one value"
	}
	version := lines[0]
	if hasSuffix(version, "\r") {
		version = version[:len(version)-1]
	}
	if version != encVersion1 {
		return nil, fmt.Sprintf("unknown encoding version: %s", version)
	}
	var vals []any
	for _, line := range lines[1:] {
		line = trimSpace(line)
		if line == "" {
			continue
		}
		v, msg := parseCorpusValue(line)
		if msg != "" {
			return nil, fmt.Sprintf("malformed line %q: %s", line, msg)
		}
		vals = append(vals, v)
	}
	return vals, ""
}

// parseCorpusValue parses one line of a corpus file: a conversion T(literal).
func parseCorpusValue(line string) (any, string) {
	open := indexByte(line, '(')
	if open < 0 || !hasSuffix(line, ")") {
		return nil, "expected call expression"
	}
	typ := trimSpace(line[:open])
	arg := trimSpace(line[open+1 : len(line)-1])
	if arg == "" {
		return nil, "expected exactly one argument"
	}
	switch typ {
	case "string":
		s, ok := unquoteString(arg)
		if !ok {
			return nil, "string literal is not valid"
		}
		return s, ""
	case "[]byte":
		s, ok := unquoteString(arg)
		if !ok {
			return nil, "string literal is not valid"
		}
		return []byte(s), ""
	case "bool":
		if arg == "true" {
			return true, ""
		}
		if arg == "false" {
			return false, ""
		}
		return nil, "string literal is not valid"
	case "float32", "float64":
		f, ok := libParseFloat(arg)
		if !ok {
			return nil, fmt.Sprintf("invalid floating-point literal %s", arg)
		}
		if typ == "float32" {
			return float32(f), ""
		}
		return f, ""
	case "int", "int8", "int16", "int32", "int64", "rune", "uint", "uint8", "uint16", "uint32", "uint64", "byte":
		bits, signed := 64, true
		switch typ {
		case "int8":
			bits = 8
		case "int16":
			bits = 16
		case "int32", "rune":
			bits = 32
		case "uint":
			signed = false
		case "uint8", "byte":
			bits, signed = 8, false
		case "uint16":
			bits, signed = 16, false
		case "uint32":
			bits, signed = 32, false
		case "uint64":
			signed = false
		}
		var v int64
		var u uint64
		var ok bool
		if arg[0] == '\'' {
			var r rune
			r, ok = unquoteChar(arg)
			v, u = int64(r), uint64(r)
		} else {
			v, u, ok = parseInteger(arg, signed)
		}
		if !ok {
			return nil, fmt.Sprintf("invalid integer literal %s", arg)
		}
		if signed {
			if bits < 64 && (v < -(int64(1)<<(bits-1)) || v >= int64(1)<<(bits-1)) {
				return nil, fmt.Sprintf("integer literal %s overflows %s", arg, typ)
			}
			switch typ {
			case "int":
				return int(v), ""
			case "int8":
				return int8(v), ""
			case "int16":
				return int16(v), ""
			case "int32", "rune":
				return int32(v), ""
			}
			return v, ""
		}
		if bits < 64 && u >= uint64(1)<<bits {
			return nil, fmt.Sprintf("integer literal %s overflows %s", arg, typ)
		}
		switch typ {
		case "uint":
			return uint(u), ""
		case "uint8", "byte":
			return uint8(u), ""
		case "uint16":
			return uint16(u), ""
		case "uint32":
			return uint32(u), ""
		}
		return u, ""
	}
	return nil, fmt.Sprintf("expected []byte or primitive type, got %s", typ)
}

// parseInteger parses a Go integer literal with an optional sign: v for the signed types, u for
// the unsigned ones.
func parseInteger(s string, signed bool) (v int64, u uint64, ok bool) {
	neg := false
	if s != "" && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s = s[1:]
	}
	base := uint64(10)
	switch {
	case hasPrefix(s, "0x") || hasPrefix(s, "0X"):
		base, s = 16, s[2:]
	case hasPrefix(s, "0b") || hasPrefix(s, "0B"):
		base, s = 2, s[2:]
	case hasPrefix(s, "0o") || hasPrefix(s, "0O"):
		base, s = 8, s[2:]
	case len(s) > 1 && s[0] == '0':
		base, s = 8, s[1:]
	}
	if s == "" {
		return 0, 0, false
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		var d uint64
		switch {
		case c == '_':
			continue
		case '0' <= c && c <= '9':
			d = uint64(c - '0')
		case 'a' <= c && c <= 'f':
			d = uint64(c-'a') + 10
		case 'A' <= c && c <= 'F':
			d = uint64(c-'A') + 10
		default:
			return 0, 0, false
		}
		if d >= base || n > (^uint64(0)-d)/base {
			return 0, 0, false
		}
		n = n*base + d
	}
	if neg {
		if !signed || n > uint64(1)<<63 {
			return 0, 0, false
		}
		return -int64(n), 0, true
	}
	if signed {
		if n >= uint64(1)<<63 {
			return 0, 0, false
		}
		return int64(n), 0, true
	}
	return int64(n), n, true
}

// unquoteString returns the value of a Go string literal, "..." or `...`.
func unquoteString(lit string) (string, bool) {
	n := len(lit)
	if n < 2 {
		return "", false
	}
	switch lit[0] {
	case '`':
		if lit[n-1] != '`' {
			return "", false
		}
		body := lit[1 : n-1]
		if indexByte(body, '`') >= 0 {
			return "", false
		}
		var out []byte
		for i := 0; i < len(body); i++ {
			if body[i] != '\r' {
				out = append(out, body[i])
			}
		}
		return string(out), true
	case '"':
		if lit[n-1] != '"' {
			return "", false
		}
		var out []byte
		body := lit[1 : n-1]
		for i := 0; i < len(body); {
			c := body[i]
			if c == '"' || c == '\n' {
				return "", false
			}
			if c != '\\' {
				out = append(out, c)
				i++
				continue
			}
			r, isByte, size, ok := unescape(body[i:])
			if !ok {
				return "", false
			}
			i += size
			if isByte {
				out = append(out, byte(r))
			} else {
				out = append(out, string(r)...)
			}
		}
		return string(out), true
	}
	return "", false
}

// unquoteChar returns the value of a Go character literal.
func unquoteChar(lit string) (rune, bool) {
	n := len(lit)
	if n < 3 || lit[0] != '\'' || lit[n-1] != '\'' {
		return 0, false
	}
	body := lit[1 : n-1]
	if body[0] == '\\' {
		r, _, size, ok := unescape(body)
		return r, ok && size == len(body)
	}
	count := 0
	var first rune
	for _, r := range body {
		if count == 0 {
			first = r
		}
		count++
	}
	return first, count == 1
}

// unescape reads the escape sequence at the start of s (which starts with a backslash). isByte
// says that the value is a byte (\x and octal), not a character.
func unescape(s string) (r rune, isByte bool, size int, ok bool) {
	if len(s) < 2 {
		return
	}
	switch s[1] {
	case 'a':
		return 7, false, 2, true
	case 'b':
		return 8, false, 2, true
	case 'f':
		return 12, false, 2, true
	case 'n':
		return 10, false, 2, true
	case 'r':
		return 13, false, 2, true
	case 't':
		return 9, false, 2, true
	case 'v':
		return 11, false, 2, true
	case '\\', '\'', '"':
		return rune(s[1]), false, 2, true
	case 'x', 'u', 'U':
		digits := 2
		if s[1] == 'u' {
			digits = 4
		} else if s[1] == 'U' {
			digits = 8
		}
		if len(s) < 2+digits {
			return
		}
		var v rune
		for i := 2; i < 2+digits; i++ {
			c := s[i]
			var d rune
			switch {
			case '0' <= c && c <= '9':
				d = rune(c - '0')
			case 'a' <= c && c <= 'f':
				d = rune(c-'a') + 10
			case 'A' <= c && c <= 'F':
				d = rune(c-'A') + 10
			default:
				return
			}
			v = v<<4 | d
		}
		if s[1] != 'x' && (v > 0x10FFFF || 0xD800 <= v && v < 0xE000) {
			return
		}
		return v, s[1] == 'x', 2 + digits, true
	case '0', '1', '2', '3', '4', '5', '6', '7':
		if len(s) < 4 {
			return
		}
		var v rune
		for i := 1; i < 4; i++ {
			c := s[i]
			if c < '0' || c > '7' {
				return
			}
			v = v<<3 | rune(c-'0')
		}
		if v > 255 {
			return
		}
		return v, true, 4, true
	}
	return
}

// runFuzzTests runs the fuzz tests matching the pattern for -run. This will
// only run the (*F).Fuzz function for each seed corpus without using the
// fuzzing engine to generate or mutate inputs.
func runFuzzTests(deps testDeps, fuzzTests []InternalFuzzTarget, deadline int64) (ran, ok bool) {
	ok = true
	if len(fuzzTests) == 0 {
		return ran, ok
	}
	for range cpuList {
		for i := 0; i < testCount; i++ {
			if shouldFailFast() {
				break
			}

			tstate := newTestState(parallelism, newMatcher(deps.MatchString, matchPattern, "-test.run", skipPattern))
			tstate.deadline = deadline
			root := common{w: stdoutWriter{}} // gather output in one place
			if Verbose() {
				root.chatty = chatty
			}
			for _, ft := range fuzzTests {
				if shouldFailFast() {
					break
				}
				testName, matched, _ := tstate.match.fullName(nil, ft.Name)
				if !matched {
					continue
				}
				f := &F{
					common: common{
						signal:  make(chan bool, 1),
						barrier: make(chan bool),
						name:    testName,
						parent:  &root,
						level:   root.level + 1,
						chatty:  root.chatty,
					},
					tstate: tstate,
				}
				f.w = indenter{&f.common}
				if f.chatty != nil {
					f.chatty.Updatef(f.name, "=== RUN   %s\n", f.name)
				}
				libRunningAdd(f.name, libNow())
				go fRunner(f, ft.Fn)
				<-f.signal
				ok = ok && !f.Failed()
				ran = ran || f.ran
			}
			if !ran {
				// There were no tests to run on this iteration.
				// This won't change, so no reason to keep trying.
				break
			}
		}
	}
	return ran, ok
}

// fRunner wraps a call to a fuzz test and ensures that cleanup functions are
// called and status flags are set. fRunner should be called in its own
// goroutine. To wait for its completion, receive from f.signal.
//
// fRunner is analogous to tRunner, which wraps subtests started with T.Run.
// Unit tests and fuzz tests work a little differently, so for now, these
// functions aren't consolidated. In particular, because there are no F.Run and
// F.Parallel methods, i.e., no fuzz sub-tests or parallel fuzz tests, a few
// simplifications are made. We also require that F.Fuzz, F.Skip, or F.Fail is
// called.
func fRunner(f *F, fn func(*F)) {
	// When this goroutine is done, either because a panic started, or fn returned normally or
	// called runtime.Goexit, record the duration and send f.signal, indicating the fuzz test is done.
	defer func() {
		if f.Failed() {
			libLock()
			numFailed++
			libUnlock()
		}

		// Detect whether the fuzz test panicked or called runtime.Goexit
		// without calling F.Fuzz, F.Fail, or F.Skip. If it did, panic (possibly
		// replacing a nil panic value). Nothing should recover after fRunner
		// unwinds, so this should crash the process and print stack.
		err := recover()
		if err == nil {
			libLock()
			fuzzNotCalled := !f.fuzzCalled && !f.skipped && !f.failed
			if !f.finished && !f.skipped && !f.failed {
				err = errNilPanicOrGoexit
			}
			libUnlock()
			if fuzzNotCalled && err == nil {
				f.Error("returned without calling F.Fuzz, F.Fail, or F.Skip")
			}
		}

		// Use a deferred call to ensure that we report that the test is
		// complete even if a cleanup function calls F.FailNow. See issue 41355.
		didPanic := false
		defer func() {
			if !didPanic {
				// Only report that the test is complete if it doesn't panic,
				// as otherwise the test binary can exit before the panic is
				// reported to the user. See issue 41479.
				libRunningDelete(f.name)
				f.signal <- true
			}
		}()

		// If we recovered a panic or inappropriate runtime.Goexit, fail the test,
		// flush the output log up to the root, then panic.
		doPanic := func(err any) {
			f.Fail()
			if r := f.runCleanup(recoverAndReturnPanic); r != nil {
				f.Logf("cleanup panicked with %v", r)
			}
			for root := &f.common; root.parent != nil; root = root.parent {
				libLock()
				root.duration += libNow() - root.start
				d := root.duration
				libUnlock()
				root.flushToParent(root.name, "--- FAIL: %s (%s)\n", root.name, fmtDuration(d))
			}
			didPanic = true
			panic(err)
		}
		if err != nil {
			doPanic(err)
		}

		// No panic or inappropriate Goexit.
		f.duration += libNow() - f.start

		if len(f.sub) > 0 {
			// Unblock inputs that called T.Parallel while running the seed corpus.
			f.tstate.release()
			libRunningDelete(f.name)
			f.tstate.releaseParallel(f.sub)
			// Wait for the subtests to complete.
			for _, sub := range f.sub {
				<-sub.signal
			}
			cleanupStart := libNow()
			libRunningAdd(f.name, cleanupStart)
			err := f.runCleanup(recoverAndReturnPanic)
			f.duration += libNow() - cleanupStart
			if err != nil {
				doPanic(err)
			}
		}

		// Report after all subtests have finished.
		f.report()
		libLock()
		f.done = true
		f.setRan()
		f.w = nil // let go of the cycle of the test with its writer (memory is reference counted)
		f.sub = nil
		libUnlock()
	}()
	defer func() {
		if len(f.sub) == 0 {
			f.runCleanup(normalPanic)
		}
	}()

	f.start = libNow()
	fn(f)

	// Code beyond this point will not be executed when FailNow or SkipNow
	// is invoked.
	libLock()
	f.finished = true
	libUnlock()
}

func (f *F) report() {
	if f.parent == nil {
		return
	}
	dstr := fmtDuration(f.duration)
	format := "--- %s: %s (%s)\n"
	if f.Failed() {
		f.flushToParent(f.name, format, "FAIL", f.name, dstr)
	} else if f.chatty != nil {
		if f.Skipped() {
			f.flushToParent(f.name, format, "SKIP", f.name, dstr)
		} else {
			f.flushToParent(f.name, format, "PASS", f.name, dstr)
		}
	}
}
