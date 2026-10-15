package cache

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/gcexportdata"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// The summaries on disk (README §2.2 item 4, step 11.3b). When the summary
// of a package is computed for an importer (summaryOf), it is also written
// to the disk cache (Config.Disk), and a summary that is not in memory is
// looked for there before the package is checked. So a server that starts
// again checks from source only the packages it shows (and those that
// changed), and loads the rest from disk.
//
// The key of an entry is the key of the package's check (keyOf: the files,
// what the resolution of the imports read, the configuration, and the API
// hashes of the imports, which come from their own summaries) with
// diskFormat; the cache keeps the entries of each gocpp binary apart (its
// hash, see filecache.Options.Version), since the export data and this
// format belong to the binary. An entry has what summary has: the
// loader's summary, the export data, the side table of opaque types, the
// API hash, and the answers of the C++ compiler lookup, of the kind
// probes, of the C header check and of pkg-config that the check got (a diskProbe). Those answers are not in the
// key (they depend on the headers and the compiler on the machine, not on
// the files): a summary from disk asks the prober for them again
// (prober.replay), and is provisional until they come and stale if they
// differ, like a check that went on without them.
//
// Not written: the summaries of the packages that have an open document
// (they change with each keystroke, and are checked from source anyway to
// show them), of packages in an import cycle (their keys don't have all
// their files), and those whose check is provisional or stale.

// diskFormat is the version of the format of an entry: a change of the
// format must change it (and the binary's hash changes the folder anyway).
// 2: the probe uses have the flags, and the answers are the kind probe's
// (Phase 04). 3: the answers of pkg-config (step 4.15). 4: the answers of
// the C header check (step 4.14).
const diskFormat = 4

// diskKind is the kind of the summaries in the disk cache.
const diskKind = "summary"

// A diskSummary is a summary on disk (gob).
type diskSummary struct {
	ImportPath string
	Load       diskLoad
	HasExport  bool
	Export     []byte
	HasC       bool
	Failed     bool
	APIHash    Hash
	Opaque     []opaqueEntry
	Probe      *diskProbe // nil when the check asked for neither the compiler nor a probe
}

// A diskLoad is a load.Package summary on disk. Of its errors, it keeps
// those that importers use (packageErrors: the ones about the package as a
// whole, which they report at their imports); the others are the package's
// own, which its importers never look at.
type diskLoad struct {
	ImportPath, Name, Dir string
	Std                   bool
	Module                *load.Module
	Errors                []diag.Diagnostic
	Incomplete            bool
	NoFiles               load.NoFilesReason
	ForTest               string
	Exported              []string
}

// diskKey returns the key of the summary of a check with the key key.
func diskKey(key Hash) Hash {
	h := newHasher("disk summary")
	h.int(diskFormat)
	h.hash(key)
	return h.sum()
}

// encodeSummary returns the content of the entry of s, or false when s must
// not be written: its check, or one of its imports, is provisional or
// stale.
func encodeSummary(s *summary) ([]byte, bool) {
	for _, r := range s.records {
		if r.provisional() || r.stale() {
			return nil, false
		}
	}
	d := diskSummary{
		ImportPath: s.importPath,
		Load: diskLoad{
			ImportPath: s.load.ImportPath,
			Name:       s.load.Name,
			Dir:        s.load.Dir,
			Std:        s.load.Std,
			Module:     s.load.Module,
			Errors:     packageErrors(s.load.Errors),
			Incomplete: s.load.Incomplete,
			NoFiles:    s.load.NoFiles,
			ForTest:    s.load.ForTest,
			Exported:   s.load.ExportedNames(),
		},
		HasExport: s.export != nil,
		Export:    s.export,
		HasC:      s.hasC,
		Failed:    s.failed,
		APIHash:   s.apiHash,
		Opaque:    s.opaque,
	}
	if s.own != nil {
		p, ok := s.own.diskForm()
		if !ok {
			return nil, false
		}
		d.Probe = p
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&d); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// decodeSummary decodes the content of an entry. The cache checked that
// the content is whole, so an error here is a bug (a change of the format
// without a change of diskFormat), never a crash.
func decodeSummary(data []byte) (d *diskSummary, err error) {
	defer func() {
		if r := recover(); r != nil {
			d, err = nil, fmt.Errorf("cache: a bad summary on disk: %v", r)
		}
	}()
	d = new(diskSummary)
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(d); err != nil {
		return nil, err
	}
	return d, nil
}

// readableExport reports whether gcexportdata can read the export data of
// the package path.
func readableExport(path string, export []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cache: the export data of %s: %v", path, r)
		}
	}()
	_, err = gcexportdata.Read(bytes.NewReader(export), token.NewFileSet(), make(map[string]*types.Package), path)
	return err
}

// diskSummaryOf returns the summary of h with the key k from the disk
// cache, or nil if it is not there or is stale already.
func (p *Pipeline) diskSummaryOf(h *handle, k *keyed) *summary {
	if p.cfg.Disk == nil || h.cyclic {
		return nil
	}
	key := diskKey(k.key)
	data, ok := p.cfg.Disk.Get(diskKind, key)
	if !ok {
		p.stats.diskMisses.Add(1)
		return nil
	}
	// The cache checked that the entry is whole; an entry that is whole
	// but can't be used (a change of the format without a change of
	// diskFormat, or export data that gcexportdata can't read: a bug) is
	// removed, and the package is checked from source. The export data
	// are read once here so that no importer's check fails on them.
	d, err := decodeSummary(data)
	if err == nil && d.HasExport {
		err = readableExport(d.ImportPath, d.Export)
	}
	if err != nil {
		p.cfg.Disk.Remove(diskKind, key)
		p.stats.diskMisses.Add(1)
		return nil
	}
	errs := new(diag.List)
	for _, e := range d.Load.Errors {
		errs.Add(e)
	}
	lp := load.NewSummary(&load.Package{
		ImportPath: d.Load.ImportPath,
		Name:       d.Load.Name,
		Dir:        d.Load.Dir,
		Std:        d.Load.Std,
		Module:     d.Load.Module,
		Errors:     errs,
		Incomplete: d.Load.Incomplete,
		NoFiles:    d.Load.NoFiles,
		ForTest:    d.Load.ForTest,
	}, d.Load.Exported)
	s := &summary{importPath: d.ImportPath, load: lp, hasC: d.HasC, failed: d.Failed, apiHash: d.APIHash, opaque: d.Opaque}
	if d.HasExport {
		s.export = d.Export
		if s.export == nil {
			s.export = []byte{} // gob writes an empty slice as nothing
		}
	}
	if d.Probe != nil {
		rec := p.prober.replay(d.Probe)
		if rec.stale() {
			p.stats.diskMisses.Add(1)
			return nil // the prober answers differently now: check it again
		}
		s.own = rec
		s.records = append(s.records, rec)
	}
	s.inherit(k.deps)
	p.stats.diskHits.Add(1)
	return s
}

// saveSummary writes the summary s of h with the key k to the disk cache,
// in the background (Close waits for it), unless it is not to be written
// (see above).
func (p *Pipeline) saveSummary(h *handle, k *keyed, s *summary) {
	if p.cfg.Disk == nil || h.cyclic || h.open || (s.own != nil && s.own.disk) {
		return
	}
	p.diskMu.Lock()
	defer p.diskMu.Unlock()
	if p.closed {
		return
	}
	p.diskWG.Add(1)
	go func() {
		defer p.diskWG.Done()
		p.diskSem <- struct{}{}
		defer func() { <-p.diskSem }()
		data, ok := encodeSummary(s)
		if !ok {
			return
		}
		if p.cfg.Disk.Set(diskKind, diskKey(k.key), data) == nil {
			p.stats.diskWrites.Add(1)
		}
	}()
}

// DiskStats counts the summaries that the pipeline found in the disk cache
// (hits), did not find there (misses), and wrote to it (writes). The tests
// and benchmarks use it.
func (p *Pipeline) DiskStats() (hits, misses, writes int64) {
	return p.stats.diskHits.Load(), p.stats.diskMisses.Load(), p.stats.diskWrites.Load()
}
