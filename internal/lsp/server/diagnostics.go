package server

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// Diagnostics as you type (Phase 11 README §2.2, step 11.4), like gopls's
// textDocument/publishDiagnostics.
//
// A package is diagnosed while one of its files is open: every change that
// can change its diagnostics schedules a run for its folder, debounced per
// folder (a run starts when no newer change came for diagnoseDelay after a
// didChange, at once for didOpen, didClose, didSave and the other causes).
// A run takes the current snapshot, gets the checked package from the
// pipeline (Snapshot.Package: only what changed is checked again), and
// publishes the diagnostics of every file of the package: the open ones
// with their document version, the others without one. A file whose
// diagnostics and version are what the client has already is not
// published again. When the last open file of a package closes, the
// diagnostics of all its files are cleared (an empty publish); a file that
// closes while another file of its package stays open keeps its
// diagnostics, as a file of an open package (workspace diagnostics of the
// packages that are not open come later, README §2.2 item 6).
//
// The importers: a run compares the API hash of the package with the one of
// its last run, and when it changed (or there was none), it schedules a run
// for every other open package; their checks see the new API (an importer is
// checked again only when the API of an import changed, step 11.3). A
// body edit changes no API, so the importers are not even looked at, and
// nothing is published for them.
//
// Never stale: the runs of one folder never overlap and start in order
// (debounce.Debouncer), and a newer change cancels the run that is under
// way (its result is dropped). A run that finds the session at a newer
// snapshot than the one it checked checks again on the newest one (a few
// times at most; the changes that matter to the package schedule a newer
// run anyway), and a file's diagnostics are never replaced by those of an
// older snapshot. Nothing is published after the answer to shutdown (the
// debouncer stops first, and waits for the runs) or after exit
// (protocol.Conn.Notify fails then).
//
// The C++ namespace probe never holds a publish back (README §2.2 item 8):
// a check that needs an answer the pipeline doesn't have yet (a new
// include set, a new name, or the compiler still being looked for) gets
// the names it doesn't know as opaque, without errors, and the probe runs
// in the background (cache/probe.go). When it answers, the pipeline calls
// probed, which checks the open packages whose checks are stale again, in
// a new snapshot (Session.Recheck of their folders), and publishes
// again: then the "undefined: stdd" of a typo and the include errors are
// there. An answer that comes between a check and its publish is seen by
// the publish (the package is stale already), which rechecks it the same
// way.
//
// The diagnostics are those of gocpp check for the package (all of them,
// as with -e; the known difference of the 11.3 pipeline aside: an error of
// an import as a whole is at every importing spec): the load errors (syntax
// errors among them) and the check errors with the warnings, the namespace
// probe's errors and the phase gates. Unlike gocpp check's printout, which
// shows only the warnings of a package whose import is broken and only the
// first syntax error per line, the editor gets every diagnostic of the
// package, like gopls. An overlay marked broken (a didChange that failed)
// gets an empty publish: its diagnostics would be at the wrong places. A
// diagnostic without a place (about the package as a whole) goes to the
// package clause of every file of the package; one in a file of the folder
// that the package leaves out (another package name) goes to that file.
//
// The ranges are in UTF-16 (protocol.Mapper), converted from byte offsets,
// never from token.Position.Column. A diagnostic with an end (go/types has
// one for some errors, the gates and the probe set theirs) spans to it;
// one without spans the token at its start, as gopls does
// (analysisinternal.TypeErrorEndPos), but only that token on that line
// (not up to the next operator, which gopls's heuristic adds). Related
// places (the go/types continuation lines, such as "other declaration of
// x") are relatedInformation when the file can be read, and else lines
// of the message.

// diagnoseDelay is the quiet time after a didChange before the diagnostics
// of its package run (README §2.2 item 7: "about 30 ms"). With a 10k-line
// package checked again in about 30 ms, a keystroke gets new diagnostics
// in about 60 ms, within the budget of 100 ms (BenchmarkKeystroke).
const diagnoseDelay = 30 * time.Millisecond

// maxRetries is how often a run checks again when the snapshot changed
// while it checked.
const maxRetries = 3

// diagSource is the source of the diagnostics.
const diagSource = "gocpp"

// diagnostics is the diagnostics loop of a server.
type diagnostics struct {
	s     *Server
	deb   *debounce.Debouncer[string] // by package folder
	delay time.Duration               // after a didChange

	// related: the client takes relatedInformation (initialize); without
	// it, the related places are lines of the message.
	related atomic.Bool

	// checked, if not nil, runs when a run has the package it checked on
	// the snapshot snapID, in its try (0, and more after a newer snapshot
	// came), before it looks at the current snapshot (for the tests).
	checked func(ctx context.Context, dir string, snapID uint64, try int)

	mu    sync.Mutex
	files map[protocol.DocumentURI]*published // what the client has
	pkgs  map[string]*pkgState                // by folder
}

// published is the last publish of a file.
type published struct {
	snapshot uint64
	version  int32
	text     string // the diagnostics as JSON
}

// pkgState is what the last run of a package found.
type pkgState struct {
	api   cache.Hash
	known bool           // api is set
	pkg   *cache.Package // the package while it is open (for Stale)
	files map[protocol.DocumentURI]bool
}

func newDiagnostics(s *Server, clock debounce.Clock) *diagnostics {
	return &diagnostics{
		s: s, deb: debounce.New[string](clock), delay: diagnoseDelay,
		files: make(map[protocol.DocumentURI]*published), pkgs: make(map[string]*pkgState),
	}
}

// stop cancels the runs and waits (a little) for them to end.
func (d *diagnostics) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	d.deb.Stop(ctx)
}

// schedule schedules a run for the package of the folder dir after delay.
func (d *diagnostics) schedule(dir string, delay time.Duration) {
	d.deb.Run(dir, delay, func(ctx context.Context) { d.run(ctx, dir) })
}

// scheduleFile schedules a run for the package of the file uri.
func (d *diagnostics) scheduleFile(uri protocol.DocumentURI, delay time.Duration) {
	if path := uri.Path(); path != "" {
		d.schedule(filepath.Dir(path), delay)
	}
}

// openDirs returns the folders of the open .gp documents of snap.
func openDirs(snap *cache.Snapshot) map[string]bool {
	dirs := make(map[string]bool)
	for _, o := range snap.Overlays() {
		if path := o.URI().Path(); strings.HasSuffix(path, ".gp") {
			dirs[filepath.Dir(path)] = true
		}
	}
	return dirs
}

// scheduleOpen schedules a run for every open package but the one in skip.
func (d *diagnostics) scheduleOpen(skip string, delay time.Duration) {
	snap, release := d.s.session.Snapshot()
	dirs := openDirs(snap)
	release()
	for dir := range dirs {
		if dir != skip {
			d.schedule(dir, delay)
		}
	}
}

// run diagnoses the package of the folder dir on the current snapshot.
func (d *diagnostics) run(ctx context.Context, dir string) {
	ctx, span := d.s.obs.begin(ctx, "diagnostics", "dir", dir)
	defer d.s.obs.end(span, func() string { return fmt.Sprintf("dir=%s (now: %s)", dir, d.s.snapshotInfo()) })
	for try := 0; ; try++ {
		snap, release := d.s.session.Snapshot()
		pkg, err := snap.Package(ctx, dir)
		if ctx.Err() != nil {
			release()
			return
		}
		if err != nil {
			// No package (a folder that is gone): what it published goes,
			// and the importers are checked again.
			id := snap.ID()
			release()
			d.s.log.Printf("diagnostics of %s: %v", dir, err)
			if d.forget(dir, id) {
				d.scheduleOpen(dir, 0)
			}
			return
		}
		if d.checked != nil {
			d.checked(ctx, dir, snap.ID(), try)
		}
		if try < maxRetries && d.currentID() != snap.ID() {
			release()
			continue
		}
		apiChanged, stale := d.publish(ctx, snap, dir, pkg)
		release()
		if apiChanged {
			d.scheduleOpen(dir, 0)
		}
		if stale {
			// A probe answer came after the check: probed may have
			// looked before st.pkg was this package.
			d.recheck([]string{dir})
		}
		return
	}
}

// currentID returns the ID of the session's current snapshot.
func (d *diagnostics) currentID() uint64 {
	snap, release := d.s.session.Snapshot()
	defer release()
	return snap.ID()
}

// publish publishes the diagnostics of pkg, the package of the folder dir
// in snap, and reports whether its API hash changed since the last run, and
// whether pkg, which is open, is stale already (a namespace probe answer
// came after its check: it must be checked again).
func (d *diagnostics) publish(ctx context.Context, snap *cache.Snapshot, dir string, pkg *cache.Package) (apiChanged, stale bool) {
	open := make(map[string]*cache.Overlay) // the open documents of the package, by path
	for _, o := range snap.Overlays() {
		if path := o.URI().Path(); strings.HasSuffix(path, ".gp") && filepath.Dir(path) == dir {
			open[path] = o
		}
	}
	var byFile map[string][]protocol.Diagnostic
	if len(open) > 0 {
		byFile = diagnose(ctx, snap, dir, pkg, open, d.related.Load())
		for path := range open {
			if _, ok := byFile[path]; !ok {
				byFile[path] = nil
			}
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if ctx.Err() != nil {
		return false, false // a newer run comes
	}
	st := d.pkgs[dir]
	if st == nil {
		st = &pkgState{files: make(map[protocol.DocumentURI]bool)}
		d.pkgs[dir] = st
	}
	apiChanged = !st.known || st.api != pkg.APIHash
	st.known, st.api, st.pkg = true, pkg.APIHash, nil
	if len(open) > 0 {
		// From now on probed sees pkg; an answer before is seen here.
		st.pkg = pkg
		stale = pkg.Stale()
	}

	targets := make(map[protocol.DocumentURI]bool, len(byFile))
	paths := make([]string, 0, len(byFile))
	for path := range byFile {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		ds := byFile[path]
		uri := protocol.URIFromPath(path)
		var version int32
		if o := open[path]; o != nil {
			uri, version = o.URI(), o.Version()
			if o.Broken() {
				ds = nil
			}
		}
		targets[uri] = true
		if d.send(uri, snap.ID(), version, ds, open[path] != nil) {
			st.files[uri] = true
		} else {
			delete(st.files, uri)
		}
	}
	for uri := range st.files {
		if !targets[uri] {
			d.send(uri, snap.ID(), 0, nil, false)
			delete(st.files, uri)
		}
	}
	return apiChanged, stale
}

// forget clears the diagnostics of the package of dir, which is gone in the
// snapshot snapID, and forgets it. It reports whether the package had an
// API (then its importers must be checked again).
func (d *diagnostics) forget(dir string, snapID uint64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	st := d.pkgs[dir]
	if st == nil {
		return false
	}
	for uri := range st.files {
		d.send(uri, snapID, 0, nil, false)
	}
	delete(d.pkgs, dir)
	return st.known
}

// send publishes the diagnostics ds of the file uri, from the snapshot
// snapID, unless the client has them with this version already, or has
// those of a newer snapshot. A file that is not open (keep is false) and
// has none is cleared once and then forgotten. It reports whether the
// client has diagnostics of the file now (or an open file's empty list).
// d.mu is held.
func (d *diagnostics) send(uri protocol.DocumentURI, snapID uint64, version int32, ds []protocol.Diagnostic, keep bool) bool {
	if ds == nil {
		ds = []protocol.Diagnostic{}
	}
	data, err := json.Marshal(ds)
	if err != nil {
		d.s.log.Printf("publishDiagnostics %s: %v", uri, err)
		return false
	}
	text := string(data)
	prev := d.files[uri]
	switch {
	case prev != nil && prev.snapshot > snapID:
		// The client has a newer publish. (A backstop: the runs of a
		// folder, which publish its files, run one after the other, in
		// order, by the debouncer.)
		return true
	case prev != nil && prev.text == text && prev.version == version:
		return keep || len(ds) > 0
	case prev == nil && len(ds) == 0 && !keep:
		return false // nothing to clear
	}
	if d.s.conn != nil {
		err = d.s.conn.Notify("textDocument/publishDiagnostics", &protocol.PublishDiagnosticsParams{URI: uri, Version: version, Diagnostics: ds})
		if err != nil && !errors.Is(err, protocol.ErrClosed) {
			d.s.log.Printf("publishDiagnostics %s: %v", uri, err)
		}
	}
	if len(ds) == 0 && !keep {
		delete(d.files, uri)
		return false
	}
	d.files[uri] = &published{snapshot: snapID, version: version, text: text}
	return true
}

// probed is Config.OnProbe: the namespace probe has new answers, so the
// open packages whose checks are stale are checked again in a new snapshot.
func (d *diagnostics) probed() {
	d.mu.Lock()
	var dirs []string
	for dir, st := range d.pkgs {
		if st.pkg != nil && st.pkg.Stale() {
			dirs = append(dirs, dir)
		}
	}
	d.mu.Unlock()
	d.recheck(dirs)
}

// recheck checks the packages of the folders dirs again in a new snapshot
// (Session.Recheck: their handles and those of their importers are new,
// nothing is read again) and diagnoses them.
func (d *diagnostics) recheck(dirs []string) {
	if len(dirs) == 0 {
		return
	}
	if err := d.s.session.Recheck(dirs); err != nil {
		return // the session is shut down
	}
	for _, dir := range dirs {
		d.schedule(dir, 0)
	}
}

// A sourceFile is a file whose positions are converted to LSP positions.
type sourceFile struct {
	src    []byte
	mapper *protocol.Mapper
	lines  []int // the byte offset of the start of each line
}

func newSourceFile(src []byte) *sourceFile {
	f := &sourceFile{src: src, mapper: protocol.NewMapper(string(src)), lines: []int{0}}
	for i, b := range src {
		if b == '\n' {
			f.lines = append(f.lines, i+1)
		}
	}
	return f
}

// diagnose returns the diagnostics of pkg, the package of the folder dir,
// by file: for every file of the package, and every file of dir that a
// diagnostic is in. A diagnostic without a place goes to every file of the
// package, and to every open file that the package leaves out (build
// constraints exclude it, or all files): "build constraints exclude all
// .gp files". With related, the related places are relatedInformation.
func diagnose(ctx context.Context, snap *cache.Snapshot, dir string, pkg *cache.Package, open map[string]*cache.Overlay, related bool) map[string][]protocol.Diagnostic {
	byFile := make(map[string][]protocol.Diagnostic)
	sources := make(map[string]*sourceFile)
	source := func(path string) *sourceFile {
		if f, ok := sources[path]; ok {
			return f
		}
		var f *sourceFile
		if src, err := snap.ReadFile(path); err == nil {
			f = newSourceFile(src)
		}
		sources[path] = f
		return f
	}
	root := pkg.Program.Roots[0]
	// The package clause of each file, where a diagnostic without a place
	// (about the package as a whole) goes.
	clause := make(map[string]token.Position)
	inPkg := make(map[string]bool)
	var placeless []string // the files of the diagnostics without a place
	for _, f := range root.Files {
		byFile[f.Path] = nil
		inPkg[f.Path] = true
		placeless = append(placeless, f.Path)
		if f.AST != nil && f.AST.Name != nil && f.AST.Name.Pos().IsValid() {
			clause[f.Path] = pkg.Fset().Position(f.AST.Name.Pos())
		}
	}
	var left []string
	for path := range open {
		if !inPkg[path] {
			left = append(left, path)
		}
	}
	slices.Sort(left)
	for _, path := range left {
		placeless = append(placeless, path)
		inPkg[path] = true // its positions are exact (the loader parsed it)
		pf, err := snap.ParseFile(ctx, open[path].URI(), cache.ParseFull)
		if err == nil && pf.File.Name != nil && pf.File.Name.Pos().IsValid() {
			clause[path] = token.Position{Filename: path, Offset: pf.Tok.Offset(pf.File.Name.Pos())}
		}
	}
	var all []diag.Diagnostic
	for _, l := range []*diag.List{pkg.Program.Errors, pkg.Check.Errors} {
		all = append(all, l.Diagnostics()...)
	}
	for _, x := range all {
		if x.Pos.Filename == "" {
			for _, path := range placeless {
				y := x
				y.Pos, y.End = clause[path], token.Position{}
				if y.Pos.Filename == "" {
					y.Pos = token.Position{Filename: path, Line: 1, Column: 1}
				}
				byFile[path] = append(byFile[path], toProtocol(y, source, inPkg, related))
			}
			continue
		}
		if filepath.Dir(x.Pos.Filename) != dir {
			continue // in another package (an import cycle): its own run shows it
		}
		inPkg[x.Pos.Filename] = true // a file of dir that the package left out
		byFile[x.Pos.Filename] = append(byFile[x.Pos.Filename], toProtocol(x, source, inPkg, related))
	}
	for path, ds := range byFile {
		slices.SortStableFunc(ds, func(a, b protocol.Diagnostic) int {
			return cmp.Or(cmp.Compare(a.Range.Start.Line, b.Range.Start.Line), cmp.Compare(a.Range.Start.Character, b.Range.Start.Character))
		})
		byFile[path] = ds
	}
	return byFile
}

// toProtocol converts a diagnostic in a file of the package (inPkg) to LSP.
// Without related, the related places are lines of the message.
func toProtocol(x diag.Diagnostic, source func(string) *sourceFile, inPkg map[string]bool, related bool) protocol.Diagnostic {
	d := protocol.Diagnostic{Severity: protocol.SeverityError, Source: diagSource, Message: x.Msg}
	if x.Severity == diag.Warning {
		d.Severity = protocol.SeverityWarning
	}
	if f := source(x.Pos.Filename); f != nil {
		start := offsetOf(f, x.Pos, true)
		end := start
		if x.End.Filename == x.Pos.Filename && x.End.Line > 0 {
			end = max(start, offsetOf(f, x.End, true))
		}
		if end == start {
			end = tokenEnd(f.src, start)
		}
		d.Range = protocol.Range{Start: f.mapper.Position(start), End: f.mapper.Position(end)}
	}
	for _, r := range x.Related {
		var f *sourceFile
		if r.Pos.Filename != "" && related {
			f = source(r.Pos.Filename)
		}
		if f == nil {
			// No place, a file that can't be read, or a client without
			// relatedInformation: a line of the message, as gocpp check
			// prints it.
			if r.Pos.Filename != "" {
				d.Message += fmt.Sprintf("\n\t%s: %s", r.Pos, r.Msg)
			} else {
				d.Message += "\n\t" + r.Msg
			}
			continue
		}
		start := offsetOf(f, r.Pos, inPkg[r.Pos.Filename])
		rng := protocol.Range{Start: f.mapper.Position(start), End: f.mapper.Position(tokenEnd(f.src, start))}
		d.RelatedInformation = append(d.RelatedInformation, protocol.DiagnosticRelatedInformation{
			Location: protocol.Location{URI: protocol.URIFromPath(r.Pos.Filename), Range: rng},
			Message:  r.Msg,
		})
	}
	return d
}

// offsetOf returns the byte offset of pos in f: its Offset when f is a
// file of the checked package (exact), whose positions have one. A position
// in another file is from export data (an object of an import): it has no
// right Offset, but a line and a column (which counts bytes; the importer
// gives 1, the start of the line), and its offset comes from them.
func offsetOf(f *sourceFile, pos token.Position, exact bool) int {
	if exact || pos.Line <= 0 || pos.Line > len(f.lines) {
		return min(max(pos.Offset, 0), len(f.src))
	}
	lineStart := f.lines[pos.Line-1]
	lineEnd := len(f.src)
	if pos.Line < len(f.lines) {
		lineEnd = f.lines[pos.Line] - 1
	}
	return min(lineStart+max(pos.Column-1, 0), lineEnd)
}

// tokenEnd returns the end of the token that starts at the byte offset off
// of src, on its line: the end of a diagnostic without one. It is off when
// no token starts there (a line end, the end of the file, a space).
func tokenEnd(src []byte, off int) int {
	if off < 0 || off >= len(src) {
		return off
	}
	line := src[off:]
	if i := bytes.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if len(line) == 0 {
		return off
	}
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(line))
	var s scanner.Scanner
	s.Init(file, line, nil, scanner.ScanComments) // a warning about a comment spans it
	pos, tok, lit := s.Scan()
	if tok == token.EOF || tok == token.SEMICOLON && lit == "\n" || file.Offset(pos) != 0 {
		return off
	}
	n := len(lit)
	if n == 0 {
		n = len(tok.String())
	}
	return off + min(n, len(line))
}
