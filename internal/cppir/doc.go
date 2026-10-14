// Package cppir is a small C++ syntax tree, and its printer: the last two stages of gocpp's
// backend (Phase 03 README §1).
//
//	typed AST ──► internal/lower (analysis + rewrites) ──► cppir (this tree) ──► Print
//
// The tree maps 1:1 to C++ syntax: a File holds declarations (Namespace, Func, Struct,
// VarDecl, Include, Comment, Blank), a Func's Body holds statements (Block, If, For, While,
// Switch, Return, ...), and statements hold expressions (Name, Lit, Call, Binary, Lambda, ...)
// and types (TypeName, Ref, Ptr, Const, FuncType). It knows nothing about Go: internal/lower
// decides what the C++ is (Go's order of evaluation, the names, the casts, the shapes of the
// statements), and builds it here; Print only writes it.
//
// # Positions and #line
//
// Every node that starts a C++ line has a Pos: the .gp file and line that the line maps to. A
// statement's Pos is the place of the line that it starts, and a node that ends with a closing
// brace on a line of its own has an End (Block, Scope, Switch, Case, ...; a Func's closing
// brace is its Body's End). The zero Pos marks generated code that has no place in the .gp
// source (the prototypes, the main wrapper, gp_init_funcs): Print maps such a line to the
// generated file itself (its out argument, Config.OutputPath), so a debugger doesn't show it
// at an unrelated .gp line. Inside a Line, an inline Block, or anything else that Print writes
// on one line, the inner nodes' positions don't count: the line has one place.
//
// Print writes `#line N "file"` before a mapped line exactly when the line that C++ would give
// it is not already N in that file (the previous line + 1), so the C++ lines of one .gp line
// each get a #line N, and a breakpoint on the next .gp line can't stop between them (Phase 02
// README §2.1).
//
// # What the printer decides
//
// Lowering decides which statements share a C++ line, with the nodes it builds: a Line, an
// inline Block (`{ auto gp_t0 = f(); g(gp_t0); }`), a statement without braces
// (`if (!c) break;`), and a Scope, whose opening line holds its Head (`int64_t gp_t0 = v + 1;
// { int64_t v = gp_t0;` is a Line that ends with a Scope). The printer writes them that way,
// and owns the rest of the layout, and nothing else:
//   - indentation (4 spaces per level; namespaces and the cases of a switch don't indent);
//   - #line directives, from the positions (above);
//   - the braces: `} else if (c) {`, `} else {` and `} break;` join a closing brace with what
//     follows it, and nested Scopes that end together close on one line, `} } }`;
//   - the parentheses that C++'s precedence needs: an operand that binds less tightly than
//     its place needs gets them (Prec), and a unary -, + or & before an operand that starts
//     with the same character gets them too (- -x is not --x). Parentheses that only make
//     the code easier to read, or quiet a compiler warning ((a & b) | c), are Paren nodes
//     that lowering adds.
//
// Everything else, the text of each name, literal and operator, is the tree's. So the printer
// has no choices that depend on Go, and lowering never writes text with newlines, indentation
// or #line (a newline in a node's text is a panic).
//
// One expression goes on over several lines: a Lambda whose body is not Inline (a func
// literal, step 3.11). Its statement's line ends with the lambda's opening brace, the body's
// statements follow, each with its own place (so a debugger steps through a closure's lines),
// the closing brace is a line at the body's End, and the rest of the statement, `);`, a line at
// the statement's place (or, for Lambda.TailAtEnd, after the brace on its line). A line can hold several such lambdas, one after the other, and a body can hold
// more of them.
//
// # Known limits, for later steps
//
//   - Struct.Name and Func.Name are plain text, so a template specialization is spelled in the
//     name (Box<int64_t>).
//   - There is no range-based for, and no structured binding (auto [a, b] = ...); Phase 02 and
//     the plan's Phase 03 shapes don't use them.
package cppir
