package benchdata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(b)) }

func TestStats(t *testing.T) {
	if Median(nil) != 0 || MAD(nil) != 0 || Min(nil) != 0 {
		t.Fatal("empty input must give 0")
	}
	xs := []int64{9, 1, 5, 3, 7}
	if got := Median(xs); got != 5 {
		t.Errorf("Median odd = %v, want 5", got)
	}
	if xs[0] != 9 {
		t.Error("Median changed its argument")
	}
	if got := Median([]int64{4, 1, 2, 3}); got != 2.5 {
		t.Errorf("Median even = %v, want 2.5", got)
	}
	// deviations of {1,2,3,4,100} from 3: 2,1,0,1,97 -> median 1: one outlier does not move the MAD.
	if got := MAD([]int64{1, 2, 3, 4, 100}); got != 1 {
		t.Errorf("MAD = %v, want 1", got)
	}
	if got := MAD([]int64{5}); got != 0 {
		t.Errorf("MAD of one value = %v, want 0", got)
	}
	if got := Min([]int64{7, 3, 9}); got != 3 {
		t.Errorf("Min = %v", got)
	}
	s := Summarize([]int64{10, 12, 11, 10, 13})
	if s.N != 5 || s.Median != 11 || s.Min != 10 || s.MAD != 1 {
		t.Errorf("Summarize = %+v", s)
	}
	if !near(s.RelNoise(), 1.0/11) {
		t.Errorf("RelNoise = %v", s.RelNoise())
	}
	if (Stat{}).RelNoise() != 0 {
		t.Error("RelNoise of nothing must be 0")
	}
}

func TestGeoMeanAndRatio(t *testing.T) {
	g, n := GeoMean([]float64{1, 4, 16})
	if n != 3 || !near(g, 4) {
		t.Errorf("GeoMean = %v, %d", g, n)
	}
	// values that are not positive or not finite are left out, not zeroed
	g, n = GeoMean([]float64{2, 0, -3, math.Inf(1), math.NaN(), 8})
	if n != 2 || !near(g, 4) {
		t.Errorf("GeoMean with junk = %v, %d", g, n)
	}
	if g, n = GeoMean(nil); g != 0 || n != 0 {
		t.Errorf("GeoMean(nil) = %v, %d", g, n)
	}
	if Ratio(6, 3) != 2 || Ratio(0, 3) != 0 || Ratio(3, 0) != 0 || Ratio(-1, 2) != 0 {
		t.Error("Ratio")
	}
}

func st(vals ...int64) Stat { return Summarize(vals) }

func TestDecide(t *testing.T) {
	base := st(1000, 1010, 990, 1005, 995, 1000, 1002, 998, 1001, 999)
	cfg := GateConfig{Threshold: 0.05}
	cases := []struct {
		name string
		cur  Stat
		cfg  GateConfig
		want Verdict
	}{
		{"same", st(1000, 1003, 997, 1001, 999, 1000, 1002, 998, 1001, 999), cfg, OK},
		{"slower 3 percent is ok", st(1030, 1031, 1029, 1030, 1032, 1028), cfg, OK},
		{"slower 20 percent, tight", st(1200, 1201, 1199, 1200, 1202, 1198, 1200), cfg, Regression},
		{"faster 20 percent", st(800, 801, 799, 800, 802, 798, 800), cfg, Improved},
		// the median is up but the fastest run is not: a busy machine, never a failure
		{"noisy median", st(1000, 1300, 1310, 1290, 1305, 1295, 1300), cfg, Inconclusive},
		{"too few runs", st(1300, 1301), cfg, Inconclusive},
		{"no data", Stat{}, cfg, Inconclusive},
		// drift: the whole machine is 25 percent slower now; the program kept up
		{"drift cancels", st(1250, 1251, 1249, 1250, 1252, 1248, 1250), GateConfig{Threshold: 0.05, Drift: 1.25}, OK},
		{"drift does not hide a real regression", st(1500, 1501, 1499, 1500, 1502, 1498, 1500), GateConfig{Threshold: 0.05, Drift: 1.25}, Regression},
	}
	for _, c := range cases {
		d := Decide(base, c.cur, c.cfg)
		if d.Verdict != c.want {
			t.Errorf("%s: verdict %v (%s), want %v", c.name, d.Verdict, d.Reason, c.want)
		}
	}
	// the threshold itself: 8 % is a regression, 4.9 % is not, 5.1 % is (a gate that waits for 10 % is not a 5 % gate)
	tight := st(1000, 1000, 1001, 999, 1000, 1000, 1001, 999, 1000, 1000)
	slow := func(m int64) Stat { return st(m, m, m+1, m-1, m, m, m+1, m-1, m, m) }
	for _, c := range []struct {
		name string
		cur  Stat
		want Verdict
	}{{"8 percent", slow(1080), Regression}, {"5.1 percent", slow(1051), Regression}, {"4.9 percent", slow(1049), OK}, {"6 percent faster", slow(940), Improved}, {"4.9 percent faster", slow(951), OK}, {"5.1 percent faster", slow(949), Improved}} {
		if d := Decide(tight, c.cur, cfg); d.Verdict != c.want {
			t.Errorf("%s: %v (%s), want %v", c.name, d.Verdict, d.Reason, c.want)
		}
	}
	// a slowdown within twice the MAD is never a regression, even when over the threshold
	wide := st(1000, 1400, 600, 1300, 700, 1100, 900, 1000, 1500, 500)
	d := Decide(wide, st(1080, 1500, 700, 1400, 800, 1180, 1000, 1080, 1600, 600), cfg)
	if d.Verdict == Regression {
		t.Errorf("a slowdown inside the noise is a regression: %+v", d)
	}
}

func TestConfirm(t *testing.T) {
	reg, inc, ok := Decision{Verdict: Regression}, Decision{Verdict: Inconclusive}, Decision{Verdict: OK}
	cases := []struct {
		name string
		ds   []Decision
		want Verdict
	}{
		{"none", nil, Inconclusive},
		{"reproduced", []Decision{reg, reg, reg}, Regression},
		{"one clean run clears it", []Decision{reg, ok, reg}, OK},
		{"improved clears it", []Decision{reg, {Verdict: Improved}}, OK},
		{"mix stays inconclusive", []Decision{reg, inc}, Inconclusive},
		{"single regression", []Decision{reg}, Regression},
	}
	for _, c := range cases {
		if got := Confirm(c.ds); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLoadAndDefaultMaxLoad(t *testing.T) {
	if !LoadOK(10, 0) || !LoadOK(3, 4) || LoadOK(5, 4) {
		t.Error("LoadOK")
	}
	if DefaultMaxLoad(128) != 16 || DefaultMaxLoad(8) != 4 || DefaultMaxLoad(2) != 4 {
		t.Errorf("DefaultMaxLoad: %v %v %v", DefaultMaxLoad(128), DefaultMaxLoad(8), DefaultMaxLoad(2))
	}
}

func res(prog, impl string, wall ...int64) Result {
	return Result{Program: prog, Category: "cpu", Impl: impl, Status: StatusOK, Wall: wall, CPU: wall, RSS: []int64{100}}
}

func itoa(i int) string { return strconv.Itoa(i) }

func steady(v int64) []int64 { return []int64{v, v + 1, v - 1, v, v + 2, v - 2, v} }

func report(gocppScale, refScale float64) *Report {
	scale := func(v int64, f float64) []int64 {
		out := steady(int64(float64(v) * f))
		return out
	}
	r := &Report{Schema: SchemaVersion, Commit: "abc", Size: "full", Reps: 7}
	r.Machine.CPU, r.Machine.Cores = "fake cpu", 8
	for _, p := range []string{"a", "b", "c"} {
		r.Results = append(r.Results,
			res(p, ImplGo, scale(10000, refScale)...),
			res(p, ImplCppGxx, scale(9000, refScale)...),
			res(p, ImplGocppGxx, scale(12000, gocppScale)...))
	}
	return r
}

func TestCompareDriftAndMissing(t *testing.T) {
	base := report(1, 1)
	cur := report(1.3, 1.3) // the whole machine is 30 % slower; gocpp kept up
	c := Compare(base, cur, GateConfig{Threshold: 0.05}, true, nil)
	if !near(c.Drift, 1.3) || c.DriftPairs != 6 {
		t.Fatalf("drift %v from %d pairs, want 1.3 from 6", c.Drift, c.DriftPairs)
	}
	if len(c.Rows) != 3 || c.Count(Regression) != 0 {
		t.Errorf("rows %d, regressions %d: %+v", len(c.Rows), c.Count(Regression), c.Rows)
	}
	// without the correction the same data is three regressions
	c = Compare(base, cur, GateConfig{Threshold: 0.05}, false, nil)
	if c.Count(Regression) != 3 {
		t.Errorf("uncorrected: %d regressions, want 3", c.Count(Regression))
	}
	// gocpp itself is 20 % slower on a steady machine: all three regress
	cur = report(1.2, 1)
	c = Compare(base, cur, GateConfig{Threshold: 0.05}, true, nil)
	if c.Count(Regression) != 3 {
		t.Errorf("real regression: %d, want 3", c.Count(Regression))
	}
	// a result that is no longer ok is reported as missing, not compared
	cur.Results[2].Status = StatusRunFailed
	c = Compare(base, cur, GateConfig{Threshold: 0.05}, true, nil)
	if len(c.Missing) != 1 || len(c.Rows) != 2 {
		t.Errorf("missing %v rows %d", c.Missing, len(c.Rows))
	}
	// a baseline entry that was not ok is not a benchmark of the baseline: neither compared nor missing
	base.Results[2].Status = StatusBuildFailed
	c = Compare(base, report(1, 1), GateConfig{Threshold: 0.05}, true, nil)
	if len(c.Missing) != 0 || len(c.Rows) != 2 {
		t.Errorf("a baseline entry that is not ok: missing %v rows %d", c.Missing, len(c.Rows))
	}
}

func TestReportJSONRoundTripAndDeterminism(t *testing.T) {
	r := report(1, 1)
	// shuffle: Sort must give the same bytes whatever the order of the results
	a, err := r.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	r2 := report(1, 1)
	for i, j := 0, len(r2.Results)-1; i < j; i, j = i+1, j-1 {
		r2.Results[i], r2.Results[j] = r2.Results[j], r2.Results[i]
	}
	b, err := r2.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("Marshal depends on the order of the results")
	}
	back, err := Unmarshal(a)
	if err != nil {
		t.Fatal(err)
	}
	c, err := back.Marshal()
	if err != nil || !bytes.Equal(a, c) {
		t.Errorf("round trip changed the bytes (err %v)", err)
	}
	if back.Find("b", ImplGo) == nil || back.Find("zz", ImplGo) != nil {
		t.Error("Find")
	}
	if got := strings.Join(back.Programs(), ","); got != "a,b,c" {
		t.Errorf("Programs = %s", got)
	}
}

func TestReportSchema(t *testing.T) {
	r := report(1, 1)
	data, _ := r.Marshal()
	for _, key := range []string{`"schema"`, `"commit"`, `"date"`, `"size"`, `"reps"`, `"warmup"`, `"machine"`, `"results"`, `"wall_ns"`, `"cpu_ns"`, `"rss_kb"`, `"program"`, `"category"`, `"impl"`, `"status"`, `"load_before"`} {
		if !bytes.Contains(data, []byte(key)) {
			t.Errorf("the JSON has no %s", key)
		}
	}
	if _, err := Unmarshal([]byte(`{"schema": 99, "results": []}`)); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Errorf("a newer schema version must be an error that says so: %v", err)
	}
	if _, err := Unmarshal([]byte(`{"schema": 99, "from_the_future": 1}`)); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Errorf("a newer file with new fields must say newer, not name its field: %v", err)
	}
	if _, err := Unmarshal([]byte(`{"schema": 0}`)); err == nil {
		t.Error("schema 0 must be an error")
	}
	if _, err := Unmarshal([]byte(`{"schema": 1, "bogus": 1}`)); err == nil {
		t.Error("an unknown field must be an error")
	}
	dir := t.TempDir()
	name := filepath.Join(dir, "x", "r.json")
	if err := WriteReport(name, r); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReport(name); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReport(filepath.Join(dir, "missing.json")); !os.IsNotExist(err) && (err == nil || !strings.Contains(err.Error(), "no such file")) {
		t.Errorf("a missing file: %v", err)
	}
}

func TestHistory(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "hist", "m.jsonl")
	r := report(1, 1)
	// a result that is not ok is left out even when it has times (a run that failed halfway)
	r.Results = append(r.Results, Result{Program: "z", Category: "cpu", Impl: ImplGocppGxx, Status: StatusBuildFailed, Wall: []int64{5}})
	e := FromReport(r, "subject")
	if len(e.Wall) != 9 {
		t.Fatalf("the entry has %d results, want 9 (failed ones are left out)", len(e.Wall))
	}
	for i := 0; i < 3; i++ {
		e.Commit = string(rune('a' + i))
		if err := AppendHistory(name, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadHistory(name)
	if err != nil || len(got) != 3 || got[2].Commit != "c" {
		t.Fatalf("ReadHistory = %d entries, %v", len(got), err)
	}
	if again, _ := ReadHistory(filepath.Dir(name)); len(again) != 3 {
		t.Error("reading a folder")
	}
	data, _ := os.ReadFile(name)
	if bytes.Count(data, []byte("\n")) != 3 {
		t.Error("one line per entry")
	}
	// a line of more than 64 KiB (bufio's default limit) reads: 3000 benchmarks of history
	big := e
	big.Wall = map[string]float64{}
	for i := 0; i < 3000; i++ {
		big.Wall[HistKey("cpu", "program"+strings.Repeat("x", 20)+string(rune('a'+i%26))+strings.Repeat("y", i%7)+itoa(i), ImplGo)] = 1
	}
	bigName := filepath.Join(dir, "hist", "big.jsonl")
	if err := AppendHistory(bigName, big); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(bigName); fi.Size() < 128<<10 {
		t.Fatalf("the long line is only %d bytes", fi.Size())
	}
	if got, err := ReadHistory(bigName); err != nil || len(got) != 1 || len(got[0].Wall) != 3000 {
		t.Errorf("a long line: %d entries, %v", len(got), err)
	}
	// a malformed line names the file and the line
	if err := os.WriteFile(filepath.Join(dir, "hist", "bad.jsonl"), []byte("{}\n{oops\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadHistory(filepath.Join(dir, "hist", "bad.jsonl")); err == nil || !strings.Contains(err.Error(), "bad.jsonl:2") {
		t.Errorf("malformed line: %v", err)
	}
	// group ratio: gocpp 12000 over Go 10000 over three programs
	g, n := e.GroupRatio("cpu", ImplGocppGxx, ImplGo)
	if n != 3 || !near(g, 1.2) {
		t.Errorf("GroupRatio = %v, %d", g, n)
	}
	if _, n := e.GroupRatio("data", ImplGocppGxx, ImplGo); n != 0 {
		t.Error("an empty group has no ratio")
	}
	c, p, i := SplitHistKey(HistKey("cpu", "a", "go"))
	if c != "cpu" || p != "a" || i != "go" {
		t.Error("SplitHistKey")
	}
}

func TestMergeResults(t *testing.T) {
	base := report(1, 1)
	add := &Report{Size: "full", Results: []Result{
		res("b", ImplGo, 1, 2, 3), res("z", ImplGo, 4, 5, 6),
	}}
	MergeResults(base, add)
	if got := len(base.Find("b", ImplGo).Wall); got != 3 || base.Find("b", ImplGo).Wall[0] != 1 {
		t.Errorf("b was not replaced: %+v", base.Find("b", ImplGo))
	}
	if base.Find("b", ImplCppGxx) != nil {
		t.Error("the old results of a replaced program must go")
	}
	if base.Find("z", ImplGo) == nil || base.Find("a", ImplGocppGxx) == nil || base.Find("c", ImplGocppGxx) == nil {
		t.Error("programs not in add must stay, new ones must come")
	}
	if len(base.Results) != 8 {
		t.Errorf("%d results, want 8", len(base.Results))
	}
}

// oldResult and oldReport are the layout of version 1 (before Result.Origin) with the strict decoding
// that the readers of that time had (Unmarshal: DisallowUnknownFields, then the version check).
type oldResult struct {
	Program  string  `json:"program"`
	Category string  `json:"category"`
	Impl     string  `json:"impl"`
	Status   string  `json:"status"`
	Note     string  `json:"note,omitempty"`
	Output   string  `json:"output,omitempty"`
	BinSize  int64   `json:"bin_size,omitempty"`
	Stripped int64   `json:"bin_stripped,omitempty"`
	BuildMS  int64   `json:"build_ms,omitempty"`
	Wall     []int64 `json:"wall_ns,omitempty"`
	CPU      []int64 `json:"cpu_ns,omitempty"`
	RSS      []int64 `json:"rss_kb,omitempty"`
}

func oldRead(data []byte) error {
	var r struct {
		Schema  int         `json:"schema"`
		Commit  string      `json:"commit"`
		Dirty   bool        `json:"dirty"`
		Date    string      `json:"date"`
		Size    string      `json:"size"`
		Reps    int         `json:"reps"`
		Warmup  int         `json:"warmup"`
		Machine Machine     `json:"machine"`
		Results []oldResult `json:"results"`
		Compile []Compile   `json:"compile"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return err
	}
	if r.Schema != 1 {
		return fmt.Errorf("schema version %d, this tool reads 1", r.Schema)
	}
	return nil
}

// TestOriginSchema: a report without origins is still version 1 and a strict reader of version 1
// reads it; one with origins is version 2, which that reader rejects (named in SchemaVersion's
// comment and in docs/benchmarks/README.md), and this reader reads both.
func TestOriginSchema(t *testing.T) {
	plain := report(1, 1)
	data, err := plain.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if plain.Schema != 1 || !bytes.Contains(data, []byte(`"schema": 1`)) {
		t.Errorf("a report without origins has schema %d", plain.Schema)
	}
	if err := oldRead(data); err != nil {
		t.Errorf("the version 1 reader fails on a version 1 file: %v", err)
	}
	stamped := report(1, 1)
	stamped.Results[0].Origin = "commit x, load 60"
	data2, err := stamped.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if stamped.Schema != 2 {
		t.Errorf("a report with an origin has schema %d, want 2", stamped.Schema)
	}
	if err := oldRead(data2); err == nil || !strings.Contains(err.Error(), "origin") {
		t.Errorf("the version 1 reader on a version 2 file: %v, want the unknown field origin", err)
	}
	for name, d := range map[string][]byte{"1": data, "2": data2} {
		back, err := Unmarshal(d)
		if err != nil {
			t.Errorf("this reader on version %s: %v", name, err)
		} else if name == "2" && back.Results[0].Origin != "commit x, load 60" {
			t.Errorf("origin lost: %q", back.Results[0].Origin)
		}
	}
}
