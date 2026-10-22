package lower

import "strings"

// The "keeps the pointer" rules of typed interop (Phase 12 step 12.6, README §2.5, ADR-087).
//
// Typed mode passes a Go string to a `const char*` parameter by itself: gp::cstr(s).gp_ptr(), a
// NUL-terminated copy of the bytes (or the bytes themselves when a NUL follows them already, as a
// literal's) that lives until the end of the full expression, the statement of the call. That is
// right for every function that only reads the string during the call, which is nearly all of C.
// It is wrong, and the program reads freed memory later, for a function that keeps the pointer or
// gives one back that points into it:
//
//	raylib.InitWindow(w, h, title)                          raylib keeps the title
//	sqlite3.sqlite3_bind_text(st, 1, s, n, SQLITE_STATIC)   SQLite reads s when the statement runs
//	sqlite3.sqlite3_result_text(ctx, s, n, SQLITE_STATIC)   the same, for a function's result
//	cstdlib.strtol(s, &end, 10)                             end points into s
//	getopt(argc, argv, optstring)                           getopt keeps the option string
//
// For these the conversion is a user error with a hint (C.CString(s), which C owns until C.free, or
// C.cstr(s) as a variable that holds it), as in opaque mode where the user always wrote one of
// them. What is "keeping" comes from
//
//  1. an explicit list of functions of the C library and of the libraries of the zoo (keeperFuncs);
//  2. the arguments that decide it (destructorArg, curl): a destructor that is not
//     SQLITE_TRANSIENT, the libcurl option CURLOPT_POSTFIELDS;
//  3. any other argument of the call that is the address of a char pointer (charPtrOutParam,
//     typedargs.go): strtol(s, &end, 10), sqlite3_prepare_v2(db, sql, -1, &st, &tail), where the
//     pointer that the function stores can point into the string;
//  4. conventions in the name (a name with static, borrow, nocopy, noalloc or unowned in it);
//
// and a function that is not covered (a C++ method or a function of the program's own that stores
// its const char* parameter) is trusted to copy or to read: there is nothing in its declaration
// that says otherwise. The functions that take a `char*` (putenv, strtok, setvbuf, setbuf, strsep)
// are not on the list because a Go string never converts to `char*`: the parameter is not a
// candidate, and C++ rejects the call, as opaque mode does. `gocpp vet` (Phase 13) takes the same
// rules for its warning about strings with a NUL byte in them, which the C string cuts short
// (gp::cstr, as C.CString, ends the C string at the first NUL).

// keeperFuncs are the functions whose const char* parameters the callee keeps (the last name of the
// function: std::foo, ::foo and lib.foo are foo). Value: why.
var keeperFuncs = map[string]string{
	"openlog":          "the logger keeps the identifier",
	"getopt":           "getopt keeps the option string",
	"getopt_long":      "getopt keeps the option string",
	"getopt_long_only": "getopt keeps the option string",
	"InitWindow":       "raylib keeps the title (it makes the window again from it)",
	"SetWindowTitle":   "raylib keeps the title",
}

// aliasFuncs are the functions whose result points into a string argument: the result is valid
// while the argument is, which for a converted Go string is the statement.
var aliasFuncs = map[string]bool{
	"strstr": true, "strchr": true, "strrchr": true, "strpbrk": true, "strcasestr": true,
	"index": true, "rindex": true, "basename": true, "dirname": true, "memchr": true,
	"memmem": true, "strchrnul": true, "strptime": true,
	"wcschr": true, "wcsrchr": true, "wcsstr": true, "wcspbrk": true, "wmemchr": true,
}

// destructorArg is the index of the destructor (SQLITE_STATIC, SQLITE_TRANSIENT or a function)
// of the SQLite functions that are given bytes: with anything but SQLITE_TRANSIENT, SQLite reads
// them later.
var destructorArg = map[string]int{
	"sqlite3_bind_text": 4, "sqlite3_bind_text16": 4, "sqlite3_bind_text64": 4,
	"sqlite3_bind_blob": 4, "sqlite3_bind_blob64": 4,
	"sqlite3_result_text": 3, "sqlite3_result_text16": 3, "sqlite3_result_text16le": 3,
	"sqlite3_result_text16be": 3, "sqlite3_result_text64": 3,
	"sqlite3_result_blob": 3, "sqlite3_result_blob64": 3,
}

// conventionWords are the words in a function's name that say that it keeps its pointers.
var conventionWords = []string{"static", "borrow", "nocopy", "noalloc", "unowned"}

// keeperConvention reports whether the name of the function says that it keeps its pointers.
func keeperConvention(name string) bool {
	l := strings.ToLower(name)
	for _, w := range conventionWords {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

// keepsPointer reports whether the call of the function name, whose argument number n is
// text(n) as written, lets the callee keep the pointer of a string argument, and why.
func keepsPointer(name string, nargs int, text func(n int) string) (string, bool) {
	last := lastName(name)
	if why, ok := keeperFuncs[last]; ok {
		return why, true
	}
	if d, ok := destructorArg[last]; ok {
		if d < nargs && !strings.HasSuffix(text(d), "SQLITE_TRANSIENT") {
			return "SQLite reads the bytes later, unless the destructor is SQLITE_TRANSIENT (it copies them)", true
		}
		return "", false
	}
	if last == "curl_easy_setopt" {
		if nargs > 1 && strings.HasSuffix(text(1), "CURLOPT_POSTFIELDS") {
			return "libcurl keeps the pointer of CURLOPT_POSTFIELDS (CURLOPT_COPYPOSTFIELDS copies)", true
		}
		return "", false
	}
	if keeperConvention(last) {
		return "its name says that it keeps the pointer", true
	}
	return "", false
}
