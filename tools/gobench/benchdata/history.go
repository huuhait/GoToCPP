package benchdata

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// HistEntry is one line of a bench/history/*.jsonl file: the median wall times
// of one run of the suite, keyed "category/program/impl", with what is needed
// to judge whether two entries can be compared (the machine, the load).
type HistEntry struct {
	Commit  string             `json:"commit"`
	Subject string             `json:"subject,omitempty"`
	Date    string             `json:"date"`
	Dirty   bool               `json:"dirty,omitempty"`
	Size    string             `json:"size"`
	Reps    int                `json:"reps"`
	Machine string             `json:"machine"` // "cpu, N cores"
	Load    float64            `json:"load"`    // 1-minute load average before the run
	Pinned  string             `json:"pinned,omitempty"`
	Wall    map[string]float64 `json:"wall_ns"`
}

// HistKey is the key of a result in HistEntry.Wall.
func HistKey(category, program, impl string) string { return category + "/" + program + "/" + impl }

// SplitHistKey is the inverse of HistKey.
func SplitHistKey(key string) (category, program, impl string) {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) != 3 {
		return "", key, ""
	}
	return parts[0], parts[1], parts[2]
}

// FromReport makes the history entry of a report (only the results with status ok).
func FromReport(r *Report, subject string) HistEntry {
	e := HistEntry{
		Commit: r.Commit, Subject: subject, Date: r.Date, Dirty: r.Dirty, Size: r.Size, Reps: r.Reps,
		Machine: fmt.Sprintf("%s, %d cores", r.Machine.CPU, r.Machine.Cores),
		Load:    r.Machine.LoadBefore[0], Pinned: r.Machine.Pinned,
		Wall: map[string]float64{},
	}
	for _, x := range r.Results {
		if x.Status != StatusOK || len(x.Wall) == 0 {
			continue
		}
		e.Wall[HistKey(x.Category, x.Program, x.Impl)] = Median(x.Wall)
	}
	return e
}

// AppendHistory appends the entry as one line to the file name (created with
// its folder when missing). The line is compact JSON with sorted keys, so a
// history file diffs well.
func AppendHistory(name string, e HistEntry) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	line, err := json.Marshal(e) // map keys are sorted by encoding/json
	if err != nil {
		return err
	}
	f, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ReadHistory reads the history files that match the glob pattern (or the one
// file, or every *.jsonl file of a folder), the entries in the order of the
// files' names and of the lines. A malformed line is an error that names the
// file and the line.
func ReadHistory(pattern string) ([]HistEntry, error) {
	var files []string
	if st, err := os.Stat(pattern); err == nil && st.IsDir() {
		files, _ = filepath.Glob(filepath.Join(pattern, "*.jsonl"))
	} else if err == nil {
		files = []string{pattern}
	} else {
		files, _ = filepath.Glob(pattern)
	}
	sort.Strings(files)
	var out []HistEntry
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for n := 1; sc.Scan(); n++ {
			line := bytes.TrimSpace(sc.Bytes())
			if len(line) == 0 {
				continue
			}
			var e HistEntry
			dec := json.NewDecoder(bytes.NewReader(line))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&e); err != nil {
				return nil, fmt.Errorf("%s:%d: %w", f, n, err)
			}
			out = append(out, e)
		}
	}
	return out, nil
}

// GroupRatio returns the geometric mean over the benchmarks of group ("all",
// or a category) of impl's median time over the reference implementation's
// time in the entry, and how many benchmarks it used.
func (e HistEntry) GroupRatio(group, impl, ref string) (float64, int) {
	var rs []float64
	for k, v := range e.Wall {
		cat, prog, im := SplitHistKey(k)
		if im != impl || (group != "all" && cat != group) {
			continue
		}
		if rv, ok := e.Wall[HistKey(cat, prog, ref)]; ok {
			rs = append(rs, Ratio(v, rv))
		}
	}
	return GeoMean(rs)
}
