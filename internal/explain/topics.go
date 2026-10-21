package explain

import (
	"embed"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/huuhait/gocpp/internal/lower"
)

// A Topic is one mechanism of the lowering that gocpp explain describes: the same names as the
// kinds of the reasons in the annotated view (lower.Kind), so that a [boxing] note leads to
// `gocpp explain boxing`. The text is curated; the example is compiled by the real compiler
// each time, so the C++ shown is always what this gocpp writes.
type Topic struct {
	Name    string
	Aliases []string
	Title   string
	Summary string // one line, for -list
	Text    string // paragraphs separated by blank lines
	Spec    string // the section of the specs: "cpp-mapping.md#heading-anchor"
	Example string // the file in examples/
	// Reference says that the topic has no kind of reasons in the annotated view: it explains a
	// part of the mapping that is a fixed shape, not a decision (the types, arrays, fmt...).
	Reference bool
	Phase     int // the phase that builds what the example uses, when it is later than this gocpp (the gates are opened for the example); 0 otherwise
	See       []string
}

//go:embed examples/*.gp
var exampleFS embed.FS

// Example returns the source of the example of topic t.
func (t *Topic) ExampleSource() string {
	data, err := exampleFS.ReadFile("examples/" + t.Example)
	if err != nil {
		panic(fmt.Sprintf("explain: topic %s: %v", t.Name, err))
	}
	return string(data)
}

// Topics returns the topics in the order that -list shows them. The slice is shared: do not
// change it.
func Topics() []Topic { return topics }

// FindTopic returns the topic with the name or the alias name, or nil.
func FindTopic(name string) *Topic {
	name = strings.ToLower(strings.TrimSpace(name))
	for i := range topics {
		t := &topics[i]
		if t.Name == name || slices.Contains(t.Aliases, name) {
			return t
		}
	}
	return nil
}

// IsKind reports whether name is the name of a topic, and so of a kind of reasons.
func IsKind(name string) bool {
	for _, k := range lower.AllKinds {
		if string(k) == name {
			return true
		}
	}
	return false
}

// KindNames returns the names of the kinds, sorted.
func KindNames() []string {
	out := make([]string, len(lower.AllKinds))
	for i, k := range lower.AllKinds {
		out[i] = string(k)
	}
	sort.Strings(out)
	return out
}

var topics = []Topic{
	{
		Name:    "boxing",
		Aliases: []string{"box", "boxed"},
		Title:   "Boxed variables",
		Summary: "why a local variable lives in a gp::ptr box on the heap",
		Text: `A local variable lives on the C++ stack unless something can see it after it goes out of scope, or from another function while it is in scope. Then it is boxed: the C++ variable is a gp::ptr<T> to a heap block (gp::new_<T>(...)), each use of x is (*x), and &x is the box itself. A pointer to a variable never dangles: it shares the box, which lives as long as the last pointer.

Taking the address is not enough to box a variable. The escape analysis (ADR-114) looks at every use of the address: if the pointer is only read and written through, compared, or handed to a function that borrows it, then x stays an ordinary C++ variable, a pointer to it is gp::borrow(&x) or gp::stack_ptr(x) with no heap block and no count, and the note says that it stays on the C++ stack. This holds for a pointer method called on x (x.Inc()), for p := &x, for &T{...} and new(T) in a local pointer, and for a variable that only closures called where they are made capture (not a parameter or a result: those keep their box).

A variable is boxed when its address (&x, &x.f, &x.arr[i]) is stored, returned, sent, captured, converted to an interface, passed to a function that keeps it, or used by go or defer; when it slices an array in it; when a closure that can outlive the call captures it and changes it anywhere; when a closure captures it and it is larger than 128 bytes; when it is larger than 128 KiB; or when keeping it on the stack would pass the budget (2 KiB for one object, 16 KiB for one function). Function parameters, results and loop variables are boxed whenever their address is taken, and the generic functions are not analysed: they box as before. The first rule that applies is the reason in the annotation.

gocpp build -gp:m prints the decisions like go build -gcflags=-m (-gp:m=2 adds the reasons), and -gp:noescape turns the analysis off, so that every variable whose address is taken is boxed.`,
		Spec:    "cpp-mapping.md#pointers-and-boxing",
		Example: "boxing.gp",
		See:     []string{"closures", "refcount", "allocation"},
	},
	{
		Name:    "closures",
		Aliases: []string{"closure", "capture", "lambda", "funclit"},
		Title:   "Func literals and captures",
		Summary: "how a func literal becomes a lambda, and how it captures variables",
		Text: `A func literal that is only called where it is made (func() {...}() or f := func() {...} with every use of f a call, not go, not defer, not stored, passed or returned, and no recover inside) cannot outlive the call: it is a plain C++ lambda that captures by reference, with no gp::func and no allocation, and a local variable that only such closures capture is not boxed (ADR-114; a parameter or a result that one captures and changes still is).

A literal that go or defer runs directly is also the lambda itself, handed to gp::go_call or gp_push_call, with its captures copied into it.

Any other func literal, used as a value, is a C++ lambda wrapped in gp::func<R(A...)>. The lambda has a hidden template parameter, <int = N>, which numbers the literal for the stack trace (main.f.func1, as Go names it). Copies of a gp::func share the closure's state.

A captured variable is copied into the lambda when that is safe: the closure never sees a change, because the variable never changes after it is declared, and it is at most 128 bytes. Otherwise the variable is boxed (see gocpp explain boxing) and the lambda captures the box, so the function and the closure share one variable, as in Go. The annotation says which of the two happened for each captured name.

A closure that refers to itself (a recursive func literal stored in a variable it captures) makes a reference cycle: the reference counts never reach zero, and the memory is not freed. gocpp vet, once it is built (Phase 13), is planned to report the obvious cases.`,
		Spec:    "cpp-mapping.md#closures",
		Example: "closures.gp",
		See:     []string{"boxing", "defer"},
	},
	{
		Name:    "defer",
		Aliases: []string{"deferred"},
		Title:   "defer and the single exit",
		Summary: "the defer frame, the try block and why each return jumps to its own exit",
		Text: `A function with a defer statement gets a gp::defer_frame (gp_df), its body goes into a try block, and every way out of the body goes through gp_df.gp_run(), which runs the deferred calls, the last one first, and then throws again the panic that left the body, if there was one. The results become variables declared before the frame, so a deferred closure can change a named result and a return assigns the results before the deferred calls run (Go's order).

defer f(a, b) evaluates f and its arguments at the defer statement and stores them in the frame: gp_df.gp_push(f, a, b) for a function of the program, and gp_df.gp_push_call(lambda, ...) for a builtin, a method, a package function or a func value. Each return has its own exit label (gp_ret0, gp_ret1...) at the return's line, so a panic in a deferred call shows the function at that line in the stack trace.

A function without defer is written as it is, with no frame and no try block.`,
		Spec:    "cpp-mapping.md#defer",
		Example: "defer.gp",
		See:     []string{"recover", "panics", "closures"},
	},
	{
		Name:    "recover",
		Aliases: []string{"aware"},
		Title:   "recover",
		Summary: "the ticket that lets recover find the panic of a deferred call's caller",
		Text: `recover returns the panic value only when it is called directly by a deferred function while the program is panicking. C++ has no such rule, so a function that calls recover is "aware": it takes a ticket at its entry (gp::defer_ctx gp_dc = gp::take_defer_ctx()), which says whether a deferred call of a panicking function is running it. gp::recover(gp_dc) then returns the panic and stops it, or nil.

The defer machinery calls an aware function differently from a plain one: gp_push_aware for a function of the program, gp_push_call_aware for a func literal, gp_push_call_icall for a method called through an interface (the runtime looks at the method's thunk then). A func value is looked up when it is called: a literal that contains recover is made as gp::func<Sig>(gp::aware(lambda)).

A function without recover is written exactly as it would be without this feature.`,
		Spec:    "cpp-mapping.md#defer",
		Example: "recover.gp",
		See:     []string{"defer", "panics"},
	},
	{
		Name:    "checks",
		Aliases: []string{"check", "nochecks", "bounds", "nil"},
		Title:   "Run-time checks",
		Summary: "which operations check at run time, and what -nochecks removes",
		Text: `The generated C++ checks what Go checks, with Go's messages and exit code 2: an index of a string, slice or array, a slice expression, a nil pointer dereference (*p, p.f, a value method through a nil pointer), a call of a nil func value, a method call on a nil interface, a failed type assertion, integer division by zero, and a negative shift count.

Most checks are in the runtime types, not in the generated code: gp::slice::operator[] checks the index, gp::ptr::operator* checks for nil. gocpp build -nochecks defines GOCPP_NOCHECKS, which turns off the bounds and nil checks (an index out of range is then undefined behavior, as in C++). Division by zero and negative shifts keep their checks, because C++ has undefined behavior there and the checks cost little. The panics that are part of an operation (a write to a nil map, make with a bad length, append's overflow) stay too.

A division by a constant that is neither 0 nor -1 is the plain C++ operator: nothing can go wrong, so nothing is checked. gocpp emit -annotate -nochecks writes the annotations for a program built with -nochecks.`,
		Spec:    "cpp-mapping.md#runtime-checks-and-panics",
		Example: "checks.gp",
		See:     []string{"panics", "interfaces"},
	},
	{
		Name:    "refcount",
		Aliases: []string{"arc", "owner", "owners", "references", "pointers"},
		Title:   "Reference counts and owners",
		Summary: "where reference counts are touched and which pointers share an owner",
		Text: `Memory is reference counted (ADR-004). A *T is a gp::ptr<T>, which holds a reference count and an address. new(T), &T{...} and a boxed variable make a block with a count of one (unless the escape analysis keeps the object on the stack, see gocpp explain boxing); copying a pointer increments the count, and the last copy to go out of scope frees the block. Counts are atomic, so goroutines can share pointers.

A pointer into an object shares the owner of the whole object: &p.f has p's owner, &s[i] has the owner of the slice's array, &x.f of a boxed x has x's box. So the object stays alive while any pointer into it exists, and the pointer is never dangling. The annotation says where a new owner is made and where one is shared.

A pointer parameter or receiver that the function only reads and writes through, compares, or lends on to other borrowing functions is borrowed (ADR-086): the C++ parameter is a gp::bptr<T>, a raw pointer without a count, and the call passes the caller's pointer as it is (gp::lend(p)), or the address of a field (gp::borrow(&p->f)) instead of an aliasing pointer. The caller makes sure that the object stays alive during the call: the pointer is a temporary or a local that nothing can change, or the callee drops no pointer (it is pure); otherwise the call copies the pointer first. A parameter that is stored, returned, captured, converted to an interface or passed to a function that keeps it stays an owning gp::ptr<T>, and the annotation says why. A parameter that the body never assigns (a string, slice, map, interface, func or such a pointer) is a const T&: the call passes a temporary or a local that nothing can change as it is, and copies anything else first.

Values that hold counts also copy them: a slice or a string copies its owner (one increment), an interface shares its box, a gp::func shares its closure. A moved value (static_cast<T&&>) transfers its count without touching it: the emitter moves where it can prove that the source is not used again, as in s = append(s, x).

Cycles are not freed: use weak.Pointer[T] for the back pointer of a parent-child pair.`,
		Spec:    "cpp-mapping.md#pointers-and-boxing",
		Example: "refcount.gp",
		See:     []string{"boxing", "allocation"},
	},
	{
		Name:    "interfaces",
		Aliases: []string{"interface", "iface", "itab", "assert", "typeswitch"},
		Title:   "Interfaces",
		Summary: "conversions to interfaces (which allocate), assertions, type switches, method calls",
		Text: `An interface value is 32 bytes: a data pointer, the type descriptor of the dynamic type, and a method table (null for the empty interface). There are no C++ virtual functions: x.M() is the interface struct's wrapper, gp::icall(x, slot), which finds the method in the table. A nil interface has no type; a nil *T in an interface is not nil (Go's famous gotcha), because its type is *T.

Converting a value to an interface (gp::to_iface) copies it into a new reference-counted box, which allocates, unless the value is a pointer (the interface shares the pointer's owner) or already an interface (gp::iface_cast shares the box). Copies of an interface share the box: boxes are never written, as in Go.

x.(T) compares the descriptor of the dynamic type (gp::assert_type) and panics with Go's message when it differs; x.(I) asks whether the dynamic type has the methods of I; the comma-ok forms return a tuple instead of panicking. A type switch is an if/else chain of those tests, in case order.

fmt functions take their operands with a descriptor (gp::fmt::as) and do not box them first, except an operand of a non-pointer type with an Error method, which is converted to an any.`,
		Spec:    "cpp-mapping.md#interfaces",
		Example: "interfaces.gp",
		See:     []string{"allocation", "checks", "methods"},
	},
	{
		Name:    "generics",
		Aliases: []string{"generic", "instance", "instances", "typeparams"},
		Title:   "Generic functions",
		Summary: "one C++ function for each list of type arguments (gp_inst_...)",
		Text: `A generic function becomes a C++ function for each list of type arguments that the program names, an instance: gp_inst_<hash>_Map, with the hash taken from the package, the file and the type arguments. The instance is lowered as if the function were written with those types, so a + b, T(x), len(s), x.M() and any(x) mean what they mean for those Go types. Types are not C++ template parameters, because int and int64 are one C++ type (int64_t) while Go tells them apart (type switches, %T, interfaces).

Each use of the function asks for its instance by name; the instance is written once in the unit of the file that first needed it, and a comment names it in Go's notation (// Map[int,string]). The annotation on the generic function's line lists every instance made from that body, and each call site says which instance it calls.

A type argument that is a C/C++ type makes a C++ function template that the C++ compiler deduces instead (Phase 07 README, C/C++ type arguments).`,
		Spec:    "cpp-mapping.md#generics-steps-71-to-73",
		Example: "generics.gp",
		Phase:   7,
		See:     []string{"interfaces", "interop"},
	},
	{
		Name:    "select",
		Aliases: []string{"selects"},
		Title:   "select",
		Summary: "how a select statement picks a case",
		Text: `Each case of a select becomes an object, made in source order before the select runs: gp::sel_send(ch, v), gp::sel_recv(ch, ...) or gp::sel_default. The channel and the value of each case are evaluated once, in Go's order (a call in a later case cannot change an earlier channel). gp::select(cases...) waits until a case can proceed, picks one of the ready ones at random as Go does, and returns its index; a C++ switch on the index runs the case's body. The variables that a receive case declares are assigned in the first statement of the case.

break leaves the C++ switch, like Go's select; continue and labelled jumps belong to the enclosing loop. A select without cases blocks the goroutine forever (gp::select()). A select with a default never blocks.`,
		Spec:    "cpp-mapping.md#goroutines-and-channels",
		Example: "select.gp",
		Phase:   8,
		See:     []string{"channels", "goroutines"},
	},
	{
		Name:    "goroutines",
		Aliases: []string{"goroutine", "go", "threads"},
		Title:   "go statements",
		Summary: "how go f(x) starts a goroutine",
		Text: `A goroutine is an OS thread (ADR-007), taken from a pool of reused threads (ADR-026). go f(a, b) evaluates f and the arguments in the calling goroutine, as defer does, and starts the call on a new one: gp::go_fn(f, a, b) for a function of the program and gp::go_call(callable, args...) for anything else. When main returns the program exits, whatever the other goroutines do, as in Go.

A goroutine's stack is a fixed C++ stack (8 MiB by default), where Go's grows; a very deep recursion stops sooner than in Go, with Go's "stack overflow" fatal error. Panics in a goroutine that nothing recovers end the program, as in Go.`,
		Spec:    "cpp-mapping.md#goroutines-and-channels",
		Example: "goroutines.gp",
		Phase:   8,
		See:     []string{"channels", "select", "defer"},
	},
	{
		Name:    "channels",
		Aliases: []string{"channel", "chan", "send", "receive"},
		Title:   "Channels",
		Summary: "gp::chan, sends, receives, close and range over a channel",
		Text: `chan T is gp::chan<T>, a shared reference to the channel: copies, and the directional views chan<- T and <-chan T, are the same C++ type and share one channel. make(chan T, n) is gp::make_chan<T>(n). A send is gp::send(ch, v), a receive gp::recv(ch), or gp::recv2(ch) for v, ok := <-ch. Sending blocks until a receiver is ready or the buffer has room; sending on a closed channel panics; a nil channel blocks forever. After close, a receive drains the buffer and then gives the zero value, with ok false.

for v := range ch receives until the channel is closed (gp::range_chan). The runtime's channel locks and parks the thread; there is no spinning.`,
		Spec:    "cpp-mapping.md#goroutines-and-channels",
		Example: "channels.gp",
		Phase:   8,
		See:     []string{"select", "goroutines", "range"},
	},
	{
		Name:    "allocation",
		Aliases: []string{"alloc", "heap", "allocations"},
		Title:   "Where gocpp allocates",
		Summary: "the operations that allocate: new, make, append, interface conversions, strings",
		Text: `These operations allocate: new(T) and &T{...} (a reference-counted box, unless the escape analysis keeps the object on the stack); make of a slice, map or channel; append when the capacity is not enough; converting a non-pointer value to an interface (a box that holds a copy); string concatenation; converting between a string and a []byte or []rune; a closure that captures a variable that gets boxed; a boxed variable itself.

Slicing does not allocate: s[a:b] shares the array. Neither does copy, clear, delete, indexing, ranging, or calling a method. gp::concat joins a chain of + operands with one allocation, as Go's compiler does. Reading the annotations of a hot function shows which lines allocate.`,
		Spec:    "cpp-mapping.md#pointers-and-boxing",
		Example: "allocation.gp",
		See:     []string{"boxing", "interfaces", "strings", "slices"},
	},
	{
		Name:    "order",
		Aliases: []string{"evaluation-order", "temporaries", "temps"},
		Title:   "Order of evaluation",
		Summary: "why gp_t0 temporaries appear: Go's order, not C++'s",
		Text: `Go evaluates the calls in an expression from left to right; C++ leaves the order of function arguments and of most operands unspecified, and g++ and clang++ pick different orders. So wherever the order could be seen (two calls, a call and a variable that the call can change, two operations that can panic), the emitter evaluates the operands into gp_ temporaries first, in Go's order: auto gp_t0 = f(); auto gp_t1 = g(); then the expression uses them.

A boxed variable, a package variable and *p can change in any call, so they are read after the calls of their statement, as gc does: in x + f(), f runs first. An expression with at most one call, or with operands that nothing can change, needs no temporaries and is written as it is.`,
		Spec:    "cpp-mapping.md#order-of-evaluation",
		Example: "order.gp",
		See:     []string{"boxing", "panics"},
	},
	{
		Name:    "interop",
		Aliases: []string{"c", "cpp", "c++", "foreign", "cgo"},
		Title:   "C and C++ values",
		Summary: "C and C++ names, values and calls in gocpp code",
		Text: `A header import (import cstring "<string.h>") makes the header's names usable as cstring.strlen, cstring.size_t and so on. Calls and values of C/C++ types are written as C++ writes them: the C++ compiler checks the arguments, a C/C++ value keeps C++'s lifetime rules, and pointers to C/C++ types are raw T* or gp::cptr<T> (a pointer that shares gocpp ownership), with no nil check. The C built-in types are C.int, C.char, C.size_t..., and C.cstr(s) makes a C string that lives for the statement (see docs/spec/interop.md).

A C/C++ value in an interface is copied into a box with a descriptor made from its C++ type (gp::to_iface_foreign); only interfaces without methods can hold one until typed interop (Phase 12).`,
		Spec:    "interop.md",
		Example: "interop.gp",
		See:     []string{"generics", "checks"},
	},
	{
		Name:    "embedding",
		Aliases: []string{"embed", "embedded", "promoted", "promotion"},
		Title:   "Embedded fields",
		Summary: "promoted fields and methods",
		Text: `A struct that embeds another gets the embedded type's fields and methods promoted: d.Name() is d.Animal.Name(), and d.name is d.Animal.name. The emitter writes the path through the embedded fields (one member access for each step, -> after an embedded pointer, which checks for nil). Methods that an interface needs are also written as forwarding members of the struct, so that the method tables of interfaces can find them.

A struct that embeds one C/C++ type has that type's members promoted too (mv.push_back(1) for a struct that embeds std.vector[int]); with several embedded C/C++ types, name the field.`,
		Spec:    "cpp-mapping.md#embedding",
		Example: "embedding.gp",
		See:     []string{"methods", "interfaces"},
	},
	{
		Name:    "range",
		Aliases: []string{"loops", "for"},
		Title:   "range loops",
		Summary: "how range over strings, slices, maps, channels, integers and functions is written",
		Text: `A range loop is a C++ for over an iterator that holds the range expression, evaluated once. A string is gp::range_string (decoding UTF-8; an invalid byte gives U+FFFD); a slice is gp::range_slice, which holds a copy of the slice header, so the loop covers the original length even when the body appends; a map is gp::range_map, which starts at a random slot of the map's table and holds the table, so a key deleted before it is reached is skipped, a new key may or may not be produced (as in Go), and no entry is produced twice, also when the map grows during the loop; a channel receives until it is closed; an integer is a counting loop. An array is ranged in place, with a copy only where the body could change it.

A range over a function (for x := range seq) is Go's own rewrite: seq is called with the loop body as its yield function, a closure, and a boxed gp::rangefunc_state records break, continue and return and checks that the iterator does not misuse yield.

Each iteration has its own copy of the loop variables (Go 1.22): assigning to them in the body does not change the iteration, and a closure or a pointer that keeps one keeps the variable of its iteration (a boxed variable gets a new box each time).`,
		Spec:    "cpp-mapping.md#slices",
		Example: "range.gp",
		See:     []string{"slices", "maps", "channels"},
	},
	{
		Name:    "names",
		Aliases: []string{"naming", "escapes", "reserved"},
		Title:   "Names in C++",
		Summary: "when a Go name is spelled differently in C++",
		Text: `gocpp keeps a Go name in C++ wherever it can. A name gets a _ suffix when C++ reserves it: a keyword or alternative token (new, class, delete, and), an extension word of g++ or clang++, a macro of the headers that the runtime includes (EOF, errno, NULL), a name the generated code uses unqualified (int64_t), or a name that starts with gp_ (the generated names). The suffix keeps the mapping one to one: new becomes new_ and the user's own new_ becomes new__. Names that the generated code makes (gp_t0, gp_df, gp_inst_...) start with gp_ and never end in _, so they cannot clash.

A shadowing variable is declared in a block of its own, so the C++ scoping matches Go's. The package namespace is gpkg_<path>, with the import path escaped.`,
		Spec:    "cpp-mapping.md#names-inside-a-package",
		Example: "names.gp",
		See:     []string{"order"},
	},
	{
		Name:    "methods",
		Aliases: []string{"method", "receiver", "receivers"},
		Title:   "Methods",
		Summary: "how methods are declared and called",
		Text: `A method is a member of its type's C++ struct. A value receiver is a copy (T v = *this;), or a const reference when the body cannot change it; a pointer receiver is the first parameter of a static member, T::M(p, args), so no dynamic dispatch is involved. A value method called through a pointer is p->M(), which checks p for nil and panics like Go. The receiver is evaluated in Go's order, and a method value (x.M) or method expression (T.M) is a gp::func.

A method called on an interface is different: see gocpp explain interfaces.`,
		Spec:    "cpp-mapping.md#methods",
		Example: "methods.gp",
		See:     []string{"interfaces", "embedding"},
	},
	{
		Name:    "panics",
		Aliases: []string{"panic", "errors", "exceptions"},
		Title:   "panic",
		Summary: "how panics unwind and what the runtime prints",
		Text: `panic(v) throws a C++ exception (gp::panic_error) that carries the value. Deferred calls run while it unwinds (gp_df.gp_take keeps it for gp_run; a panic statement in the function itself does not even throw: gp_df.gp_panic and a goto), and recover can stop it. An unrecovered panic prints Go's message ("panic: boom", or "panic: runtime error: index out of range [5] with length 3", with the goroutine trace) and exits with status 2. The runtime's own checks panic with Go's runtime.Error messages; a nil dereference also prints Go's signal line.

Go's fatal errors (out of memory, stack overflow, all goroutines asleep) end the program the same way, but no recover stops them and no deferred call runs.`,
		Spec:    "cpp-mapping.md#runtime-checks-and-panics",
		Example: "panics.gp",
		See:     []string{"defer", "recover", "checks"},
	},
	{
		Name:    "strings",
		Aliases: []string{"string", "concat"},
		Title:   "Strings",
		Summary: "gp::string, literals, concatenation, slicing and conversions",
		Text:    `A string is a gp::string: an owner, a data pointer and a length (32 bytes). A literal "hi" is "hi"_gs and points at static storage (no allocation). s[a:b] shares the bytes; s[i] is a bounds-checked byte. A chain of + is one gp::concat call that allocates the result once. string(b) and []byte(s) copy, as in Go; string(r) of an integer is the UTF-8 of the rune. Strings are immutable bytes, usually UTF-8; range decodes them.`,
		Spec:    "cpp-mapping.md#strings",
		Example: "strings.gp",
		See:     []string{"slices", "allocation", "range"},
	},
	{
		Name:    "slices",
		Aliases: []string{"slice", "append"},
		Title:   "Slices",
		Summary: "gp::slice, append, make, copy and slicing",
		Text:    `A slice is a gp::slice<T>: an owner, a data pointer, a length and a capacity (40 bytes). Slicing shares the array and keeps it alive through the owner. make allocates a zeroed array. append writes in place when the capacity is enough and otherwise allocates a bigger array; the new capacity is Go's (its growth formula, Go's element sizes and malloc size classes), except where gc's escape analysis changes it for a slice that does not escape, and for C/C++ element types. In s = append(s, x) of a local s, s is moved into append (no extra count). copy handles overlap and does not allocate. An index is checked against the length, a slice expression against the capacity.`,
		Spec:    "cpp-mapping.md#slices",
		Example: "slices.gp",
		See:     []string{"strings", "allocation", "range"},
	},
	{
		Name:    "maps",
		Aliases: []string{"map"},
		Title:   "Maps",
		Summary: "gp::map reads, writes, literals and delete",
		Text:    `A map is a gp::map<K, V>, a shared reference to the table (a Swiss table, with a random hash seed per map). m[k] is gp::get, which gives the zero value for a missing key and never inserts; v, ok := m[k] is gp::lookup. m[k] = v is gp::set(m, k, v), which stores the value and inserts the key if it is new. m[k]++ and m[k] += v change the entry in place with gp::ref, which inserts the zero value first when the key is missing. A write to a nil map panics, a read does not. A map literal is gp::map_of in gc's order (the constant keys first). delete is gp::erase. Iteration order is random, as in Go.`,
		Spec:    "cpp-mapping.md#maps",
		Example: "maps.gp",
		See:     []string{"range", "allocation"},
	},
	{
		Name:    "initialization",
		Aliases: []string{"init", "pkgvars", "globals", "package-variables"},
		Title:   "Package initialization",
		Summary: "package variables, constinit and init functions",
		Text:    `A package variable is constinit, so C++ runs no dynamic initialization of its own (whose order across files C++ does not define). A variable whose initializer is a constant, nil, or a literal of constants is built by the C++ compiler; the others start as their zero value and gp_var_init assigns them at program start, in Go's initialization order (dependencies first, as go/types computes it). gp_init_funcs then calls the init functions in source order, and gp_init_all, which main's wrapper runs, calls both. Packages are initialized after the packages they import.`,
		Spec:    "cpp-mapping.md#package-initialization",
		Example: "initialization.gp",
		See:     []string{"order"},
	},
	// The reference topics: shapes of the mapping that the annotated view has no reasons for.
	{
		Name:      "types",
		Aliases:   []string{"typemap", "sizes"},
		Title:     "The type mapping",
		Summary:   "which C++ type each Go type is, and why int and int64 are one C++ type",
		Reference: true,
		Text: `Every Go type has one C++ type: int and uint are int64_t and uint64_t (all gocpp targets are 64-bit), the sized integers are their stdint types, float32 and float64 are float and double, string is gp::string (32 bytes: owner, data, length), []T is gp::slice<T> (40 bytes), [N]T is gp::array<T, N>, map[K]V is gp::map<K, V>, *T is gp::ptr<T> and func(A) R is gp::func<R(A)>. Results (A, B) are std::tuple<A, B>.

Different Go types can share one C++ type: int and int64 are both int64_t. Where Go can tell them apart (any(int(1)) != any(int64(1)), type switches, %T), the type descriptor in an interface value is chosen by the Go type, not the C++ one. The results of int8, int16, uint8 and uint16 arithmetic are cast back, because C++ promotes these types to int, and signed overflow wraps because everything is compiled with -fwrapv.

unsafe.Sizeof reports the C++ layout, which differs from Go's: a pointer is 16 bytes, a string 32, a slice 40.`,
		Spec:    "cpp-mapping.md#types",
		Example: "types.gp",
		See:     []string{"strings", "slices", "interfaces"},
	},
	{
		Name:      "arrays",
		Aliases:   []string{"array"},
		Title:     "Arrays",
		Summary:   "gp::array is a value: it copies, and a range loop works on a copy",
		Reference: true,
		Text: `A [N]T is a gp::array<T, N>: the N elements are in the object itself, so assigning, passing and returning an array copies it, as in Go. An index is checked against N and panics with Go's message; a constant index is checked by the C++ compiler. a == b compares element by element when the elements are comparable.

A slice expression a[lo:hi] shares the array (gp::sub(&a, lo, hi)), so a local array that is sliced is boxed (gocpp explain boxing). Go ranges over a copy of an array: gocpp makes the copy (gp::range_array_copy) only where the loop body could change the array, and ranges over the array itself otherwise.

Arrays larger than 128 KiB are kept off the C++ stack: the variable is boxed and its literal is built in the box.`,
		Spec:    "cpp-mapping.md#arrays",
		Example: "arrays.gp",
		See:     []string{"slices", "boxing", "range"},
	},
	{
		Name:      "results",
		Aliases:   []string{"multiple-results", "multiple results", "tuple", "tuples"},
		Title:     "Multiple results",
		Summary:   "a function with several results returns a std::tuple",
		Reference: true,
		Text: `The results (A, B) of a function are a std::tuple<A, B>. return a, b is return {a, b};, which evaluates left to right. q, r := f() stores the tuple in a temporary and reads each element with std::get, and it works when one of the names already exists (a, err := ... with an old err).

a, b = b, a evaluates the whole right side first, as Go says: it copies both values into temporaries, then assigns. Named results are variables of the function; a function with defer returns them through its single exit (gocpp explain defer).`,
		Spec:    "cpp-mapping.md#multiple-results",
		Example: "results.gp",
		See:     []string{"order", "defer"},
	},
	{
		Name:      "complex",
		Aliases:   []string{"complex-numbers", "complex numbers", "complex128"},
		Title:     "Complex numbers",
		Summary:   "gp::complex128 follows Go's arithmetic, not std::complex",
		Reference: true,
		Text: `complex64 and complex128 are gp::complex64 and gp::complex128: two floats, the real part first, as in Go. gocpp does not use std::complex, because its * and / follow C's Annex G and differ between C++ libraries. Each operation is what gc does on amd64: the product is always computed in float64 and never fused (-ffp-contract=off), and the quotient is Go's runtime.complex128div, so (Inf+0i) * 2 is (+Inf+NaNi) as in Go, and a division by zero gives infinities or NaN and never panics.

complex(r, i), real(c) and imag(c) are gp::complex128(r, i), gp::real(c) and gp::imag(c). fmt prints a complex number as Go does, (1.5-2i).`,
		Spec:    "cpp-mapping.md#complex-numbers",
		Example: "complex.gp",
		See:     []string{"types", "fmt"},
	},
	{
		Name:      "named",
		Aliases:   []string{"named-types", "named types", "newtype"},
		Title:     "Named types",
		Summary:   "type Celsius float64 is a struct over gp::newtype",
		Reference: true,
		Text: `A named type that is not a struct (type Celsius float64, type IDs []int) is a C++ struct of its own over gp::newtype<T, U>, so it is a distinct C++ type that can have methods and its Go name for fmt and %T. Over a number or a bool, gp::newtype holds the value (gp_value()) and has every operator of the underlying type; over a string, a slice, an array, a map or a pointer, the named type is its underlying runtime type, a public base class, so ids[i], len(ids) and range work unchanged.

Go converts implicitly between a named type and a type with the same underlying type when one of them is unnamed (a []int to an IDs). go/types records no conversion there, so the emitter finds those places and writes IDs(...). A switch over a named integer switches on gp_value(), because C++ rejects a class in a switch.`,
		Spec:    "cpp-mapping.md#named-types",
		Example: "named.gp",
		See:     []string{"methods", "types"},
	},
	{
		Name:      "fmt",
		Aliases:   []string{"builtins", "print", "println", "fmt-and-the-builtins"},
		Title:     "fmt and the builtins",
		Summary:   "fmt is a port of Go's printer that walks values by their type descriptors",
		Reference: true,
		Text: `fmt.Println, Printf, Sprintf, Errorf and the others are the runtime's gp::fmt functions: a port of Go's fmt that walks values by their type descriptors, as Go's walks them by reflection, so every verb, flag, width and error form (%!d(string=hi)) prints as in Go. An operand whose C++ type says its Go type (int as int64_t, string, float64...) is passed as it is; an interface value is passed as it is; every other operand is gp::fmt::as(descriptor, value), which is how int64 (the same C++ type as int), named types, structs, slices and maps are told apart.

The arguments are values at the call, and the order of evaluation is gc's: a variable is read after the calls of its statement. min and max of non-constant values are gp::min and gp::max with Go's rules for NaN and -0; print and println write to stderr in the runtime's format.`,
		Spec:    "cpp-mapping.md#fmt-and-the-builtins",
		Example: "fmt.gp",
		See:     []string{"interfaces", "order", "types"},
	},
	{
		Name:      "files",
		Aliases:   []string{"layout", "output"},
		Title:     "The generated files",
		Summary:   "one header and a unit per .gp file for each package",
		Reference: true,
		Text: `Each package is written to a folder named by its import path: pkg.hpp (everything other packages and its own units need), one translation unit for each .gp file that has functions (render.gp.cpp), pkg_desc.cpp (the descriptors of its types) and pkg_init.cpp (package variables, gp_var_init, gp_init_funcs; main's also gp_init_all and main). gocpp emit prints all the files, each after a line //// gpkg/<import path>/<file>, packages in import order.

The folder is the same in every build of a program, so #line paths and the object cache are stable: a unit that did not change, and whose headers did not, is not compiled again. A declaration in a header has no #line, so a change in a function body does not change the header.`,
		Spec:    "cpp-mapping.md#files",
		Example: "files.gp",
		See:     []string{"initialization", "linemap"},
	},
	{
		Name:      "linemap",
		Aliases:   []string{"line", "debugging", "debug", "line-directives"},
		Title:     "#line and debugging",
		Summary:   "how C++ errors and debuggers point at .gp lines",
		Reference: true,
		Text: `Before every generated line whose .gp line is not the previous line plus one, gocpp writes a #line N "/abs/path/file.gp" directive. So compiler errors and debuggers (gdb, lldb) point at .gp lines, and a breakpoint on a .gp line stops at the right place. Your variable names stay the same in C++, so print s works in the debugger. The lines that have no place in the .gp source (the C++ main wrapper) point at the generated file itself.

gocpp emit leaves the directives of generated lines out, and gocpp emit -annotate replaces all of them by the "// ---- file.gp:LINE" headers, which are the same information for reading. A C++ compile error in generated code is a bug in gocpp: gocpp build says so and keeps the work folder.`,
		Spec:    "cpp-mapping.md#line-and-debugging",
		Example: "linemap.gp",
		See:     []string{"files", "names"},
	},
}
