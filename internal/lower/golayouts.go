package lower

import (
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// A GoLayout is the size, alignment and pointer-ness of a type in real Go (gc on amd64).
type GoLayout struct {
	Size, Align int64
	Ptrs        bool
}

// stdGoLayouts are the layouts in real Go of the standard types whose gocpp declaration has
// other fields: a stub that leaves fields out ("and fields that the stub leaves out"), or a
// type that gocpp builds differently (sync.Cond, atomic.Value). The capacity that append gives a
// slice depends on Go's size of the element (gocpp/golayout.hpp), so these structs carry it as
// gp_go_layout. Only exported types are listed: a program cannot name the others, which reach
// it only through pointers. The key is "import/path.Name"; TestStdGoLayouts (stdlib) checks every value
// against real Go 1.25 and that no other standard struct differs.
var stdGoLayouts = map[string]GoLayout{
	"embed.FS":                            {8, 8, true},
	"encoding/json.Decoder":               {304, 8, true},
	"encoding/json.Encoder":               {96, 8, true},
	"encoding/json.InvalidUnmarshalError": {16, 8, true},
	"encoding/json.MarshalerError":        {48, 8, true},
	"encoding/json.SyntaxError":           {24, 8, true},
	"encoding/json.UnmarshalFieldError":   {136, 8, true},
	"encoding/json.UnmarshalTypeError":    {72, 8, true},
	"encoding/json.UnsupportedTypeError":  {16, 8, true},
	"encoding/json.UnsupportedValueError": {40, 8, true},
	"flag.FlagSet":                        {104, 8, true},
	"os.Process":                          {64, 8, true},
	"reflect.MapIter":                     {120, 8, true},
	"regexp.Regexp":                       {160, 8, true},
	"sync.Cond":                           {56, 8, true},
	"sync.Map":                            {48, 8, true},
	"sync/atomic.Pointer":                 {8, 8, true},
	"sync/atomic.Value":                   {16, 8, true},
	"testing.B":                           {648, 8, true},
	"testing.F":                           {520, 8, true},
	"testing.M":                           {152, 8, true},
	"testing.T":                           {448, 8, true},
	"time.Ticker":                         {16, 8, true},
	"time.Timer":                          {16, 8, true},
}

// StdGoLayouts returns the table (a copy), for the test in package stdlib.
func StdGoLayouts() map[string]GoLayout {
	m := make(map[string]GoLayout, len(stdGoLayouts))
	for k, v := range stdGoLayouts {
		m[k] = v
	}
	return m
}

// goLayoutMember returns the member gp_go_layout of the struct si if real Go lays the type out
// differently from its fields (stdGoLayouts), or nil.
func goLayoutMember(si *structInfo) cppir.Decl {
	n, ok := si.t.(*types.Named)
	if !ok {
		return nil
	}
	obj := n.Origin().Obj()
	if obj.Pkg() == nil {
		return nil
	}
	l, ok := stdGoLayouts[obj.Pkg().Path()+"."+obj.Name()]
	if !ok {
		return nil
	}
	return &cppir.VarDecl{Specs: []string{"static", "constexpr"}, Type: typeName("auto"),
		Vars: []cppir.Declarator{{Name: "gp_go_layout", Init: call("gp::detail::go_layout_t",
			lit(strconv.FormatInt(l.Size, 10)), lit(strconv.FormatInt(l.Align, 10)), lit(strconv.FormatBool(l.Ptrs)))}}}
}
