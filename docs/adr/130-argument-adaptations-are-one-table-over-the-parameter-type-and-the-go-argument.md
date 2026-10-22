# ADR-130: Argument adaptations are one table over the parameter type and the Go argument, and the C number types are Go's

Status: accepted (2026-10-21; user order "go make it", and the user decision of the same day on the numeric types)

## Context

In typed interop (Phase 12; the default when clang++ is installed, ADR-131) clang says the exact type of every parameter of a C/C++ function. A
program that calls `ImGui::SliderFloat`, `IDXGISwapChain::GetBuffer`, `CreateWindowExW` or `glfwGetFramebufferSize`
still had to write the conversions that those types make obvious: `C.cstr(s)` for text, `C.wstr(s)` for wide text,
`var raw unsafe.Pointer` and a cast for a `void**` that a COM call fills, `var width C.int` for an `int*`,
`C.float(x)` at every boundary. The user's order: "since clang knows the parameter's type, the compiler should insert the
safe conversions from the types alone", in **one table of argument adaptations** and not as a case here and a case there.
A second decision of the same day (numeric types, below) makes the basic C number types **the** Go fixed-size types.

This ADR is the design (soundness, the table, what is not adapted, lifetimes, modes, messages) and the record of what is
built. [ADR-087](087-typed-interop-code-generation.md) is the code generation of step 12.6.

## What existed before this ADR, and what is new

Typed mode (step 12.6) and C++ itself already did:

| Already there | By |
|---|---|
| a runtime `string` to a `const char*` parameter: `gp::cstr(s).gp_ptr()`, a NUL-terminated copy for the statement; the keeps list (`InitWindow`, `sqlite3_bind_text`, ...) and the "gives back a pointer into the string" rule (`strtol(s, &end, 10)`, `strstr`) | 12.6, `keeps.go` |
| a `string` to a `std::string`, `const std::string&`, `&&` parameter, also a map key | 12.6 |
| a pointer to a C type, a `unsafe.Pointer` or a gocpp pointer to a `void*` or `const void*` parameter (an `HWND` too) | C++ and `gp::ptr`'s conversion |
| `&b` of a Go `bool` to a `bool*` | same C++ type |
| `&x` of a variable of a C type (`var speed C.float`, `&speed`) to a pointer or reference out-parameter | `gp::cgo::out_ptr`, `out_cptr`, `out_pointer` (a destructor copies back at the end of the statement) |
| a Go function to a C function pointer | callbacks (Phase 04) |
| a C/C++ number into a Go number and the other way, by C++'s rules, with `gp::cgo::as<T>` | 12.6 ("compatibility rule") |
| a number by value to a number parameter (`int64` to `int`) | C++'s implicit conversion |

New in this ADR:

| New | Row |
|---|---|
| a `string` (also a constant) to `const wchar_t*`, `const char16_t*`, `const char32_t*` (`LPCWSTR`) | `wide` |
| `&x` of a Go number variable or field to `T*` / `const T*`, `T` another C number type (`&n` of a Go `int` to `int*`; Windows `uint32` to `unsigned long*`) | `scalarptr` |
| `&p`, `p` a `*lib.T`, to `void**` (`GetBuffer` with a `*d3d11.ID3D11Texture2D`) | `voidout` |
| an untyped integer constant to an enum parameter | `enum` |
| an untyped number constant to a number parameter: a range check by Go's rule | `numconst` |
| a Go float to an integer parameter (and `float64` to `float`): Go's conversion | `floatconv` |
| a runtime string at the format place of a printf-like function: an error | the guard of `cstr` |
| `C.int` is `int32`, `C.float` is `float32`, `C.bool` is `bool`, ... | the identity of the types |

The old rows (`cstr`, `stdstring`) were moved into the table, unchanged in behavior (the keeps rules are the lowering
of the `cstr` row).

## Decision

### The table

An argument adaptation is a row: **(class of the Go argument) x (what clang says the parameter is) -> (what the lowering
writes)**. One list (`adaptTable` in `internal/lower/typedadapt.go`); each row has the argument classes it handles,
a function `param` that says for one parameter type "adapts (with this detail)", "can never take this argument" or "unknown",
and the lowering. The classes of the Go argument are decided from the Go types alone.

| Row | Go argument | Parameter (clang's type) | Written | Soundness |
|---|---|---|---|---|
| `cstr` | a string value, not a constant | `const char*` | `gp::cstr(s).gp_ptr()` | C++ has no conversion from `gp::string` to `const char*`, so the unadapted call is ill-formed; the temporary has the parameter's type |
| `stdstring` | a string value | `std::string` (value, `const&`, `&&`) | `::std::string(s)` | same, `std::string` has no constructor from `gp::string` |
| `wide` | a string value or a constant | `const wchar_t*`, `const char16_t*`, `const char32_t*` | `gp::wide_cstr<W>(s).gp_ptr()` | no conversion from a string, a literal `"x"` is `const char*` (another type), so an unadapted call never compiled; the units are UTF-16 where `W` has 16 bits, else UTF-32 (ADR-082's rules) |
| `scalarptr` | `&x`, `x` a variable (or field) of a Go number type | `T*`, `const T*`, `T` an arithmetic C type that is not `x`'s own C++ type and not a character type | `gp::cgo::scalar_arg<T>(&x, "x")` or `scalar_in<T>` | `x`'s address is a `gp::ptr<long>`, which is not convertible to `int*`, so the unadapted call never compiled. See the rules below |
| `voidout` | `&p`, `p` a variable (or field) of type `*lib.T` | `void**` | `gp::cgo::void_out(p)` | `T**` does not convert to `void**` in C++ (the classic unsound conversion); a temporary `void*` that is stored into `p` when the call ends is the sound form |
| `enum` | an untyped integer constant | an enum or `enum class` | `static_cast<E>(3)` | an integer has no implicit conversion to an enum; the cast is defined only for the values C++ defines, so the lowering checks the range: an `enum class` or an enum with a fixed type: the range of the underlying type; another enum: the smallest bit-field that holds every enumerator ([dcl.enum]); outside it is an error |
| `numconst` | an untyped number constant | a C number type by value | the argument itself | not an adaptation but Go's check: the value must be representable in the parameter's type (`300` for a `char`, `-1` for `unsigned`, `2.5` for `int`, `1e60` for `float` are errors, as in cgo); C++ would wrap or truncate silently |
| `floatconv` | a `float32` or `float64` value | an integer type, or `float` from a `float64` | `gp::cgo::as<T>(x)` | C++'s float to integer conversion is undefined when the value is out of range; Go's conversion (`as<T>`) is defined for every value |

**When a row applies (the one rule that makes all of them sound).** An argument is adapted only if **every** overload
that the call can reach (an overload with enough parameters, or a variadic one) has the **same row, with the same
detail**, at that place, or never takes this class of argument at all ("ignore"). Then, for each reachable overload,
the unadapted call was ill-formed (no conversion exists in C++), and the adapted argument has **exactly** the parameter's
type, so overload resolution has one viable function for the argument: the one the user meant. If one overload is
unknown (a class that might have a constructor, a `std::string_view`, a template, a parameter pack), or two overloads
adapt differently (`f(int*)` next to `f(long long*)`), nothing is adapted and the call is C++'s. A callee whose
signature is not known (a template, a macro, a function value) is not adapted. A call with `args...` is not adapted.

### Rules of `scalarptr` (and why this one is conservative)

A Go number variable's address is a pointer to the variable's own bytes. A C parameter `int*` reads or writes four bytes
at the pointer; the variable is eight. Passing the address as is would be wrong (the callee would write half the variable);
passing a temporary is right if the value goes in and out unchanged:

- **Only `&x` of a variable or field** of a Go number type (`int`, `uint8`, `float32`, ...): never `&a[i]`, a slice
  element or a pointer value. The address of an element is the start of an array to a C function; one temporary would give
  the callee one element.
- **Not for a parameter that is spelled as an array** (`float col[3]`, `int v[2]`, `const int v[3]`, `void* p[2]`, and a
  parameter written with the typedef of an array: `typedef float vec3[3]; void scale(vec3 v)`, `typedef float mat4[4][4]`). Its type is
  `float*` everywhere in clang, but the callee reads three floats, and a temporary has one. The probe reads the source
  spelling of the parameter (`probe.Overload.ArrayParams`, `Param.Array`) and the row says "other" for it, so the call is
  C++'s: `&speed` of a `float32` is then an error that C++ words, and the way is the element of an array
  (`&col[0]` of a `[3]float32`, which is a `float*` already). Where the spelling is not known (a macro-expanded
  parameter, a declaration the probe could not locate) the parameter counts as an array: the adaptation is skipped, never
  wrongly applied. A typedef of an array leaves no trace in the type (`vec3 v` is `float *` to the compiler, which decays it),
  so the probe reads the **text** of the parameter between its start and its name, from the file that the dump names (only
  files of the run), and a parameter is a pointer only if that text has a `*` (comments removed). A typedef of a pointer
  (`LPDWORD p`, `typedef int* IntPtr`) keeps its name in the type that the compiler prints (`'LPDWORD':'unsigned long *'`), and
  that is a pointer without looking at the text; a decayed array prints as `'unsigned long *'` with no name. A file that
  cannot be read, or a parameter inside a macro, counts as an array (the probe cache key is v5 for this). The
  spelling is asked only for the functions that some call gives an `&x` or a Go pointer
  (`probe.AddrFuncNames`), in the same compiler run as the overload sets.
- **No character type, no `bool` as the pointee** (`char*`, `unsigned char*`, `bool*`): a `char*` is nearly always a buffer
  or a string; `bool*` is the same type as Go's (nothing to adapt).
- **A non-const `T*`:** the callee may write, so the copy back must be lossless: every value of `T` must be a value of
  `x`'s type (`int` into `int64`: yes; `unsigned long*` of a `uint32` on Linux: **no**, an error that says to declare `x`
  with the C type). The copy-in is checked: a value of `x` that does not fit `T` stops the program with
  `runtime error: the variable x holds V, which does not fit the C type T of the parameter it is passed to`. The copy-in check
  is made for a pure out-parameter too, which never reads the value: gocpp cannot know that the callee ignores it, and the
  check is what keeps the copy back lossless for the callee that both reads and writes (`*p += 1`). The cost is one
  comparison; a program that wants a pure out-parameter for a value that does not fit declares the variable with the C type.
- **A `const T*`:** copy-in only, with the same check; a float wider than `T` is an error (the copy would round).
- **Integer and float never mix** through a pointer (C converts neither way): an error that says to declare the variable
  with the C type.
- **When the copy happens.** The value goes in when the call starts and what the callee stored is in the variable **as soon as the
  call has returned**, not at the end of the statement: `if h.get5(&n) == 0 && n > 0`, a `for` condition `next(&c) < 3 && c < 10`,
  two calls in one expression, and `f(&n, n)` (the arguments are read before the call, as in Go) all see what Go would see. A
  non-const `scalarptr` and a `voidout` are written as
  `gp::cgo::call_out<int>(&x, "x", [&](int* gp_out0) -> decltype(auto) { return ::f(gp_out0); })` and
  `gp::cgo::call_void_out(p, [&](void** gp_out0) -> decltype(auto) { return ::f(gp_out0); })`: the helper makes the
  temporary, calls the lambda, copies back as it returns, and returns the callee's result unchanged (a `void` callee works).
  A `const T*` (copy-in only) stays `scalar_in`. The lambda captures by reference and is inlined; the C++ stays readable.
  The other out-parameters of 12.6 (`out_ptr` of a variable of a C type) still copy back at the end of the statement (the
  destructor of the temporary at the end of the full expression): that is still true, and they are not part of the table.
- **When the value is read.** Go evaluates all the arguments and then makes the call, and the callee reads the variable when it
  runs. So the copy-in is the last step before the call: an argument that has a call in it (`h.two(&a, &b, h.one(&a))`,
  `add_to(&n, setN())` with a `setN` that assigns `n`) is evaluated first, in order, into a local of a function around the helper
  (`outCall.early`, `evaluateEarly`), and the helper then reads `a`, `b` and `n`. An argument without a call stays in the call
  (a variable is read after the copy-in, and the copy-in does not change it). The local is `auto&&` and is forwarded, so
  references and temporaries are as they were; the pointer of a string holder (`gp::cstr(s).gp_ptr()`) is not moved into a local
  (its holder would die first) and stays in the call. Gap: the receiver of a method call (`f().g(&n)`) and the callee value are
  evaluated in the call, after the copy-in, unless the order pass moved them (it moves a call among several calls).
- **A callee that panics or throws** leaves the variable as the callee stored it: the copy back is in a destructor and runs
  while the exception passes, as a write through a pointer in Go is seen after a panic (`runtime/test/interop_test.cpp`: a
  callee that stores 77 and panics leaves the variable 77). A value that does not fit stops the program before the callee runs
  and writes nothing back.
- A deferred or spawned call is evaluated later and the variable might be gone: `voidout` is an error there.
- The lifetime rule below applies.

### Lifetimes, and the hazard that is documented, not detected

Every temporary a row makes (`gp::cstr`, `gp::wide_cstr`, the `scalar_arg` copy, the `void*` of `void_out`) lives
until the end of the statement that holds the call, **and no longer**. That is the rule, and it is the rule `C.cstr(s)`
always had. A callee that **keeps** the pointer after the call (a C library that stores a `const wchar_t*` title, a
function that remembers the `int*`) then reads a dead object. gocpp cannot see that from a declaration.

There is no deny list for the new rows: the user's decision is that the hazard is documented prominently (the
interop spec, the guide and this ADR) and that the existing keeps checks stay as they were for `char*`. They are shared by the
narrow and the wide row (`stringKeepsOK`, one set of rules over the character type): the names of the list (`InitWindow` and
friends, and the `wcs*` functions that give back a pointer into their argument: 18 names), a method that stores a pointer
parameter (the heuristic of 12.6) and a character pointer that the callee writes (`wchar_t*`, `GetWindowTextW`) are errors for
`wchar_t`, `char16_t` and `char32_t` as for `char` (`err_adapt_keeps_wide`, `err_adapt_wide_method`, `err_adapt_wide_outptr`,
`err_adapt_wide_alias`). The ways out are the ones that already exist: `C.CString`, `C.WString`,
`C.malloc`, a variable of the C type that lives as long as the callee needs it (then the argument is a pointer already and
no row applies). The `Win32` window class name in `demos/imgui-dx11` shows it: `WNDCLASSEXW.lpszClassName` keeps the pointer,
and the field assignment is not a call argument, so it stays a `C.WString`.

### Opaque mode is unchanged

Opaque mode (a machine without clang++, or `-interop=opaque`) has no parameter types, so it adapts nothing: the same calls are the C++ compiler's
errors at the `.gp` line, with the C++ message (`cannot convert 'const char*' to 'const wchar_t*'`, `invalid conversion
from 'int' to 'Mode'`, `invalid user-defined conversion from 'gp::cgo::out_cptr<Obj>' to 'void**'`, `cannot convert
'long int*' to 'int*'`). The generated code in opaque mode is byte-identical to before for programs that do not use
the type identity below. `tests/typed/adapt_*.gp` say `// opaque: reject`, and the harness checks that the opaque build fails.
`gocpp check` in typed mode does not warn about the shapes that the lowering adapts (the checker's rules mirror the
table: `adaptedArg` in `internal/typedinterop/icheck/messages.go`), and reports the ones the table refuses (ADR-134); in opaque mode it never had a view of them.

### Overloads, templates, variadics

- **Overloads:** the rule above. `f(const char*)` next to `f(std::string_view)` is left to C++ (a gocpp string converts to the
  `string_view` only); `f(int*)` next to `f(long long*)` with `&n` of an `int64` is a C++ error (`err_adapt_overloads.gp`).
- **Templates:** the signature of a template is not a fixed type; nothing is adapted, the call is C++'s (a `std::stoi`-like
  overload set of `string` and `wstring` is not a template and works: a wide-string overload is "never takes a narrow
  string").
- **Variadics:** the fixed parameters follow the rules; the arguments after `...` are not adapted, except a runtime string,
  which is a `char*` for `%s` (12.6, unchanged). A constant integer in `...` stays a C literal.

### The printf format string

A parameter that clang knows as a format string (`__attribute__((format(printf, N, M)))`: glibc's `printf`, ImGui's
`IM_FMTARGS`, a user's own) is **not** adapted from a runtime Go string: a `%` in its text would be a conversion, and
nothing in the types says whether that is meant (the format-string bug). Choices considered:

1. Convert silently (what 12.6 did for every `const char*`): risky, and a Go string with a `%` is common (`fmt.Sprintf`
   results, user text).
2. Escape the `%` automatically: a runtime string at a format place is then always literal text; but a program that builds
   a format at run time on purpose (`f(prefix + "%d")`) would print garbage without a word.
3. **An error** with the two ways out in the message (chosen): `f("%s", s)` for text, `C.cstr(s)` for a format that is meant.
   A constant string is a format as written (nothing changes for `printf("%d\n", n)`).

How the parameter is found: the attribute is not in the AST dump in a usable form (no format type, no index), so the probe asks
clang to compile a generated call of every variadic function with a `char*` parameter with a deliberately incomplete format
(`"%["`) at that parameter and reads the warning ("incomplete format specifier" is printf; "no closing ']'" is scanf).
The result is `probe.Overload.FormatKind` and `FormatParam`, cached with the declarations. Limits: the check is made for
free functions (`ImGui::Text` is a function in a namespace), not for methods (**a documented gap**: a printf-like method, `Logger.Log(fmt, ...)`, is not guarded and its string is
converted as in 12.6); the guard reports only when every overload that
the call reaches has the format at that place. The message is built from the call as written, in Go syntax: the callee as the user spelled it, the format argument replaced by
`"%s"` and the string inserted after it (`ImGui.Text("%s", label)` for `ImGui.Text(label)`; `err_adapt_printf_second` has the
format at the second place). `printf(s)` with `C.cstr(s)` compiles, and clang may warn (`-Wformat-security`)
about the non-literal format: that is the right warning for what the user wrote on purpose.

### The C number types are Go's fixed-size types (the user decision of 2026-10-21)

`C.int` is `int32`, `C.uint` is `uint32`, `C.longlong` is `int64`, `C.ulonglong` is `uint64`, `C.float` is `float32`,
`C.double` is `float64`, `C.bool` is `bool`, `C.char` and `C.schar` are `int8`, `C.uchar` is `uint8`, `C.short` is `int16`,
`C.ushort` is `uint16`; `C.long` and `C.ulong` follow the target (`int64` and `uint64` on Linux, `int32` and `uint32` on
Windows). **Identical types, not aliases with a conversion**: `var x C.int = n32` and `x + n32` need no conversion, a
function that takes `C.int` takes an `int32`, and `int32(x)`, `C.int(n32)` stay legal (cgo-source compatible: a
conversion between identical types). Go's `int`, `uint` and `uintptr` are **not** aliases (they are 64 bits and spell no C
type), nor is a named type over a fixed-size type (`type Count int32`), nor `C.size_t` and `C.wchar_t`.

How it is built: in gocpp the Go type checker has no C types (`go/types` gives every C name the invalid type); the rules of
cgo for C values are gocpp's own checks (`cguard.go`: `cOperandsOK`, `targetOK`, `gocppTargetOK`). The identity is a check
rule: where the C operand's declared type is a C builtin and the Go side is the **predeclared** type of the same kind on
the target (`cIdentical`, `internal/lower/cidentity.go`), the mismatch is not reported. It needs no clang, so it holds in
**both modes**; the generated C++ is unchanged, because the C++ types are already the same (`int` and `int32_t`) or have the
same size (`long long` and `int64_t`, which is `long` on Linux; the two convert by themselves). `C.bool` is added as a C
builtin (`bool`) with constants `true` and `false`.

What changes for the user, honestly:

- `fmt.Printf("%T", x)` of a `C.int` prints `int32` (it did already); of a `C.long` on Linux prints `int` (gocpp's `int` is
  also `long`: a limit of the representation, not of the rule).
- An interface that holds a `C.int` holds a C value with a descriptor of its C++ type: `v.(int32)` is false for it. Write
  `any(int32(x))`. A type switch with both `case C.int` and `case int32` cannot be told as a duplicate by `go/types`
  (it does not know `C.int`); the first case wins. Not changed.
- No methods are possible on either (a named type is needed: `type Count C.int`, which is its own type as in Go).
- The result of an operator on C values (`x*n`) is a C/C++ value of unknown type in opaque mode (promotions); `x*n + m` with
  `m int32` still says "mismatched types": convert `x*n`. Identity covers the declared types, not the results.
- Extern declarations (ADR-128) stay strict about Go `int`
  and `uint`: they are 64 bits and spell no C type. `C.int` is the way, and it now is `int32`.

### Numbers at a call: the whole table

What happens to a Go number passed by value to a C/C++ number parameter. "Opaque" is C++'s implicit conversion (no clang,
no parameter types; constants are checked only where Go itself knows the C type: a declaration, an assignment, an operand).
"Typed" is the table above. "vet" is the design; the warnings are **not built** (open item).

| Go argument to parameter | Opaque | Typed | `gocpp vet` |
|---|---|---|---|
| the same type (`int32` to `int`, `float32` to `float`, `bool` to `bool`) | silent | silent | silent |
| wider integer, same signedness; unsigned to a wider signed (`int32` to `long long`, `uint8` to `int`) | silent | silent | silent |
| `float32` to `double`; any integer to a floating type | silent | silent | silent |
| narrower integer (`int`, `int64`, `uint32` to `int`, `short`, `char`) | wraps (C++20: modular, Go's conversion) | wraps | warns unless provably in range |
| same width, other sign (`int32` to `unsigned`) | wraps | wraps | warns, as narrowing |
| float to integer | C++: **undefined** out of range | `as<T>`: Go's conversion, truncation toward zero, defined for every value | warns |
| `float64` to `float` | C++: undefined out of range | `as<float>` | warns |
| untyped constant that fits | silent | silent | silent |
| untyped constant that does not fit | C++ wraps or truncates, **no error** | an error with Go's words (`overflows`, `truncated`) | the error stops the build |
| number to `bool`, `bool` to number | C++ converts | C++ converts | silent |
| `int`, `uint` to anything narrower, by value | as narrowing above | as narrowing above | the `narrowing` analyzer reports the explicit `C.int(n)` only |

Opaque mode can do only what needs no parameter type: the identity (checker-level) and C++'s own conversion. It cannot check
a constant at a call: `f(300)` for a `char` parameter is C++'s business. Typed mode checks it, and says so in Go's words.

Why float to integer is the only conversion the lowering writes: C++20 defines the conversion between integer types as
modular, which is Go's wrapping conversion, so for integers nothing is written and nothing is lost. For floats the standard
leaves out-of-range undefined, and the optimizer uses that; `as<T>` goes through `gp::f2i`, which gives Go's results on amd64 for
values out of range, NaN and infinities.

### What is NOT adapted, and why

- A `string` to `char*`, `unsigned char*` (the callee may write; the user passes `C.CString`), to `std::string&` (needs an
  lvalue), to a `[]byte` parameter; a `char*` result to a `string` (stays `C.GoString`, "later" below).
- `&a[i]`, `&s.f[0]` and any pointer value to `int*`, `char*`, `bool*`: the element is not "one value".
- `&x` to a parameter spelled as an array (`float col[3]`): see the rules of `scalarptr`.
- A non-constant integer to an enum, and a **typed** constant (`const mode int = 2`): the Go type says it is an `int`; write
  `Mode(2)`... in C++ terms, the enumerator or an explicit conversion.
- A number to `bool` and the reverse: C++ converts, Go does not, and Go's `if n` is an error; left to C++, silent.
- A narrowing `int` behind a non-const pointer (`&uint32` to `unsigned long*`): an error, not a silent cut.
- An integer behind a pointer for a floating pointee and the reverse.
- A Go `int`/`int64` to a `C.int` parameter *with a check* (a panic or an error): the user rejected it; it wraps like Go's
  conversion, and vet is where the warning belongs.
- HRESULT to `error` and `const char*` result to `string` (below).

## Messages (compile, check, LSP, vet)

Errors name the Go source line and the variable; each says the way out:

```text
say is printf-like (its parameter 1 is a format string), and text would be read as a format: a % in its text is a
  conversion (the format-string bug); pass it as an argument, say("%s", text), or write C.cstr(text) when it is a format on purpose
add_one takes a pointer to unsigned long that it may write, and &n points to uint32 that cannot hold every unsigned long
  (the copy's value would be cut when it is stored back): declare n with the C type (C.ulong)
scale takes a pointer to double, and &n is a pointer to int: C converts neither way through a pointer; declare n with the C type (C.double)
level_value: the constant 300 is outside the values that C++ defines for the enum Level (0 to 255): pass an enumerator by name
make_object: the address of a pointer variable for a void** parameter cannot be the argument of a deferred or spawned call ...
cannot use 200 (untyped int constant) as C.char value in argument to take_char (overflows)
runtime error: the variable huge holds 1099511627776, which does not fit the C type int of the parameter it is passed to
```

They are decided by `adapt.Decide` and reported by the lowering at `gocpp build` and `run`, and, since [ADR-134](134-the-argument-adaptation-errors-are-decided-by-one-package-that-the-lowering-the-checker-and-the-editor-use.md),
by `gocpp check`, `gocpp vet` and the editor too, with the same words at the same place (this ADR first left that open as a gap, with an estimate of 2.5 to
3 days: the decisions extracted into a package of their own, an `icheck` pass over every call, the editor tests; the
estimate held). The refusals that are C++'s own errors (the address of an element, `&x` for an array parameter, overloads that
disagree) stay the C++ compiler's. The checker's own messages for the adapted shapes are silenced (a probe of
`&p` for a `void**` would say "cannot use &p (value of type **T) as *unsafe.Pointer"; the rule `adaptedArg` skips the
shapes the table adapts). The language server shows clang's parameter type in hover and signature help as before (`const wchar_t*`
for `LPCWSTR`): the adaptation is invisible there, which is the point. `gocpp vet`: see the numbers table; the design is a
warning at an implicit narrowing and sign change at a call, and none is built.

## Consequences

- Programs written for typed mode lose the `C.cstr`, `C.wstr`, `unsafe.Pointer` and `C.float` noise; `demos/imgui` and
  `demos/imgui-dx11` are written that way and build with no flag (typed is the default when clang++ is installed, ADR-131;
  cross-built for Windows with MinGW-w64; not run again on the Windows PC in this job).
- On a machine without clang++, or with `-interop=opaque`, these programs stop at the first adapted call with the C++
  compiler's own error, and say so in their README.
- The C++ of a call with out-parameters is long: the lambdas are written on one line, and a call of a C++ function whose result
  gocpp does not type is written three times by the wrapper that owns a `values<R>` result (`ownedValue`). Wrapping lines
  inside an expression is not done: a multi-line lambda is a statement of its own in the line map of the debugger, which an
  expression inside another inline lambda cannot have.
- The table is the place to add a row: a class of argument, a verdict function, a lowering and its test. A row must say its
  soundness argument as above.
- `C.bool`, `C.int` and friends being Go's types: code that relied on the error "mismatched types C.int and int32" no
  longer gets it. That was the point.

## Later (not in this ADR)

- **HRESULT to `error`.** A COM call returns an `HRESULT`; `if hr != 0` is the idiom. My view: a result row of the table
  (`HRESULT` to `error`) is the next job, but the type is `LONG`, which is a typedef in the header, and the safe rule needs
  the typedef name from the declaration (clang gives it as `Alias`); `SUCCEEDED(hr)` and `FAILED(hr)` are macros. Do it with
  the `!` error operator once the result adaptations have their own table.
- **A `const char*` result to `string`.** `C.GoString(p)` copies; an adaptation would copy silently, and a function that returns
  a pointer to static or to freed memory makes the copy a bug the user cannot see. A policy question for the user, not a
  type-only decision.
- **gocpp vet at a call.** Narrowing and sign warnings need the callee's parameter types, which only the typed pipeline has.
  `gocpp vet` in typed mode runs the checker but not the lowering; the way is a hook in `vet.Config` fed with the findings
  of a lowering pass, or the `icheck` package (which has the overload sets) emitting them with an option.

## Tests

- `tests/typed/adapt_numbers.gp`, `adapt_enum.gp`, `adapt_wide.gp`, `adapt_voidout.gp`, `adapt_format.gp`,
  `adapt_numbers_value.gp` (a C++ twin each, with g++ and clang++; the harness also checks the opaque build is rejected, or
  for `adapt_numbers_value.gp` and `c_identity.gp` that opaque mode builds and prints the same).
- Expected errors, `tests/typed/err_adapt_*.gp`: printf, narrowing behind a pointer, int/float pointee, float pointee width,
  enum range, `void**` under defer, the address of an element, overloads that adapt differently, a typed constant for an enum,
  and five constants that do not fit (char, unsigned, truncated, float, int).
- Review round 1: `err_adapt_array_float`, `_writer`, `_const`, `_voidptr`, `_method` (an `&x` for a parameter spelled as an
  array), `adapt_array_element.gp` (the address of an element is what such a parameter takes; with a twin),
  `adapt_pointer_default.gp` (a pointer with a default argument is still one pointer) and `TestArrayParams` in the probe (the spellings), `adapt_order.gp` with a twin (the copy back right after the call: if, `&&`, `for` condition, two calls in one
  expression, `f(&n, n)`, a `void` callee), `err_adapt_wide_method`, `_outptr`, `_alias`, `err_adapt_printf_second`, the
  `bool(x)` of a `C.bool` in `c_identity.gp` (also with a C++ twin) and `TestInterop/generics_cxx_methods` (g++ and clang++),
  and `call_out` / `call_void_out` in `runtime/test/interop_test.cpp`.
- Review round 2: `err_adapt_array_typedef`, `_typedef_const`, `_typedef_matrix`, `_typedef_method`, `adapt_pointer_typedef.gp`
  (with a twin) and `TestTypedefParams` (a typedef of an array is an array, a typedef of a pointer a pointer, for functions
  and methods, named and unnamed); `adapt_order_args.gp` (with a twin: the copy-in after the other arguments, the two repros,
  a method, a call in the arguments of a call with two out-parameters); `err_adapt_wide_char16_method`; the panic case in
  `runtime/test/interop_test.cpp`; the loop of `adapt_order.gp` is bounded, so a missing copy back fails the output at once.
- Identity: `tests/typed/c_identity.gp` (both modes) and the rows in `internal/lower/interop_test.go` (`TestCUserErrors`): what is
  accepted (`C.int` with `int32`, each pair of the list) and what stays an error (`int`, `uint32`, `float64`, a named type, a constant that does not fit).
- Mutation checks (each change to the code makes the named test fail):
  - every overload must agree on the row (a row applied when only one overload agrees):
    `strings_auto`, `err_adapt_overloads`;
  - the lossless copy-back rule (`scalarptr` accepting a narrower C type): `err_adapt_narrow`;
  - the enum range check: `err_adapt_enum_range`;
  - the printf guard: `err_adapt_printf`; the format probe without warnings: `TestFormatKind`;
  - `void_out` under defer or go: `err_adapt_defer`;
  - the float conversion lowering: `TestTypedEmitAdaptations`;
  - the constant representability check: `err_adapt_constchar` and its siblings;
  - the checker silencing of adapted arguments: `adapt_enum`, `adapt_wide`, `adapt_voidout`;
  - ignore versus other when combining overloads: `strings_auto`;
  - the keeps rule for wide strings: `err_adapt_keeps_wide`;
  - the address of an element taken as a variable: `err_adapt_element`;
  - the C identity rule: `TestCUserErrors` and `c_identity` (a real test failure for both: the rule inverted fails the rows
    of `TestCUserErrors` and the output of `c_identity`);
  - review round 1, each killed by a failing test and not by a build error:
    - `bool(x)` of a `C.bool` refused again: `TestCUserErrors`, `c_identity`;
    - `&x` adapted for an array parameter (`arrayAt` ignored): `err_adapt_array_const` and its siblings;
    - the copy back not made by `call_out`: `runtime/test/interop_test.cpp` (3 of 426 checks fail) and `adapt_order`
      (whose loop never ends, so it is killed by the time limit);
    - the wide string without the keeps checks: `err_adapt_wide_method`, `_outptr`, `_alias`;
    - the printf hint without the `"%s"` argument: `err_adapt_printf_second`;
    - the address of a Go variable not noticed by the probe (`passesAddress`): `adapt_order`;
    - an array spelling read as a pointer for a named parameter, or a default argument with spaces:
      `TestArrayParams`; the array spellings asked for every function: `TestArrayParamsAreAskedForTheFunctionsThatGetAnAddress`.
