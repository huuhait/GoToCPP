// Package libgen writes what a library needs besides the packages' own C++ (Phase 06 step 6.10,
// docs/spec/build.md "Libraries"): for the build modes c-archive and c-shared the C header and the
// extern "C" functions of the `//export` directives, with cgo's rules for the types that may cross
// the boundary; for cpp-lib the C++ header and the CMake files of the installed tree.
package libgen

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
)

// An Export is a function that a `//export Name` directive makes a C function.
type Export struct {
	Name string // the C name: the function's own name
	Pkg  *check.Package
	Decl *ast.FuncDecl
	Pos  token.Position // of the directive: messages and #line
	Doc  string         // the Go doc comment, without the directive

	Params  []Field // the C parameters, in order
	Results []Field // the C results: none, one, or several (the C function returns a struct)
}

// A Field is a parameter or a result of an exported function.
type Field struct {
	Name string // the Go name, or "" (the header uses pN or rN then)
	C    string // the C type, as the header writes it: GoInt32, GoString, char*
}

// An Error is a mistake in an export, at its position.
type Error struct {
	Pkg *check.Package
	Pos token.Position
	Msg string
}

func (e *Error) Error() string { return e.Pos.String() + ": " + e.Msg }

// directive is the comment that makes a function a C function, the same as cgo's.
const directive = "//export"

// isDirective reports whether the comment text is an `//export` line: "//export" and a space or
// tab (cgo's rule; "//exports" or "//export" alone are ordinary comments).
func isDirective(text string) bool {
	rest, ok := strings.CutPrefix(text, directive)
	return ok && rest != "" && (rest[0] == ' ' || rest[0] == '\t')
}

// Collect finds the exported functions of the packages and checks them: it returns the exports in
// the order of the packages and, in a package, of the source, and the errors, in the same order. A
// package that has no `//export` has none.
func Collect(fset *token.FileSet, pkgs []*check.Package) ([]*Export, []*Error) {
	var exports []*Export
	var errs []*Error
	byName := make(map[string]*Export)
	for _, pkg := range pkgs {
		for _, f := range pkg.Load.Files {
			if f.AST == nil {
				continue
			}
			c := &collector{fset: fset, pkg: pkg, file: f.AST}
			es, es2 := c.file1()
			errs = append(errs, es2...)
			for _, e := range es {
				if prev := byName[e.Name]; prev != nil {
					errs = append(errs, &Error{Pkg: pkg, Pos: e.Pos, Msg: fmt.Sprintf("//export %s: the name is exported twice (the other is at %s)", e.Name, prev.Pos)})
					continue
				}
				byName[e.Name] = e
				exports = append(exports, e)
			}
		}
	}
	return exports, errs
}

type collector struct {
	fset *token.FileSet
	pkg  *check.Package
	file *ast.File
}

// file1 returns the exports of one file and its errors.
func (c *collector) file1() (exports []*Export, errs []*Error) {
	owned := make(map[*ast.Comment]bool)
	for _, d := range c.file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Doc == nil {
			continue
		}
		for _, cm := range fd.Doc.List {
			if !isDirective(cm.Text) {
				continue
			}
			owned[cm] = true
			e, err := c.export(fd, cm)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			exports = append(exports, e)
		}
	}
	// A directive that is not in the doc comment of a function does nothing in Go: an error here,
	// because the usual cause is a blank line between it and the function.
	for _, g := range c.file.Comments {
		for _, cm := range g.List {
			if isDirective(cm.Text) && !owned[cm] {
				errs = append(errs, c.errAt(cm.Slash, "//export must be in the doc comment of a function (with no blank line between them)"))
			}
		}
	}
	slices.SortFunc(errs, func(a, b *Error) int { return a.Pos.Offset - b.Pos.Offset })
	return exports, errs
}

func (c *collector) errAt(pos token.Pos, format string, args ...any) *Error {
	return &Error{Pkg: c.pkg, Pos: c.fset.Position(pos), Msg: fmt.Sprintf(format, args...)}
}

// cKeywords are the words that a C function, parameter or result can't be named (C17 and the usual
// extensions); the header and the glue would not compile.
var cKeywords = map[string]bool{
	"auto": true, "break": true, "case": true, "char": true, "const": true, "continue": true, "default": true,
	"do": true, "double": true, "else": true, "enum": true, "extern": true, "float": true, "for": true,
	"goto": true, "if": true, "inline": true, "int": true, "long": true, "register": true, "restrict": true,
	"return": true, "short": true, "signed": true, "sizeof": true, "static": true, "struct": true,
	"switch": true, "typedef": true, "union": true, "unsigned": true, "void": true, "volatile": true,
	"while": true, "bool": true, "true": true, "false": true, "asm": true, "typeof": true,
	"alignas": true, "alignof": true, "nullptr": true, "class": true, "namespace": true, "new": true,
	"delete": true, "template": true, "this": true, "throw": true, "try": true, "catch": true,
	"public": true, "private": true, "protected": true, "virtual": true, "operator": true,
	"main": true, "NULL": true, "EOF": true, "std": true, "gp": true, "gpkg": true, "gocpp": true,
}

// libcNames are names of the C library (and POSIX) that the glue's headers declare: an exported
// function with one of them would clash with the declaration (or silently replace the library's
// function inside the process), so the build rejects them instead of failing in the C++ compiler.
var libcNames = map[string]bool{
	"abort": true, "abs": true, "accept": true, "access": true, "acos": true, "acosh": true, "alarm": true,
	"asctime": true, "asin": true, "asinh": true, "atan": true, "atan2": true, "atanh": true, "atexit": true,
	"atof": true, "atoi": true, "atol": true, "atoll": true, "basename": true, "bind": true, "bsearch": true,
	"btowc": true, "calloc": true, "cbrt": true, "ceil": true, "chdir": true, "chmod": true, "chown": true,
	"clearerr": true, "clock": true, "clock_gettime": true, "close": true, "closedir": true, "connect": true,
	"copysign": true, "cos": true, "cosh": true, "creat": true, "ctime": true, "dirname": true, "div": true,
	"dlclose": true, "dlerror": true, "dlopen": true, "dlsym": true, "dup": true, "dup2": true, "environ": true,
	"erf": true, "erfc": true, "errno": true, "exec": true, "execl": true, "execle": true, "execlp": true,
	"execv": true, "execve": true, "execvp": true, "exit": true, "exp": true, "exp2": true, "expm1": true,
	"fabs": true, "fclose": true, "fcntl": true, "fdopen": true, "feof": true, "ferror": true, "fflush": true,
	"fgetc": true, "fgetpos": true, "fgets": true, "fileno": true, "floor": true, "fma": true, "fmax": true,
	"fmin": true, "fmod": true, "fopen": true, "fork": true, "fprintf": true, "fputc": true, "fputs": true,
	"fread": true, "free": true, "freopen": true, "frexp": true, "fscanf": true, "fseek": true, "fsetpos": true,
	"fstat": true, "ftell": true, "fwrite": true, "getc": true, "getchar": true, "getcwd": true, "getenv": true,
	"getgid": true, "getpid": true, "getppid": true, "getrlimit": true, "gets": true, "gettimeofday": true,
	"getuid": true, "gmtime": true, "hypot": true, "ilogb": true, "index": true, "isalnum": true,
	"isalpha": true, "isatty": true, "isdigit": true, "islower": true, "isprint": true, "ispunct": true,
	"isspace": true, "isupper": true, "kill": true, "labs": true, "ldexp": true, "ldiv": true, "lgamma": true,
	"link": true, "listen": true, "llabs": true, "lldiv": true, "llrint": true, "llround": true,
	"localtime": true, "log": true, "log10": true, "log1p": true, "log2": true, "logb": true, "longjmp": true,
	"lrint": true, "lround": true, "lseek": true, "lstat": true, "malloc": true, "mblen": true, "memchr": true,
	"memcmp": true, "memcpy": true, "memmove": true, "memset": true, "mkdir": true, "mkfifo": true,
	"mktime": true, "mmap": true, "modf": true, "mprotect": true, "munmap": true, "nan": true,
	"nanosleep": true, "nearbyint": true, "nextafter": true, "open": true, "opendir": true, "optarg": true,
	"optind": true, "pause": true, "perror": true, "pipe": true, "poll": true, "pow": true, "printf": true,
	"pthread_create": true, "pthread_exit": true, "pthread_join": true, "pthread_self": true, "putc": true,
	"putchar": true, "puts": true, "qsort": true, "raise": true, "rand": true, "read": true, "readdir": true,
	"readlink": true, "realloc": true, "recv": true, "remainder": true, "remove": true, "rename": true,
	"rewind": true, "rindex": true, "rint": true, "rmdir": true, "round": true, "scanf": true,
	"sched_yield": true, "select": true, "send": true, "setbuf": true, "setenv": true, "setjmp": true,
	"setlocale": true, "setrlimit": true, "setvbuf": true, "shutdown": true, "signal": true, "sin": true,
	"sinh": true, "sleep": true, "snprintf": true, "socket": true, "sprintf": true, "sqrt": true, "srand": true,
	"sscanf": true, "stat": true, "stderr": true, "stdin": true, "stdout": true, "strcasecmp": true,
	"strcat": true, "strchr": true, "strcmp": true, "strcpy": true, "strcspn": true, "strdup": true,
	"strerror": true, "strftime": true, "strlen": true, "strncasecmp": true, "strncat": true, "strncmp": true,
	"strncpy": true, "strndup": true, "strpbrk": true, "strrchr": true, "strspn": true, "strstr": true,
	"strtod": true, "strtof": true, "strtok": true, "strtol": true, "strtold": true, "strtoll": true,
	"strtoul": true, "strtoull": true, "symlink": true, "syscall": true, "sysconf": true, "system": true,
	"tan": true, "tanh": true, "tgamma": true, "time": true, "tmpfile": true, "tmpnam": true, "tolower": true,
	"toupper": true, "trunc": true, "umask": true, "ungetc": true, "unlink": true, "unsetenv": true,
	"usleep": true, "vfprintf": true, "vprintf": true, "vsnprintf": true, "vsprintf": true, "wait": true,
	"waitpid": true, "write": true,
}

// CheckSymbols returns an error for each export whose name is a symbol that the C library defines
// (isLibcSymbol says): the library would define it twice or, in a shared library, silently replace the
// C library's function in the whole process. The names that libcNames lists are rejected before
// (Collect); this catches the others, which only the C library itself knows.
func CheckSymbols(exports []*Export, isLibcSymbol func(string) bool) []*Error {
	var errs []*Error
	for _, e := range exports {
		if isLibcSymbol(e.Name) {
			errs = append(errs, &Error{Pkg: e.Pkg, Pos: e.Pos, Msg: "the name " + e.Name + " is a symbol of the C library: the exported function would replace it in the process; choose another name"})
		}
	}
	return errs
}

// reservedNames are the names that the header declares itself.
var reservedNames = map[string]bool{
	"GoInt8": true, "GoUint8": true, "GoInt16": true, "GoUint16": true, "GoInt32": true, "GoUint32": true,
	"GoInt64": true, "GoUint64": true, "GoInt": true, "GoUint": true, "GoUintptr": true, "GoFloat32": true,
	"GoFloat64": true, "GoString": true, "GoSlice": true, "GoMap": true, "GoChan": true, "GoInterface": true,
}

// cName returns why name can't be the name of a C function that gocpp writes, or "".
func cName(name string) string {
	switch {
	case name == "" || name == "_":
		return "it has no name"
	case !isCIdent(name):
		return "it is not a C name (letters, digits and _ only, not starting with a digit)"
	case cKeywords[name]:
		return "it is a C or C++ keyword (or a namespace of the glue)"
	case reservedNames[name] || strings.HasPrefix(name, "_Go") || strings.HasPrefix(name, "gp_") || strings.HasPrefix(name, "gpkg_") ||
		strings.HasPrefix(name, "gocpp_") || strings.HasPrefix(name, "GOCPP_"):
		return "the name is one that gocpp's header uses"
	case libcNames[name] || strings.HasPrefix(name, "__") || len(name) > 1 && name[0] == '_' && 'A' <= name[1] && name[1] <= 'Z':
		return "the name is one of the C library or reserved for the C implementation: the library would clash with it"
	case strings.HasSuffix(name, "_return"):
		return "a name that ends in _return is the struct of another function's results"
	}
	return ""
}

func isCIdent(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_', 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z':
		case '0' <= c && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return s != ""
}

// export checks the function fd with the directive cm and returns its C signature.
func (c *collector) export(fd *ast.FuncDecl, cm *ast.Comment) (*Export, *Error) {
	args := strings.Fields(strings.TrimPrefix(cm.Text, directive))
	switch {
	case len(args) != 1:
		return nil, c.errAt(cm.Slash, "usage: //export Name (the name of the function)")
	case args[0] != fd.Name.Name:
		return nil, c.errAt(cm.Slash, "//export %s: the name must be the function's own: %s", args[0], fd.Name.Name)
	}
	name := fd.Name.Name
	fail := func(format string, a ...any) (*Export, *Error) {
		return nil, c.errAt(cm.Slash, "//export %s: %s", name, fmt.Sprintf(format, a...))
	}
	switch {
	case fd.Recv != nil:
		return fail("a method can't be exported: export a function that calls it")
	case fd.Type.TypeParams != nil:
		return fail("a generic function can't be exported: export a function that calls an instance")
	case fd.Body == nil:
		return fail("the function has no body")
	case name == "init":
		return fail("init can't be exported")
	}
	if why := cName(name); why != "" {
		return fail("%s", why)
	}
	e := &Export{Name: name, Pkg: c.pkg, Decl: fd, Pos: c.fset.Position(cm.Slash)}
	if fd.Doc != nil {
		var doc []string
		for _, l := range fd.Doc.List {
			if !isDirective(l.Text) {
				doc = append(doc, l.Text)
			}
		}
		e.Doc = docText(doc)
	}
	info := c.pkg.Info
	// The parameters and results as the AST names them: a name can be shared by several fields
	// (a, b int32), and the C types of C.int come from the AST (go/types has no type for them).
	cl := &classifier{info: info}
	i := 0
	for _, fld := range fd.Type.Params.List {
		if _, ok := fld.Type.(*ast.Ellipsis); ok {
			return fail("a variadic function can't be exported: take a slice")
		}
		n := max(len(fld.Names), 1)
		for k := 0; k < n; k++ {
			i++
			pname := ""
			if k < len(fld.Names) {
				pname = fld.Names[k].Name
			}
			ct, why := cl.typ(fld.Type)
			if why != "" {
				return fail("parameter %s (%s): %s", paramLabel(pname, i), c.typeText(fld.Type), why)
			}
			e.Params = append(e.Params, Field{Name: pname, C: ct})
		}
	}
	if fd.Type.Results != nil {
		i = 0
		for _, fld := range fd.Type.Results.List {
			n := max(len(fld.Names), 1)
			for k := 0; k < n; k++ {
				i++
				rname := ""
				if k < len(fld.Names) {
					rname = fld.Names[k].Name
				}
				ct, why := cl.typ(fld.Type)
				if why != "" {
					return fail("result %s (%s): %s", paramLabel(rname, i), c.typeText(fld.Type), why)
				}
				e.Results = append(e.Results, Field{Name: rname, C: ct})
			}
		}
	}
	if len(e.Results) > 16 {
		return fail("more than 16 results")
	}
	return e, nil
}

func paramLabel(name string, i int) string {
	if name != "" && name != "_" {
		return name
	}
	return fmt.Sprintf("#%d", i)
}

// typeText prints the type expression as written (for messages).
func (c *collector) typeText(e ast.Expr) string {
	var b strings.Builder
	writeExpr(&b, e)
	return b.String()
}

func writeExpr(b *strings.Builder, e ast.Expr) {
	switch x := e.(type) {
	case *ast.Ident:
		b.WriteString(x.Name)
	case *ast.SelectorExpr:
		writeExpr(b, x.X)
		b.WriteByte('.')
		b.WriteString(x.Sel.Name)
	case *ast.StarExpr:
		b.WriteByte('*')
		writeExpr(b, x.X)
	case *ast.ParenExpr:
		b.WriteByte('(')
		writeExpr(b, x.X)
		b.WriteByte(')')
	case *ast.ArrayType:
		b.WriteByte('[')
		if x.Len != nil {
			writeExpr(b, x.Len)
		}
		b.WriteByte(']')
		writeExpr(b, x.Elt)
	case *ast.MapType:
		b.WriteString("map[")
		writeExpr(b, x.Key)
		b.WriteByte(']')
		writeExpr(b, x.Value)
	case *ast.ChanType:
		b.WriteString("chan ")
		writeExpr(b, x.Value)
	case *ast.FuncType:
		b.WriteString("func(...)")
	case *ast.InterfaceType:
		b.WriteString("interface{...}")
	case *ast.StructType:
		b.WriteString("struct{...}")
	case *ast.BasicLit:
		b.WriteString(x.Value)
	case *ast.IndexExpr:
		writeExpr(b, x.X)
		b.WriteString("[...]")
	default:
		b.WriteString("?")
	}
}

// docText joins the lines of a doc comment, for a C comment: the // and one space are cut, and a
// "*/" can't end the comment early.
func docText(lines []string) string {
	var out []string
	for _, l := range lines {
		l = strings.TrimPrefix(l, "//")
		l = strings.TrimPrefix(l, " ")
		out = append(out, commentSafe(l))
	}
	return strings.Join(out, "\n")
}

// commentSafe makes a line of text safe inside a C comment (/* ... */) under -Wall -Wextra and
// -trigraphs: no "*/" (ends it), no "/*" (-Wcomment), no "??" (a trigraph such as ??/ is a
// backslash), and no backslash at the end of the line (it joins the next line).
func commentSafe(l string) string {
	l = strings.ReplaceAll(l, "*/", "* /")
	l = strings.ReplaceAll(l, "/*", "/ *")
	for strings.Contains(l, "??") { // "???/" needs two rounds
		l = strings.ReplaceAll(l, "??", "? ?")
	}
	if strings.HasSuffix(l, "\\") {
		l += " "
	}
	return l
}

// A classifier maps the types of an exported signature to C types (cgo's rules, with copies at
// the boundary: lib.hpp).
type classifier struct {
	info *types.Info
}

// cTypes are the C names, as the header writes them (the types of C: docs/spec/interop.md "What C
// holds").
var cTypes = map[string]string{
	"char": "char", "schar": "signed char", "uchar": "unsigned char", "short": "short", "ushort": "unsigned short",
	"int": "int", "uint": "unsigned int", "long": "long", "ulong": "unsigned long", "longlong": "long long",
	"ulonglong": "unsigned long long", "float": "float", "double": "double", "size_t": "size_t",
}

// typ returns the C type of the type expression e, or why it can't cross the boundary.
func (c *classifier) typ(e ast.Expr) (ctype, why string) {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return c.typ(x.X)
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok && id.Name == "C" {
			if t, ok := cTypes[x.Sel.Name]; ok {
				return t, ""
			}
			return "", "this C type is not one that an exported function can have: use C.int, C.char, ... or a pointer"
		}
	case *ast.StarExpr:
		return c.pointer(x.X)
	case *ast.ArrayType:
		if x.Len == nil {
			return c.slice(x.Elt)
		}
		return "", "an array can't cross the boundary: use a slice, or a pointer and a length"
	case *ast.MapType:
		return "", "a map can't cross the boundary: copy it into a slice of keys and a slice of values"
	case *ast.ChanType:
		return "", "a channel can't cross the boundary"
	case *ast.FuncType:
		return "", "a func can't cross the boundary: C code can't call a Go func value (pass a C function pointer type, or call a Go function by its exported name)"
	case *ast.InterfaceType:
		return "", "an interface can't cross the boundary"
	case *ast.StructType:
		return "", "a struct can't cross the boundary: use a pointer to a C struct, or pass the fields"
	}
	t := c.info.TypeOf(e)
	if t == nil {
		return "", "its type is not known"
	}
	return c.goType(t, true)
}

// goType is the C type of the type t. pointee says that t is not behind a pointer or in a slice,
// so a string or a slice is allowed.
func (c *classifier) goType(t types.Type, value bool) (ctype, why string) {
	if t == types.Typ[types.Invalid] {
		return "", "its type comes from C or C++ and is not one of C's own types: use C.int, C.char, ... or unsafe.Pointer"
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch u.Kind() {
		case types.Bool:
			return "GoUint8", ""
		case types.Int8:
			return "GoInt8", ""
		case types.Int16:
			return "GoInt16", ""
		case types.Int32:
			return "GoInt32", ""
		case types.Int64:
			return "GoInt64", ""
		case types.Int:
			return "GoInt", ""
		case types.Uint8:
			return "GoUint8", ""
		case types.Uint16:
			return "GoUint16", ""
		case types.Uint32:
			return "GoUint32", ""
		case types.Uint64:
			return "GoUint64", ""
		case types.Uint:
			return "GoUint", ""
		case types.Uintptr:
			return "GoUintptr", ""
		case types.Float32:
			return "GoFloat32", ""
		case types.Float64:
			return "GoFloat64", ""
		case types.String:
			if !value {
				return "", "a string can only be a parameter or a result itself, not an element or behind a pointer: the copy at the boundary would not reach the caller"
			}
			return "GoString", ""
		case types.UnsafePointer:
			return "void*", ""
		case types.Complex64, types.Complex128:
			return "", "complex numbers are not supported: use two floats"
		}
		return "", "this type can't cross the boundary"
	case *types.Pointer:
		return c.pointee(u.Elem())
	case *types.Slice:
		if !value {
			return "", "a slice can only be a parameter or a result itself, not an element or behind a pointer"
		}
		return c.sliceOf(u.Elem())
	case *types.Struct:
		return "", "a struct can't cross the boundary: use a pointer to a C struct, or pass the fields"
	case *types.Array:
		return "", "an array can't cross the boundary: use a slice, or a pointer and a length"
	case *types.Map:
		return "", "a map can't cross the boundary: copy it into a slice of keys and a slice of values"
	case *types.Chan:
		return "", "a channel can't cross the boundary"
	case *types.Signature:
		return "", "a func can't cross the boundary: C code can't call a Go func value"
	case *types.Interface:
		if types.Identical(t, types.Universe.Lookup("error").Type()) {
			return "", "an error can't cross the boundary: return an error code (an int) or the message (a string)"
		}
		return "", "an interface can't cross the boundary"
	}
	return "", "this type can't cross the boundary"
}

// pointer is the C type of a pointer to the type expression e.
func (c *classifier) pointer(e ast.Expr) (string, string) {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return c.pointer(x.X)
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok && id.Name == "C" {
			t, ok := cTypes[x.Sel.Name]
			if !ok {
				return "", "this C type is not one that an exported function can have"
			}
			return t + "*", ""
		}
	case *ast.StarExpr:
		inner, why := c.pointer(x.X)
		if why != "" {
			return "", why
		}
		return inner + "*", ""
	}
	t := c.info.TypeOf(e)
	if t == nil {
		return "", "its type is not known"
	}
	return c.pointeeType(t)
}

// pointee is the C type of a pointer to the Go type t that go/types has: pointers to numbers, to
// bool and to unsafe.Pointer are C pointers to them; a pointer to a Go struct or to anything with
// gocpp's own layout (a string, a slice) is not.
func (c *classifier) pointee(t types.Type) (string, string) {
	return c.pointeeType(t)
}

func (c *classifier) pointeeType(t types.Type) (string, string) {
	if t == types.Typ[types.Invalid] {
		return "", "its type comes from C or C++ and is not one of C's own types: use C.int, C.char, ... or unsafe.Pointer"
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		if u.Kind() == types.String || u.Kind() == types.Complex64 || u.Kind() == types.Complex128 {
			break
		}
		inner, why := c.goType(t, false)
		if why != "" {
			return "", why
		}
		return inner + "*", ""
	case *types.Pointer:
		inner, why := c.pointeeType(u.Elem())
		if why != "" {
			return "", why
		}
		return inner + "*", ""
	}
	return "", "a pointer to a Go type with gocpp's own layout (a struct, a string, a slice, ...) can't cross the boundary: gocpp frees its memory by reference counting, so C could not keep it. Use unsafe.Pointer, or a pointer to a number or a C type"
}

// slice is the C type of a slice whose element is the type expression e.
func (c *classifier) slice(e ast.Expr) (string, string) {
	if sel, ok := e.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "C" {
			if _, ok := cTypes[sel.Sel.Name]; ok {
				return "GoSlice", ""
			}
			return "", "this C type is not one that an exported function can have"
		}
	}
	t := c.info.TypeOf(e)
	if t == nil {
		return "", "its type is not known"
	}
	return c.sliceOf(t)
}

// sliceOf is GoSlice when the elements of a slice are plain: numbers and bool (and the named types
// over them), which are copied as bytes.
func (c *classifier) sliceOf(elem types.Type) (string, string) {
	if elem == types.Typ[types.Invalid] {
		return "", "its element type comes from C or C++: use C.int, C.char, ..."
	}
	if b, ok := elem.Underlying().(*types.Basic); ok && b.Info()&(types.IsNumeric|types.IsBoolean) != 0 && b.Info()&types.IsComplex == 0 {
		return "GoSlice", ""
	}
	return "", "only a slice of numbers or bools can cross the boundary (its elements are copied as bytes): use a slice of numbers and the lengths"
}
