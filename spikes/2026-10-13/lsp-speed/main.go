// How fast is "re-check one package after a keystroke" with go/types,
// when dependencies come from cached export data (the gopls model)?
package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func median(ds []time.Duration) time.Duration {
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds[len(ds)/2]
}

func main() {
	imp := importer.Default() // export data for deps (cached by the go command)
	for _, path := range []string{"strings", "fmt", "encoding/json", "go/types", "net/http"} {
		bp, err := build.Import(path, "", 0)
		if err != nil {
			panic(err)
		}
		lines := 0
		var parseT []time.Duration
		var files []*ast.File
		var fset *token.FileSet
		for run := 0; run < 7; run++ {
			fset = token.NewFileSet()
			files = files[:0]
			lines = 0
			t0 := time.Now()
			for _, name := range bp.GoFiles {
				src, _ := os.ReadFile(filepath.Join(bp.Dir, name))
				lines += strings.Count(string(src), "\n")
				f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
				if err != nil {
					panic(err)
				}
				files = append(files, f)
			}
			parseT = append(parseT, time.Since(t0))
		}
		// warm the importer once (first use reads export data)
		conf := types.Config{Importer: imp, Error: func(error) {}}
		conf.Check(path, fset, files, nil)
		var checkT []time.Duration
		for run := 0; run < 7; run++ {
			info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
			t0 := time.Now()
			conf.Check(path, fset, files, info)
			checkT = append(checkT, time.Since(t0))
		}
		fmt.Printf("%-14s %6d lines %3d files   parse %6.1f ms   type-check %6.1f ms   (median of 7)\n",
			path, lines, len(files), float64(median(parseT).Microseconds())/1000, float64(median(checkT).Microseconds())/1000)
	}
}
