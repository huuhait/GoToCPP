package cppir

// The nodes of the tree. Each one is a piece of C++ syntax; the comment shows how Print
// writes it. Fields that hold C++ text (names, literals, operators) hold it exactly as
// written: Print adds no escapes, and checks nothing but the precedence (Prec).

// A Pos is the place in the .gp source that a C++ line maps to: an absolute file name and a
// line (from 1). The zero Pos is no place: the line is generated code (see the package doc).
type Pos struct {
	File string
	Line int
}

// IsValid reports whether p is a place in the .gp source.
func (p Pos) IsValid() bool { return p.Line > 0 }

// A Node is a node of the tree: a Decl, a Stmt, an Expr or a Type.
type Node interface{ node() }

// A Decl is a declaration of a File, a Namespace or a Struct.
type Decl interface {
	Node
	decl()
}

// A Stmt is a statement.
type Stmt interface {
	Node
	stmt()
	position() Pos
}

// An Expr is an expression.
type Expr interface {
	Node
	expr()
}

// A Type is the spelling of a C++ type.
type Type interface {
	Node
	typ()
}

// A File is one C++ translation unit.
type File struct {
	Decls []Decl
}

// ---- Declarations ----

// An Include is `#include <Path>`, or `#include "Path"` if it is not a System header. It has
// no place in the .gp source.
type Include struct {
	Path   string
	System bool
}

// A Namespace is
//
//	namespace Name {
//	Decls...
//	} // namespace Name
//
// Its declarations are not indented. An anonymous namespace has no Name: namespace {.
type Namespace struct {
	Pos   Pos
	Name  string
	Decls []Decl
	End   Pos
}

// A Func is a function, a declaration (Body is nil) or a definition:
//
//	template <Template.Params>                      (if Template is not nil)
//	Specs Result Name(Params) Quals;                (Body nil)
//	Specs Result Name(Params) Quals {               (at Pos)
//	    Body.Stmts...
//	}                                               (at Body.End)
//
// Specs are the specifiers in front (static, inline, constexpr), Quals the words after the
// parameters (const, noexcept, override, "= default"). A constructor has no Result. Body.Pos
// is not used: the opening brace ends the header's line.
type Func struct {
	Pos      Pos
	Template *Template
	Specs    []string
	Result   Type
	Name     string
	Params   []Param
	Quals    []string
	Body     *Block
}

// A Template is the template head of a Func or a Struct: template <typename T, int N>. Each
// parameter is a Param: {Type: TypeName{Name: "typename"}, Name: "T"}.
type Template struct {
	Params []Param
}

// A Param is a parameter of a function, a lambda or a template: `Type Name`, `Type... Name`
// for a parameter pack (Pack), and only `Type` without a Name; `Type Name = Default` with a
// default argument (int = 1 for a template parameter without a name).
type Param struct {
	Type    Type
	Name    string
	Pack    bool
	Default Expr
}

// A Struct is a class type, defined, or only declared (Forward):
//
//	struct Name;                        (Forward)
//	struct Name : Bases {               (at Pos)
//	    Members...
//	};                                  (at End)
//
// Members are fields (VarDecl), member functions (Func), Access labels, Using declarations,
// Comments and Blank lines.
type Struct struct {
	Pos      Pos
	Template *Template
	Name     string
	Bases    []Type
	Forward  bool
	Members  []Decl
	End      Pos
}

// An Access is an access label of a Struct, `public:`, written at the struct's own
// indentation (as .clang-format does it).
type Access struct {
	Pos  Pos
	Name string // public, protected or private
}

// A Using is `using Name;` (a using-declaration, using newtype::newtype;), or `using Name =
// Type;` (an alias) if Type is not nil.
type Using struct {
	Pos  Pos
	Name string
	Type Type
}

// ---- Declarations and statements ----

// A VarDecl declares variables of one type, at namespace scope, in a struct or as a statement:
//
//	Specs Type Vars[0], Vars[1];        constinit int64_t x = 3;  int64_t i = 0, j = n;
type VarDecl struct {
	Pos   Pos
	Specs []string // constinit, static, constexpr, inline, ...
	Type  Type
	Vars  []Declarator
}

// A Declarator is one variable of a VarDecl: `Name`, `Name = Init`, or `Name{a, b}` if Init
// is an InitList without a Type (direct-list-initialization: T x{} is the zero value).
type Declarator struct {
	Name string
	Init Expr
}

// A StaticAssert is `static_assert(Cond);`, a declaration at namespace or class scope.
type StaticAssert struct {
	Pos  Pos
	Cond Expr
}

// A Comment is a comment: `// Text` on a line of its own, `/* Text */` inside a Line or an
// inline block (then Text must not hold */). Text must not hold a newline.
type Comment struct {
	Pos  Pos
	Text string
}

// A Blank is an empty line.
type Blank struct{}

// ---- Statements ----

// An ExprStmt is `X;`.
type ExprStmt struct {
	Pos Pos
	X   Expr
}

// A Return is `return;` (X is nil) or `return X;`.
type Return struct {
	Pos Pos
	X   Expr
}

// A Break is `break;`.
type Break struct{ Pos Pos }

// A Continue is `continue;`.
type Continue struct{ Pos Pos }

// A Goto is `goto Label;`.
type Goto struct {
	Pos   Pos
	Label string
}

// A Label is `Name:;`: a label of an empty statement, because C++20 doesn't allow a label at
// the end of a block.
type Label struct {
	Pos  Pos
	Name string
}

// An Attr is an attribute of an empty statement, `[[Name]];`: [[fallthrough]];.
type Attr struct {
	Pos  Pos
	Name string
}

// A Block is a C++ block, a scope:
//
//	{                                   (at Pos)
//	    Stmts...
//	}                                   (at End)
//
// An Inline block is one line, `{ Stmts... }`, at Pos.
type Block struct {
	Pos    Pos
	Stmts  []Stmt
	End    Pos
	Inline bool
}

// A Scope is a block whose opening line holds its first statements, Head:
//
//	{ Head...                           (at Pos)
//	    Stmts...
//	}                                   (at End)
//
// Lowering opens one where a declaration shadows a variable (ADR-022): the block starts on the
// declaration's own line, int64_t gp_t0 = v + 1; { int64_t v = gp_t0; (a Line that ends with
// the Scope). A Scope whose last statement is another Scope (or a Line that ends with one)
// closes with it, on one line at the outer End: } }.
type Scope struct {
	Pos   Pos
	Head  []Stmt
	Stmts []Stmt
	End   Pos
}

// A Line is statements on one C++ line at Pos, joined with spaces: auto gp_t0 = f(); a = b;.
// Only the last one may go on over more lines (a Scope, a Switch, a block that is not
// inline, ...): its first line is the Line's. The others are written inline, whatever they
// are.
type Line struct {
	Pos   Pos
	Stmts []Stmt
}

// An If is
//
//	if (Cond) {                         (at Pos)
//	    Then.Stmts...
//	} else if (Else.Cond) {             (at Else.Pos, an *If)
//	    ...
//	} else {                            (at Else.Pos, a *Block)
//	    ...
//	}                                   (at the End of the last block)
//
// The If is one line, if (!c) break;, unless Then is a Block that is not inline or Else goes
// on over more lines (then Then is written over several lines, whatever it is).
type If struct {
	Pos  Pos
	Cond Expr
	Then Stmt
	Else Stmt // nil, an *If or a *Block (another statement makes `} else stmt`)
}

// A For is `for (Init Cond; Post) {` at Pos, the body, and `}` at Body.End. Init is a VarDecl
// or an ExprStmt (written with its ;), or nil; Cond and Post may be nil: for (;;) {.
type For struct {
	Pos  Pos
	Init Stmt
	Cond Expr
	Post Expr
	Body *Block
}

// A While is `while (Cond) {` at Pos, the body, and `}` at Body.End.
type While struct {
	Pos  Pos
	Cond Expr
	Body *Block
}

// A Switch is
//
//	switch (Tag) {                      (at Pos)
//	case 1: case 2: {                   (the cases are not indented)
//	    ...
//	} break;
//	}                                   (at End)
type Switch struct {
	Pos   Pos
	Tag   Expr
	Cases []*Case
	End   Pos
}

// A Case is a clause of a Switch, with its body in a block of its own:
//
//	case Values[0]: case Values[1]: {   (at Pos; default: if Values is nil)
//	    Body...
//	} break;                            (at End; only `}` without Break)
type Case struct {
	Pos    Pos
	Values []Expr
	Body   []Stmt
	End    Pos
	Break  bool
}

// A Try is
//
//	try {                               (at Pos)
//	    Body.Stmts...
//	} catch (Catches[0].Param) {        (at Catches[0].Pos; catch (...) if Param is nil)
//	    ...
//	}                                   (at the End of the last catch's Body)
type Try struct {
	Pos     Pos
	Body    *Block
	Catches []*Catch
}

// A Catch is a handler of a Try.
type Catch struct {
	Pos   Pos
	Param *Param
	Body  *Block
}

// ---- Expressions ----

// A Name is a name, possibly qualified, with template arguments if Args is not nil (each a
// Type or an Expr): x, gp::len, gpkg_main::f, std::get<0>, gp::f2i<int64_t>.
type Name struct {
	Name string
	Args []Node
}

// A Lit is a literal, written as Text: 7, 1.5, 0.1f, INT64_MIN, "a\n"_gs, true, nullptr. (A
// negative number is a Unary -.)
type Lit struct {
	Text string
}

// A Call is Fun(Args...). A functional cast int64_t(x) is a Call too.
type Call struct {
	Fun  Expr
	Args []Expr
}

// A Binary is X Op Y, for a binary operator other than assignment: + - * / % << >> & | ^ &&
// || == != < <= > >= <=> and ,.
type Binary struct {
	Op   string
	X, Y Expr
}

// A Unary is Op X, for a prefix operator: - + ! ~ * & ++ --.
type Unary struct {
	Op string
	X  Expr
}

// A Postfix is X Op: x++, x--.
type Postfix struct {
	X  Expr
	Op string
}

// An Index is X[Index].
type Index struct {
	X, Index Expr
}

// A Member is X.Name, or X->Name with Arrow.
type Member struct {
	X     Expr
	Arrow bool
	Name  string
}

// A Paren is (X): parentheses that C++'s precedence doesn't need, for the reader or for a
// compiler warning (Print adds the needed ones itself).
type Paren struct {
	X Expr
}

// An Assign is X Op Y, for = and the compound assignments (+=, <<=, ...).
type Assign struct {
	Op   string
	X, Y Expr
}

// A Cond is Cond ? X : Y.
type Cond struct {
	Cond, X, Y Expr
}

// A Cast is Kind<Type>(X): static_cast<void>(x).
type Cast struct {
	Kind string // static_cast, reinterpret_cast, const_cast
	Type Type
	X    Expr
}

// An InitList is a braced list, {a, b}, or Type{a, b} if Type is not nil. Its elements may be
// Designated: Point{.X = 1, .Y = 2}.
type InitList struct {
	Type  Type
	Elems []Expr
}

// A Designated is .Name = X, an element of an InitList.
type Designated struct {
	Name string
	X    Expr
}

// A Lambda is [Default, Captures]<Template>(Params) Specs -> Result { Body... }, without
// <Template> if it is nil and without -> Result if Result is nil: [n, &m](int64_t x) mutable ->
// int64_t { ... }, [n]<int = 1>() -> int64_t { ... }.
//
// A Lambda whose Body is Inline is written on one line, inside the line of its statement. One
// whose Body is not Inline goes on over several lines (the body of a func literal): the line of
// its statement ends after the opening brace, the body's statements follow one level deeper,
// each at its own place, the closing brace is a line of its own at Body.End, and the rest of the
// statement (`);`) a line at the statement's place, so that the code of the statement maps to
// its line, not to the brace's. With TailAtEnd the rest follows the brace on its line instead
// (`}.operator()();`: a lambda called at once, whose call Go puts on that line). (ExprString
// writes every Lambda on one line.)
type Lambda struct {
	Default  string // the capture default: "", "=" or "&"
	Captures []Capture
	Template []Param // the template parameters (a lambda template, C++20), or nil
	Params   []Param
	Specs    []string // mutable, constexpr, noexcept, ...
	Result   Type
	Body     *Block

	TailAtEnd bool
}

// A Capture is one capture of a Lambda: Name (a copy), &Name with ByRef, and Name = Init or
// &Name = Init with an initializer (a C++14 init-capture). Name may be this, or *this.
type Capture struct {
	Name  string
	ByRef bool
	Init  Expr
}

// An Expand is a pack expansion, X...: f(gp_a...).
type Expand struct {
	X Expr
}

// ---- Types ----

// A TypeName is a named type, with template arguments if Args is not nil (each a Type or an
// Expr): int64_t, gp::string, std::tuple<int64_t, bool>, gp::array<int64_t, 3>, auto, void.
type TypeName struct {
	Name string
	Args []Node
}

// A Ref is a reference, Elem& (Elem&& with RValue).
type Ref struct {
	Elem   Type
	RValue bool
}

// A Ptr is a pointer, Elem*.
type Ptr struct {
	Elem Type
}

// A Const is const Elem (Elem const for a pointer: int64_t* const).
type Const struct {
	Elem Type
}

// A FuncType is a function type, Result(Params): the argument of gp::func<int64_t(gp::string)>.
type FuncType struct {
	Result Type
	Params []Type
}

func (*Include) node()      {}
func (*Namespace) node()    {}
func (*Func) node()         {}
func (*Struct) node()       {}
func (*Access) node()       {}
func (*Using) node()        {}
func (*VarDecl) node()      {}
func (*StaticAssert) node() {}
func (*Comment) node()      {}
func (*Blank) node()        {}

func (*Include) decl()      {}
func (*Namespace) decl()    {}
func (*Func) decl()         {}
func (*Struct) decl()       {}
func (*Access) decl()       {}
func (*Using) decl()        {}
func (*VarDecl) decl()      {}
func (*StaticAssert) decl() {}
func (*Comment) decl()      {}
func (*Blank) decl()        {}

func (*ExprStmt) node() {}
func (*Return) node()   {}
func (*Break) node()    {}
func (*Continue) node() {}
func (*Goto) node()     {}
func (*Label) node()    {}
func (*Attr) node()     {}
func (*Block) node()    {}
func (*Scope) node()    {}
func (*Line) node()     {}
func (*If) node()       {}
func (*For) node()      {}
func (*While) node()    {}
func (*Switch) node()   {}
func (*Try) node()      {}

func (*VarDecl) stmt()  {}
func (*Comment) stmt()  {}
func (*Blank) stmt()    {}
func (*ExprStmt) stmt() {}
func (*Return) stmt()   {}
func (*Break) stmt()    {}
func (*Continue) stmt() {}
func (*Goto) stmt()     {}
func (*Label) stmt()    {}
func (*Attr) stmt()     {}
func (*Block) stmt()    {}
func (*Scope) stmt()    {}
func (*Line) stmt()     {}
func (*If) stmt()       {}
func (*For) stmt()      {}
func (*While) stmt()    {}
func (*Switch) stmt()   {}
func (*Try) stmt()      {}

func (s *VarDecl) position() Pos  { return s.Pos }
func (s *Comment) position() Pos  { return s.Pos }
func (*Blank) position() Pos      { return Pos{} }
func (s *ExprStmt) position() Pos { return s.Pos }
func (s *Return) position() Pos   { return s.Pos }
func (s *Break) position() Pos    { return s.Pos }
func (s *Continue) position() Pos { return s.Pos }
func (s *Goto) position() Pos     { return s.Pos }
func (s *Label) position() Pos    { return s.Pos }
func (s *Attr) position() Pos     { return s.Pos }
func (s *Block) position() Pos    { return s.Pos }
func (s *Scope) position() Pos    { return s.Pos }
func (s *Line) position() Pos     { return s.Pos }
func (s *If) position() Pos       { return s.Pos }
func (s *For) position() Pos      { return s.Pos }
func (s *While) position() Pos    { return s.Pos }
func (s *Switch) position() Pos   { return s.Pos }
func (s *Try) position() Pos      { return s.Pos }

func (*Name) node()       {}
func (*Lit) node()        {}
func (*Call) node()       {}
func (*Binary) node()     {}
func (*Unary) node()      {}
func (*Postfix) node()    {}
func (*Index) node()      {}
func (*Member) node()     {}
func (*Paren) node()      {}
func (*Assign) node()     {}
func (*Cond) node()       {}
func (*Cast) node()       {}
func (*InitList) node()   {}
func (*Designated) node() {}
func (*Lambda) node()     {}
func (*Expand) node()     {}

func (*Name) expr()       {}
func (*Lit) expr()        {}
func (*Call) expr()       {}
func (*Binary) expr()     {}
func (*Unary) expr()      {}
func (*Postfix) expr()    {}
func (*Index) expr()      {}
func (*Member) expr()     {}
func (*Paren) expr()      {}
func (*Assign) expr()     {}
func (*Cond) expr()       {}
func (*Cast) expr()       {}
func (*InitList) expr()   {}
func (*Designated) expr() {}
func (*Lambda) expr()     {}
func (*Expand) expr()     {}

func (*TypeName) node() {}
func (*Ref) node()      {}
func (*Ptr) node()      {}
func (*Const) node()    {}
func (*FuncType) node() {}

func (*TypeName) typ() {}
func (*Ref) typ()      {}
func (*Ptr) typ()      {}
func (*Const) typ()    {}
func (*FuncType) typ() {}
