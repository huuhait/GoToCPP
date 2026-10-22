package probe

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// knownPaths resolves the spellings of a dump to the files of a run, as sourceSet.resolve does.
func knownPaths(files ...string) func(string) string {
	return func(spelled string) string {
		for _, file := range files {
			if spelled == file {
				return file
			}
		}
		return ""
	}
}

func TestDumpLocationElision(t *testing.T) {
	paths := knownPaths("/inc/a.h")
	var state locState
	first, rest, ok := state.parseLoc("/inc/a.h:3:5, col:9>", paths)
	if !ok || first.file != "/inc/a.h" || first.line != 3 || first.col != 5 || rest != ", col:9>" {
		t.Fatalf("file place = %+v rest %q ok %v", first, rest, ok)
	}
	sameLine, _, _ := state.parseLoc("col:9>", paths)
	if sameLine.file != "/inc/a.h" || sameLine.line != 3 || sameLine.col != 9 {
		t.Fatalf("column only = %+v, want the file and line printed before", sameLine)
	}
	nextLine, _, _ := state.parseLoc("line:7:2>", paths)
	if nextLine.line != 7 || nextLine.col != 2 {
		t.Fatalf("line place = %+v", nextLine)
	}
	after, _, _ := state.parseLoc("col:4>", paths)
	if after.line != 7 || after.file != "/inc/a.h" {
		t.Fatalf("column after a line place = %+v, want line 7", after)
	}
}

func TestDumpLocationFileNames(t *testing.T) {
	cases := map[string]string{
		"spaces":       "/inc/my headers/a b.h",
		"comma":        "/inc/a,b.h",
		"colon":        "/inc/a:1:2.h",
		"drive":        `C:\inc\api.hpp`,
		"unc":          `\\server\share\api.hpp`,
		"forward":      "C:/inc/api.hpp",
		"line marker":  "/inc/<built-in>.h",
		"relative dot": "./api.hpp",
	}
	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			var state locState
			got, rest, ok := state.parseLoc(file+":12:34, col:40>", knownPaths(file))
			if !ok || got.file != file || got.line != 12 || got.col != 34 || rest != ", col:40>" {
				t.Fatalf("place = %+v rest %q ok %v", got, rest, ok)
			}
		})
	}
}

func TestDumpLocationOutsideTheRunHasNoFile(t *testing.T) {
	for _, spelled := range []string{"<scratch space>:3:1", "/etc/other.h:3:1"} {
		var state locState
		got, _, ok := state.parseLoc(spelled+">", knownPaths("/inc/a.h"))
		if !ok || got.file != "" {
			t.Errorf("%s: place = %+v ok %v, want no file", spelled, got, ok)
		}
	}
}

func readDump(t *testing.T, paths func(string) string, text string) *dumpReader {
	t.Helper()
	reader := newDumpReader(1<<20, paths, nil)
	if _, err := reader.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	reader.finish()
	return reader
}

func TestDumpReaderSplitsLookupsAndDiagnostics(t *testing.T) {
	reader := readDump(t, knownPaths("/inc/a.h"), strings.Join([]string{
		"lookup results for add:",
		"FunctionDecl 0x1 </inc/a.h:1:1, col:14> col:5 add 'int (int)'",
		"`-ParmVarDecl 0x2 <col:9, col:13> col:13 a 'int'",
		"/inc/a.h:9:1: error: something is wrong",
		"lookup results for nosuch:",
		"lookup results for gocpp_loc_end:",
	}, "\n")+"\n")
	if len(reader.groups) != 3 {
		t.Fatalf("groups = %d, want 3", len(reader.groups))
	}
	if got := len(reader.groups[0].nodes); got != 2 {
		t.Errorf("nodes of add = %d, want the function and its parameter", got)
	}
	if got := len(reader.groups[1].nodes); got != 0 {
		t.Errorf("nodes of a name that is not found = %d, want none", got)
	}
	if !strings.Contains(reader.diags.String(), "something is wrong") {
		t.Errorf("diagnostics = %q", reader.diags.String())
	}
}

func TestDumpReaderDroppedLinesStillMoveThePlace(t *testing.T) {
	reader := readDump(t, knownPaths("/inc/a.h"), strings.Join([]string{
		"lookup results for S:",
		"CXXRecordDecl 0x1 </inc/a.h:1:1, line:9:1> line:1:8 struct S definition",
		"|-CXXMethodDecl 0x2 <line:3:3, col:20> col:8 m 'void ()'",
		"| `-CompoundStmt 0x3 <col:12, line:5:3>",
		"|   `-NullStmt 0x4 <line:5:2>",
		"`-FieldDecl 0x5 <col:3, col:7> col:7 y 'int'",
	}, "\n")+"\n")
	nodes := reader.groups[0].nodes
	last := nodes[len(nodes)-1]
	if last.hdr.kind != "FieldDecl" || last.hdr.begin.line != 5 || last.hdr.at.line != 5 {
		t.Fatalf("last node = %+v, want the field on line 5 where the dropped lines left the place", last.hdr)
	}
	for _, node := range nodes {
		if node.hdr.kind == "CompoundStmt" || node.hdr.kind == "NullStmt" {
			t.Errorf("kept the body node %s", node.hdr.kind)
		}
	}
}

func TestDumpReaderMarksTheFunctionsThatHaveABody(t *testing.T) {
	reader := readDump(t, knownPaths("/inc/a.h"), strings.Join([]string{
		"lookup results for f:",
		"FunctionDecl 0x1 </inc/a.h:1:1, line:3:1> line:1:5 f 'int ()'",
		"`-CompoundStmt 0x2 <col:9, line:3:1>",
		"  `-ReturnStmt 0x3 <line:2:3, col:10>",
		"lookup results for g:",
		"FunctionDecl 0x4 </inc/a.h:5:1, col:10> col:5 g 'int ()'",
	}, "\n")+"\n")
	if f := reader.groups[0].nodes[0].hdr; !f.body {
		t.Errorf("f = %+v, want a body", f)
	}
	if g := reader.groups[1].nodes[0].hdr; g.body {
		t.Errorf("g = %+v, want no body", g)
	}
}

func TestDumpReaderLimits(t *testing.T) {
	t.Run("byte limit cancels the run", func(t *testing.T) {
		canceled := false
		reader := newDumpReader(16, knownPaths(), func() { canceled = true })
		reader.Write([]byte(strings.Repeat("x", 64)))
		if !canceled || !reader.over {
			t.Fatalf("canceled = %v over = %v, want both", canceled, reader.over)
		}
	})
	t.Run("node limit marks the group partial", func(t *testing.T) {
		var text strings.Builder
		text.WriteString("lookup results for S:\n")
		text.WriteString("CXXRecordDecl 0x1 </inc/a.h:1:1, line:9:1> line:1:8 struct S definition\n")
		for range maxDumpNodes + 10 {
			text.WriteString("`-FieldDecl 0x5 <line:2:3, col:7> col:7 y 'int'\n")
		}
		reader := readDump(t, knownPaths("/inc/a.h"), text.String())
		group := reader.groups[0]
		if !group.partial || len(group.nodes) != maxDumpNodes {
			t.Fatalf("partial = %v nodes = %d, want partial at %d", group.partial, len(group.nodes),
				maxDumpNodes)
		}
	})
	t.Run("a line without a newline is read at the end", func(t *testing.T) {
		reader := readDump(t, knownPaths(), "lookup results for x:")
		if len(reader.groups) != 1 {
			t.Fatalf("groups = %d, want 1", len(reader.groups))
		}
	})
}

func TestDumpReaderDroppedDeclarationMakesTheSetIncomplete(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "a.h", "int f();\nint f(double);\n")
	path := filepath.Join(dir, "a.h")
	// The first overload has so many nodes below it that the second does not fit: two
	// declarations, well under the limit of overloads, and the dump cut in the second.
	var text strings.Builder
	text.WriteString("lookup results for f:\n")
	text.WriteString("FunctionDecl 0x1 <" + path + ":1:1, col:7> col:5 f 'int ()'\n")
	for range maxDumpNodes + 5 {
		text.WriteString("`-FieldDecl 0x2 <col:3, col:7> col:7 y 'int'\n")
	}
	text.WriteString("FunctionDecl 0x3 <" + path + ":2:1, col:15> col:5 f 'int (double)'\n")
	set := newSourceSet(dir, []string{path})
	reader := readDump(t, set.resolve, text.String())
	group := reader.groups[0]
	if !group.droppedTop || len(group.nodes) != maxDumpNodes {
		t.Fatalf("droppedTop = %v with %d nodes, want a cut set", group.droppedTop, len(group.nodes))
	}
	if decls, _, ok := (&renderer{set: set}).declarations(group, "", false); ok || decls != nil {
		t.Errorf("a cut set of overloads rendered: %d declarations", len(decls))
	}
	// Without the cut the same first declaration renders: the refusal is the cut's.
	group.droppedTop = false
	if decls, _, ok := (&renderer{set: set}).declarations(group, "", false); !ok || len(decls) != 1 {
		t.Errorf("the first overload alone = %v %v, want one declaration", decls, ok)
	}
}

func TestDumpTemplateParameterNames(t *testing.T) {
	for line, want := range map[string]struct {
		name string
		pre  []string
	}{
		"TemplateTypeParmDecl 0x1 <col:10, col:19> col:19 typename depth 0 index 0":     {"", []string{"typename"}},
		"TemplateTypeParmDecl 0x1 <col:10, col:19> col:19 typename depth 0 index 1 T":   {"T", []string{"typename"}},
		"TemplateTypeParmDecl 0x1 <col:10, col:19> col:19 class depth 1 index 2 ... Ts": {"Ts", []string{"class", "..."}},
		"TemplateTypeParmDecl 0x1 <col:10, col:19> col:19 typename depth 0 index 0 ...": {"", []string{"typename", "..."}},
		"TemplateTemplateParmDecl 0x1 <col:10, col:19> col:19 depth 0 index 0 Tmpl":     {"Tmpl", nil},
	} {
		var state locState
		paths := knownPaths("/inc/a.h")
		state.file, state.resolved, state.line = "/inc/a.h", "/inc/a.h", 1
		h, ok := parseNodeHeader(line, &state, false, paths)
		if !ok || h.name != want.name || !slices.Equal(h.pre, want.pre) {
			t.Errorf("%q: name %q pre %v ok %v, want %q %v", line, h.name, h.pre, ok, want.name, want.pre)
		}
	}
}
