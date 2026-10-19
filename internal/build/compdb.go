package build

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// compile_commands.json (Phase 06 step 6.11, ADR-044): the JSON compilation database that clangd
// and other tools read (https://clang.llvm.org/docs/JSONCompilationDatabase.html). It has one
// entry for each translation unit of a build with the command that compiles it: the generated C++
// of the packages, the packages' own C and C++ files, the sources of the libraries of modules,
// and the files of gocpp's runtime. The files and the include folders are in stable folders (the
// folder of the generated files of the program in the cache, the runtime's folder in the cache,
// the packages' folders), not in the work folder that a build removes, so a tool can open them
// after the build. The entries are sorted by file, so the same program gives the same bytes.
//
// The "output" field is left out (the object files are in the build's work folder, which is gone
// when the build ends), and so are -o and the object file in the arguments: no tool needs them.

// A CompDBEntry is one entry of compile_commands.json.
type CompDBEntry struct {
	Directory string   `json:"directory"`
	File      string   `json:"file"`
	Arguments []string `json:"arguments"`
}

// CompDB returns the entries of the plan, sorted by file.
func (p *Plan) CompDB() []CompDBEntry {
	var entries []CompDBEntry
	for _, c := range p.sortedCompiles() {
		dir := c.Dir
		if dir == "" {
			dir = filepath.Dir(c.File)
		}
		entries = append(entries, CompDBEntry{Directory: dir, File: c.File, Arguments: append(append([]string(nil), c.Compiler...), c.CompileArgs()...)})
	}
	return entries
}

// CompDBJSON returns compile_commands.json for the plan: an array with one entry for each
// translation unit, indented, ending in a newline.
func (p *Plan) CompDBJSON() ([]byte, error) {
	entries := p.CompDB()
	if len(entries) == 0 {
		return nil, errors.New("the build has no compile commands")
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// WriteCompDB writes the compilation database of the plan to path (the file, or a folder that
// gets compile_commands.json), and returns the name of the file. The file is replaced only when
// its content changes, and by a rename, so that a tool that watches it never reads half of it.
func WriteCompDB(p *Plan, path string) (string, error) {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		path = filepath.Join(path, "compile_commands.json")
	}
	data, err := p.CompDBJSON()
	if err != nil {
		return "", err
	}
	return path, WriteFileIfChanged(path, data)
}

// WriteFileIfChanged writes data to the file name unless it has these bytes already: through a
// temporary file next to it and a rename.
func WriteFileIfChanged(name string, data []byte) error {
	if old, err := os.ReadFile(name); err == nil && bytes.Equal(old, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(name), ".tmp-"+filepath.Base(name)+"-")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr, os.Chmod(tmp.Name(), 0o644)); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), name); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
