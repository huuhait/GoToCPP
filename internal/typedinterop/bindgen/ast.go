package bindgen

import (
	"encoding/json"
	"strings"
)

// The part of clang's JSON AST that the generator reads. Unknown keys are
// dropped by encoding/json, so another clang version only matters when it
// renames one of these.

type jloc struct {
	File         string `json:"file"`
	Line         int    `json:"line"`
	SpellingLoc  *jloc  `json:"spellingLoc"`
	ExpansionLoc *jloc  `json:"expansionLoc"`
}

type jrange struct {
	Begin jloc `json:"begin"`
	End   jloc `json:"end"`
}

type jtype struct {
	QualType          string `json:"qualType"`
	DesugaredQualType string `json:"desugaredQualType"`
	TypeAliasDeclID   string `json:"typeAliasDeclId"`
}

type jref struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type jbase struct {
	Access    string `json:"access"`
	Type      jtype  `json:"type"`
	IsVirtual bool   `json:"isVirtual"`
}

type jnode struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	Name       string  `json:"name"`
	Loc        *jloc   `json:"loc"`
	Range      *jrange `json:"range"`
	IsImplicit bool    `json:"isImplicit"`
	Type       *jtype  `json:"type"`

	StorageClass       string  `json:"storageClass"`
	TagUsed            string  `json:"tagUsed"`
	CompleteDefinition bool    `json:"completeDefinition"`
	Variadic           bool    `json:"variadic"`
	Inline             bool    `json:"inline"`
	Constexpr          bool    `json:"constexpr"`
	Virtual            bool    `json:"virtual"`
	Pure               bool    `json:"pure"`
	Deleted            bool    `json:"explicitlyDeleted"`
	IsInline           bool    `json:"isInline"` // an inline namespace
	IsBitfield         bool    `json:"isBitfield"`
	Language           string  `json:"language"`
	ScopedEnumTag      string  `json:"scopedEnumTag"`
	FixedUnderlying    *jtype  `json:"fixedUnderlyingType"`
	Access             string  `json:"access"` // AccessSpecDecl
	Init               string  `json:"init"`
	Bases              []jbase `json:"bases"`
	ArgType            *jtype  `json:"argType"` // the type operand of sizeof and alignof
	OwnedTagDecl       *jref   `json:"ownedTagDecl"`
	Decl               *jref   `json:"decl"`
	ReferencedDecl     *jref   `json:"referencedDecl"`
	Operator           string  `json:"-"`
	Message            string  `json:"message"` // a DeprecatedAttr's text
	// ParentDeclContextID is set when the declaration is written inside another one but belongs
	// to a different scope: "struct A { struct B *p; };" declares B in A's namespace, not in A.
	ParentDeclContextID string `json:"parentDeclContextId"`

	Value       json.RawMessage `json:"value"`
	Field       *jref           `json:"field"`
	NumElements int             `json:"numElements"`
	CastKind    string          `json:"castKind"`
	Opcode      string          `json:"opcode"`
	// ArrayFill is, for an InitListExpr of an array with fewer elements than the array, the
	// filler expression followed by the elements (clang puts them here, not in Inner).
	ArrayFill []jnode `json:"array_filler"`

	Inner []jnode `json:"inner"`
}

// decodeNode decodes one declaration.
func decodeNode(data []byte) (*jnode, error) {
	n := new(jnode)
	if err := json.Unmarshal(data, n); err != nil {
		return nil, err
	}
	return n, nil
}

// A fileTracker follows clang's habit of printing a file name only where the
// file changes: it replays the locations of a declaration in the order that
// clang printed them.
type fileTracker struct{ file string }

// loc applies one location and returns the file it is in.
func (t *fileTracker) loc(l *jloc) string {
	if l == nil {
		return t.file
	}
	if l.SpellingLoc != nil || l.ExpansionLoc != nil {
		t.loc(l.SpellingLoc)
		t.loc(l.ExpansionLoc)
		return t.file
	}
	if l.File != "" {
		t.file = l.File
	}
	return t.file
}

// node applies the locations of n (its own, then its range) and returns the
// file of its own location.
func (t *fileTracker) node(n *jnode) string {
	f := t.loc(n.Loc)
	if n.Range != nil {
		t.loc(&n.Range.Begin)
		t.loc(&n.Range.End)
	}
	return f
}

// walk applies n and everything inside it, in clang's order, and calls fn for
// each node with the file of its location. fn may be nil.
func (t *fileTracker) walk(n *jnode, fn func(n *jnode, file string)) {
	f := t.node(n)
	if fn != nil {
		fn(n, f)
	}
	for i := range n.ArrayFill { // printed before the children
		t.walk(&n.ArrayFill[i], fn)
	}
	for i := range n.Inner {
		t.walk(&n.Inner[i], fn)
	}
}

// valueString returns the "value" of an expression node as a string: the
// integer of an IntegerLiteral or ConstantExpr, the text of a literal.
func (n *jnode) valueString() (string, bool) {
	if len(n.Value) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(n.Value, &s); err == nil {
		return s, true
	}
	return strings.TrimSpace(string(n.Value)), true
}

// qual returns the written type of the node ("" when it has none).
func (n *jnode) qual() string {
	if n.Type == nil {
		return ""
	}
	return n.Type.QualType
}
