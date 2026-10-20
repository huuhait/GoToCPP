package check

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sync"
	"testing"
)

// TestTypeUseFeaturesComplete checks that typeUseFeatures names every feature that
// typeUses asks the gate about: the walk of the types is left out when all of them are
// allowed, so a feature missing from the list would lose its errors.
func TestTypeUseFeaturesComplete(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "gatewalk.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "typeUses" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if ok && len(id.Name) > 4 && id.Name[:4] == "feat" && id.Name[4] >= 'A' && id.Name[4] <= 'Z' {
				used[id.Name] = true
			}
			return true
		})
	}
	if len(used) == 0 {
		t.Fatal("typeUses was not found, or asks about no feature")
	}
	listed := map[featureID]bool{}
	for _, id := range typeUseFeatures {
		listed[id] = true
	}
	// The names of the features, from the table, by their identifiers.
	names := map[string]featureID{
		"featComplex": featComplex, "featPointers": featPointers, "featComposite": featComposite,
		"featChannels": featChannels, "featFuncValues": featFuncValues, "featGenerics": featGenerics,
		"featAny": featAny, "featInterfaces": featInterfaces, "featGenericTypes": featGenericTypes,
		"featErrors": featErrors,
	}
	for name := range used {
		id, ok := names[name]
		if !ok {
			t.Errorf("typeUses asks about %s, which this test does not know: add it to typeUseFeatures and names",
				name)
			continue
		}
		if !listed[id] {
			t.Errorf("typeUses asks about %s, which typeUseFeatures lacks", name)
		}
	}
	for name, id := range names {
		if !used[name] {
			t.Errorf("typeUseFeatures names %s, which typeUses no longer asks about", name)
		}
		if !listed[id] {
			t.Errorf("%s is not in typeUseFeatures", name)
		}
	}
}

// TestGateElision checks that leaving out the walks of the gates that cannot find
// anything (typesGated: the types of the expressions; mayHaveMemberRule: the uses of
// objects of packages without rows) changes no gate error: at phases 5, 8, 9 and 15 the
// diagnostics of the programs of the repository are the same with and without.
func TestGateElision(t *testing.T) {
	programs := corpusPrograms(t)
	defer func() { noGateElision = false }()
	// (Below phase 8 a type feature is gated, so the walk of the types is not left out there: the
	// phase 5 checks that it is not.)
	for _, phase := range []int{5, 8, 9, LastPhase} {
		results := make([][2]string, len(programs))
		for i, elide := range []bool{true, false} {
			noGateElision = !elide
			var wg sync.WaitGroup
			sem := make(chan struct{}, 6)
			for j, pr := range programs {
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					prog := loadCorpus(pr)
					r := Check(prog, fakeProbeConfig(phase))
					results[j][i] = errorText(t, prog, r, pr.dir, true)
				}()
			}
			wg.Wait()
		}
		noGateElision = false
		for j, pr := range programs {
			if results[j][0] != results[j][1] {
				t.Errorf("phase %d, %s: the gate errors differ with the elision\nwith:\n%s\nwithout:\n%s",
					phase, pr.name, results[j][0], results[j][1])
			}
		}
	}
}
