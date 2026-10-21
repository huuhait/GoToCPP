// phase: 09
// encoding/json and panics: a panic inside MarshalJSON, MarshalText, UnmarshalJSON or
// UnmarshalText reaches the caller unchanged (the package's own internal panics never do), a
// marshaler may call Marshal again (also with errors), and the package stays usable afterwards.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
)

type PanicsString struct{}

func (PanicsString) MarshalJSON() ([]byte, error) { panic("marshal boom") }

type PanicsError struct{}

var errPanic = errors.New("panic error value")

func (PanicsError) MarshalText() ([]byte, error) { panic(errPanic) }

type PanicsNil struct{ Inner *PanicsNil }

func (p PanicsNil) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprint(p.Inner.Inner != nil)), nil
}

type PanicsDecode struct{}

func (*PanicsDecode) UnmarshalJSON([]byte) error { panic(fmt.Sprintf("decode %d", 42)) }

type PanicsDecodeText struct{}

func (*PanicsDecodeText) UnmarshalText([]byte) error { panic(errPanic) }

type Wrapper struct {
	Before int
	Bad    PanicsString
	After  int
}

type Recursive struct{ Depth int }

func (r Recursive) MarshalJSON() ([]byte, error) {
	if r.Depth == 0 {
		return []byte(`"bottom"`), nil
	}
	inner, err := json.Marshal(Recursive{r.Depth - 1})
	if err != nil {
		return nil, err
	}
	return []byte(`{"depth":` + fmt.Sprint(r.Depth) + `,"inner":` + string(inner) + `}`), nil
}

type RecursiveDecode struct {
	Depth int
	Inner *RecursiveDecode
}

func (r *RecursiveDecode) UnmarshalJSON(data []byte) error {
	type plain RecursiveDecode
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("depth wrapper: %w", err)
	}
	*r = RecursiveDecode(decoded)
	r.Depth++
	return nil
}

type FailsAtDepth struct{ Depth int }

func (f FailsAtDepth) MarshalJSON() ([]byte, error) {
	if f.Depth == 0 {
		return nil, errPanic
	}
	return json.Marshal(FailsAtDepth{f.Depth - 1})
}

func attempt(label string, action func() any) {
	defer func() {
		recovered := recover()
		_, isRuntime := recovered.(runtime.Error)
		fmt.Printf("%-24s recovered=%v type=%T runtime=%v\n", label, recovered, recovered, isRuntime)
	}()
	result := action()
	fmt.Printf("%-24s returned %v\n", label, result)
	panic("expected a panic")
}

func main() {
	fmt.Println("== panics pass through unchanged")
	attempt("marshal string", func() any { return must(json.Marshal(PanicsString{})) })
	attempt("marshal error value", func() any { return must(json.Marshal(PanicsError{})) })
	attempt("marshal runtime error", func() any { return must(json.Marshal(PanicsNil{})) })
	attempt("marshal nested", func() any { return must(json.Marshal(Wrapper{Before: 1, After: 2})) })
	attempt("marshal in slice", func() any { return must(json.Marshal([]any{1, PanicsString{}})) })
	attempt("marshal in map", func() any { return must(json.Marshal(map[string]any{"a": PanicsString{}})) })
	attempt("marshal indent", func() any { return must(json.MarshalIndent(PanicsString{}, "", " ")) })
	attempt("encoder", func() any { return json.NewEncoder(&strings.Builder{}).Encode(PanicsString{}) })
	attempt("unmarshal", func() any { return json.Unmarshal([]byte(`1`), new(PanicsDecode)) })
	attempt("unmarshal in struct", func() any { return json.Unmarshal([]byte(`{"A":1}`), &struct{ A PanicsDecode }{}) })
	attempt("unmarshal in slice", func() any { return json.Unmarshal([]byte(`[1]`), &[]PanicsDecode{}) })
	attempt("unmarshal text", func() any { return json.Unmarshal([]byte(`"x"`), new(PanicsDecodeText)) })
	attempt("decoder", func() any { return json.NewDecoder(strings.NewReader(`1`)).Decode(new(PanicsDecode)) })

	fmt.Println("== the package works after panics")
	encoded, err := json.Marshal(map[string]any{"ok": []int{1, 2}, "n": nil})
	fmt.Println(string(encoded), err)
	var decoded map[string]any
	err = json.Unmarshal([]byte(`{"ok":[1,2]}`), &decoded)
	fmt.Println(decoded, err)
	err = json.Unmarshal([]byte(`{"ok":[1,`), &decoded)
	fmt.Println(err)
	_, err = json.Marshal(make(chan int))
	fmt.Println(err)

	fmt.Println("== marshalers that call Marshal")
	for _, depth := range []int{0, 1, 3, 6} {
		encoded, err := json.Marshal(Recursive{depth})
		fmt.Printf("%d: %s %v\n", depth, encoded, err)
	}
	encoded, err = json.Marshal([]Recursive{{1}, {2}})
	fmt.Println(string(encoded), err)
	encoded, err = json.MarshalIndent(map[string]Recursive{"a": {2}}, "", "  ")
	fmt.Println(string(encoded), err)
	for _, depth := range []int{0, 1, 4} {
		encoded, err := json.Marshal(FailsAtDepth{depth})
		fmt.Printf("fails %d: %s %v\n", depth, encoded, err)
	}
	var viaPointer RecursiveDecode
	err = json.Unmarshal([]byte(`{"Depth":5,"Inner":{"Depth":1,"Inner":{"Depth":0}}}`), &viaPointer)
	fmt.Println(viaPointer.Depth, viaPointer.Inner.Depth, viaPointer.Inner.Inner.Depth, err)
	err = json.Unmarshal([]byte(`{"Depth":"x"}`), &viaPointer)
	fmt.Println(err)
	err = json.Unmarshal([]byte(`{"Inner":`), &viaPointer)
	fmt.Println(err)
}

func must(data []byte, err error) any {
	if err != nil {
		return err
	}
	return string(data)
}

// Output:
// == panics pass through unchanged
// marshal string           recovered=marshal boom type=string runtime=false
// marshal error value      recovered=panic error value type=*errors.errorString runtime=false
// marshal runtime error    recovered=runtime error: invalid memory address or nil pointer dereference type=runtime.errorString runtime=true
// marshal nested           recovered=marshal boom type=string runtime=false
// marshal in slice         recovered=marshal boom type=string runtime=false
// marshal in map           recovered=marshal boom type=string runtime=false
// marshal indent           recovered=marshal boom type=string runtime=false
// encoder                  recovered=marshal boom type=string runtime=false
// unmarshal                recovered=decode 42 type=string runtime=false
// unmarshal in struct      recovered=decode 42 type=string runtime=false
// unmarshal in slice       recovered=decode 42 type=string runtime=false
// unmarshal text           recovered=panic error value type=*errors.errorString runtime=false
// decoder                  recovered=decode 42 type=string runtime=false
// == the package works after panics
// {"n":null,"ok":[1,2]} <nil>
// map[ok:[1 2]] <nil>
// unexpected end of JSON input
// json: unsupported type: chan int
// == marshalers that call Marshal
// 0: "bottom" <nil>
// 1: {"depth":1,"inner":"bottom"} <nil>
// 3: {"depth":3,"inner":{"depth":2,"inner":{"depth":1,"inner":"bottom"}}} <nil>
// 6: {"depth":6,"inner":{"depth":5,"inner":{"depth":4,"inner":{"depth":3,"inner":{"depth":2,"inner":{"depth":1,"inner":"bottom"}}}}}} <nil>
// [{"depth":1,"inner":"bottom"},{"depth":2,"inner":{"depth":1,"inner":"bottom"}}] <nil>
// {
//   "a": {
//     "depth": 2,
//     "inner": {
//       "depth": 1,
//       "inner": "bottom"
//     }
//   }
// } <nil>
// fails 0:  json: error calling MarshalJSON for type main.FailsAtDepth: panic error value
// fails 1:  json: error calling MarshalJSON for type main.FailsAtDepth: json: error calling MarshalJSON for type main.FailsAtDepth: panic error value
// fails 4:  json: error calling MarshalJSON for type main.FailsAtDepth: json: error calling MarshalJSON for type main.FailsAtDepth: json: error calling MarshalJSON for type main.FailsAtDepth: json: error calling MarshalJSON for type main.FailsAtDepth: json: error calling MarshalJSON for type main.FailsAtDepth: panic error value
// 6 2 1 <nil>
// depth wrapper: json: cannot unmarshal string into Go struct field plain.Depth of type int
// unexpected end of JSON input
