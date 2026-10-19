package probe

import (
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"sort"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
)

// The cache form of a declaration part (decls.go): the JSON of its records, enums and
// overload sets, with the structured types (TypeText.T, which the JSON of a Record leaves out)
// as text, read back with ctype.Parse. A type whose text does not parse back to the same type
// is stored as JSON instead, so nothing is approximated.

type ctypeFile struct {
	S string          `json:"s,omitempty"`
	J json.RawMessage `json:"j,omitempty"`
	N bool            `json:"n,omitempty"` // nil
}

type partFile struct {
	Records map[string]*Record  `json:"records,omitempty"`
	Enums   map[string]*Enum    `json:"enums,omitempty"`
	Canon   map[string]string   `json:"canon,omitempty"`
	Funcs   map[string]*FuncSet `json:"funcs,omitempty"`
	Errs    map[string]string   `json:"errs,omitempty"`
	Header  []Diag              `json:"header,omitempty"`
	Stats   Stats               `json:"stats"`
	Types   []ctypeFile         `json:"types"`
}

// eachText calls fn for every TypeText of the part, in an order that depends only on its
// content.
func (p *declPart) eachText(fn func(*TypeText)) {
	for _, k := range sortedKeys(p.records) {
		r := p.records[k]
		for i := range r.Fields {
			fn(&r.Fields[i].Type)
		}
		for i := range r.Statics {
			fn(&r.Statics[i].Type)
		}
		for i := range r.Methods {
			fn(&r.Methods[i].Sig)
		}
		for i := range r.Bases {
			fn(&r.Bases[i].Type)
		}
	}
	for _, k := range sortedKeys(p.enums) {
		fn(&p.enums[k].Underlying)
	}
	for _, k := range sortedKeys(p.funcs) {
		f := p.funcs[k]
		for i := range f.Overloads {
			fn(&f.Overloads[i].Sig)
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func encodePart(p *declPart) ([]byte, error) {
	f := partFile{Records: p.records, Enums: p.enums, Canon: p.canon, Funcs: p.funcs, Errs: p.errs, Header: p.header, Stats: p.stats}
	var err error
	p.eachText(func(tt *TypeText) {
		var c ctypeFile
		switch {
		case tt.T == nil:
			c.N = true
		default:
			s := tt.T.String()
			if back, perr := ctype.Parse(s); perr == nil && ctype.Equal(back, tt.T) {
				c.S = s
			} else if b, jerr := json.Marshal(tt.T); jerr == nil {
				c.J = b
			} else {
				err = jerr
			}
		}
		f.Types = append(f.Types, c)
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(&f)
}

func decodePart(body []byte, deps []string) (*declPart, error) {
	var f partFile
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, err
	}
	p := &declPart{records: f.Records, enums: f.Enums, canon: f.Canon, funcs: f.Funcs, errs: f.Errs, header: f.Header, stats: f.Stats, deps: slices.Clone(deps)}
	var err error
	i := 0
	p.eachText(func(tt *TypeText) {
		if err != nil {
			return
		}
		if i >= len(f.Types) {
			err = fmt.Errorf("the cached part has too few types")
			return
		}
		c := f.Types[i]
		i++
		switch {
		case c.N:
			tt.T = nil
		case c.S != "":
			t, perr := ctype.Parse(c.S)
			if perr != nil {
				err = perr
				return
			}
			tt.T = t
		default:
			var t ctype.Type
			if jerr := json.Unmarshal(c.J, &t); jerr != nil {
				err = jerr
				return
			}
			tt.T = &t
		}
	})
	if err == nil && i != len(f.Types) {
		err = fmt.Errorf("the cached part has too many types")
	}
	if err != nil {
		return nil, err
	}
	// The empty maps of a fresh part.
	if p.records == nil {
		p.records = map[string]*Record{}
	}
	if p.enums == nil {
		p.enums = map[string]*Enum{}
	}
	if p.canon == nil {
		p.canon = map[string]string{}
	}
	if p.funcs == nil {
		p.funcs = map[string]*FuncSet{}
	}
	if p.errs == nil {
		p.errs = map[string]string{}
	}
	return p, nil
}

// UnmarshalJSON reads what MarshalJSON writes.
func (e *Enumerator) UnmarshalJSON(b []byte) error {
	var v struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	e.Name, e.Value = v.Name, nil
	if v.Value != "?" {
		n, ok := new(big.Int).SetString(v.Value, 10)
		if !ok {
			return fmt.Errorf("bad enumerator value %q", v.Value)
		}
		e.Value = n
	}
	return nil
}
