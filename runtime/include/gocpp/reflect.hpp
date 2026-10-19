// gocpp/reflect.hpp: the native functions of the standard package reflect (Phase 09 step 9.3,
// ADR-035). Go's reflect is a SUBSET written in .gp (stdlib/reflect) over the type descriptors
// (gocpp/type.hpp) and their operations (gocpp/typeops.hpp), and its .gp source calls these through
// //gocpp:native (internal/load/native.go, ADR-027). fmt's engine (runtime/src/fmt.cpp) reads
// values through the same descriptors and operations; nothing here duplicates it.
//
// The vocabulary of the .gp side:
//
//     a type            a descriptor's address as a uint64_t (uintptr in Go); 0 is no type. It is
//                       always the descriptor that identifies the type (never the complete copy of
//                       a named type, type_desc::gp_full): pointer equality is type identity.
//     a value           a type and the storage of a C++ object of that type: a gp::ptr<void>
//                       (unsafe.Pointer) whose address is the object and whose owner keeps the
//                       storage alive (a box, the array of a slice, the block of a pointer's
//                       target). Every reflect.Value has the object itself, also a pointer (the
//                       gp::ptr object), never "the pointer in the interface word": one rule for
//                       all kinds.
//
// Storage that these functions make (new_storage) is a block owned by a shared_ptr whose deleter
// destroys the object by the type's lifetime operations (type_ops::gp_ctor, gp_dtor, gp_trivial).
// Values that are not addressable are never written through: a Value that came out of an interface
// shares the interface's box (boxes are immutable), one that came out of a map is a copy.
//
// The panics that Go's reflect raises for a misuse of the API (wrong kind, unaddressable,
// unexported) are raised by the .gp code, which knows the method's name; these functions panic only
// for what the .gp code can't check cheaply: a type that can't be made or assigned (a C++ type
// without default construction or copy assignment) and Go's map and slice runtime panics.
//
// C/C++ (kind Foreign) types: reflect sees them as the Go kind of the same shape (a number, a bool,
// a string, an unsafe.Pointer for every C pointer, a Struct without fields for a class); a class
// without default construction or copy assignment cannot be made or stored ("reflect: C++ type X is
// opaque"). A Foreign value is a C++ object, never a Go-layout one: every native that reads or
// writes through a descriptor goes by the descriptor's fclass for kind Foreign (std::string and
// std::string_view are read as such; SetString on a std::string assigns; SetString on a
// std::string_view, SetInt/SetUint/SetFloat on a number that is not 1, 2, 4 or 8 bytes, and
// SetComplex panic "reflect: X on a C++ value of type T is not supported"; Set/SetZero use the
// type's own copy (gp_store) or "opaque").
#pragma once

#include <gocpp/complex.hpp>
#include <gocpp/func.hpp>
#include <gocpp/iface.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>
#include <gocpp/type.hpp>

#include <cstdint>
#include <tuple>

namespace gp::reflect {

using vptr = gp::ptr<void>;

// --- types (the t of a function is a descriptor's address) ---

// type_of(x): the dynamic type of x, 0 for the nil interface.
std::uint64_t type_of(const gp::iface& x) noexcept;
// t_kind(t): Go's reflect.Kind number (0 Invalid ... 26 UnsafePointer).
std::int64_t t_kind(std::uint64_t t) noexcept;
gp::string t_name(std::uint64_t t);    // Type.Name: "" for an unnamed type
gp::string t_pkgpath(std::uint64_t t); // Type.PkgPath: the import path of a named type, else ""
gp::string t_string(std::uint64_t t);  // Type.String: what %T prints
std::uint64_t t_size(std::uint64_t t) noexcept;
std::uint64_t t_align(std::uint64_t t) noexcept;
std::uint64_t
t_elem(std::uint64_t t) noexcept; // the element of an array, chan, map, pointer, slice (0: none)
std::uint64_t t_key(std::uint64_t t) noexcept;
std::int64_t t_len(std::uint64_t t) noexcept;
std::int64_t t_chandir(std::uint64_t t) noexcept; // 1 recv, 2 send, 3 both
std::int64_t t_numfield(std::uint64_t t) noexcept;
gp::string f_name(std::uint64_t t, std::int64_t i);
gp::string f_pkgpath(std::uint64_t t, std::int64_t i); // "" for an exported name
gp::string f_tag(std::uint64_t t, std::int64_t i);
std::uint64_t f_type(std::uint64_t t, std::int64_t i) noexcept;
bool f_embedded(std::uint64_t t, std::int64_t i) noexcept;
std::uint64_t f_offset(std::uint64_t t, std::int64_t i) noexcept;
std::int64_t t_numin(std::uint64_t t) noexcept;
std::int64_t t_numout(std::uint64_t t) noexcept;
std::uint64_t t_in(std::uint64_t t, std::int64_t i) noexcept;
std::uint64_t t_out(std::uint64_t t, std::int64_t i) noexcept;
bool t_variadic(std::uint64_t t) noexcept;
bool t_comparable(std::uint64_t t) noexcept;
// Methods: the exported methods of a type that isn't an interface (the type's own method set,
// sorted by name); every method of an interface type.
std::int64_t t_nmethod(std::uint64_t t) noexcept;
gp::string m_name(std::uint64_t t, std::int64_t i);
gp::string m_pkgpath(std::uint64_t t, std::int64_t i);
gp::string m_sig(std::uint64_t t, std::int64_t i); // the method's signature text, without receiver
bool t_implements(std::uint64_t t, std::uint64_t u) noexcept;  // u is an interface type
bool t_assignable(std::uint64_t t, std::uint64_t u) noexcept;  // a value of t is assignable to u
bool t_convertible(std::uint64_t t, std::uint64_t u) noexcept; // ... convertible to u
// t_ptrto(t) and t_sliceof(t): the descriptors of *t and []t, made when the program has none (the
// interning makes them the program's own when it has).
std::uint64_t t_ptrto(std::uint64_t t);
std::uint64_t t_sliceof(std::uint64_t t);
// rtype_of(t): the *reflect.rtype of the type (one object for each type, so that the Types of one
// type compare equal): a pointer to a block that holds t.
vptr rtype_of(std::uint64_t t);

// --- values (t, p: the type and the storage of the object) ---

// v_unpack(x): the dynamic type of x and the storage of its value (0 and nil for the nil
// interface).
std::tuple<std::uint64_t, vptr> v_unpack(const gp::iface& x);
// v_ifaceelem(t, p): the same for the interface value at p (type t is an interface type).
std::tuple<std::uint64_t, vptr> v_ifaceelem(std::uint64_t t, const vptr& p);
// v_interface(t, p): the value at p as an interface{} (a copy; for an interface type, its content).
gp::any v_interface(std::uint64_t t, const vptr& p);

std::int64_t v_int(std::uint64_t t, const vptr& p) noexcept;
std::uint64_t v_uint(std::uint64_t t, const vptr& p) noexcept;
double v_float(std::uint64_t t, const vptr& p) noexcept;
gp::complex128 v_complex(std::uint64_t t, const vptr& p) noexcept;
gp::string v_string(std::uint64_t t, const vptr& p);
bool v_bool(std::uint64_t t, const vptr& p) noexcept;
gp::slice<std::uint8_t> v_bytes(std::uint64_t t, const vptr& p);
void v_setint(std::uint64_t t, const vptr& p, std::int64_t x);
void v_setuint(std::uint64_t t, const vptr& p, std::uint64_t x);
void v_setfloat(std::uint64_t t, const vptr& p, double x);
void v_setcomplex(std::uint64_t t, const vptr& p, gp::complex128 x);
void v_setstring(std::uint64_t t, const vptr& p, gp::string x);
void v_setbool(std::uint64_t t, const vptr& p, bool x);

// v_isnil(t, p): a chan, func, interface, map, pointer, slice or unsafe.Pointer is nil.
bool v_isnil(std::uint64_t t, const vptr& p) noexcept;
// v_unsafeptr(t, p): the address that Pointer and UnsafePointer give (chan, func, map, pointer,
// slice, unsafe.Pointer), as an unsafe.Pointer that keeps the pointee alive where it can.
vptr v_unsafeptr(std::uint64_t t, const vptr& p);

// v_assign(t, dst, src): *dst = *src for two objects of type t. v_assign_iface(dt, dst, st, src):
// the object at dst has the interface type dt; it gets the value of type st at src (st concrete, or
// an interface type whose content is taken), which implements dt.
void v_assign(std::uint64_t t, const vptr& dst, const vptr& src);
void v_assign_iface(std::uint64_t dt, const vptr& dst, std::uint64_t st, const vptr& src);
// v_new(t): storage for a new zero value of type t.
vptr v_new(std::uint64_t t);
// v_ptrto(pt, target): storage that holds a pointer of type pt (a *T descriptor) to the object
// target.
vptr v_ptrto(std::uint64_t pt, const vptr& target);
// v_offset(p, off): the object at off bytes from p, in the same storage (a field).
vptr v_offset(const vptr& p, std::uint64_t off);
// v_elem(t, p): the object that the pointer at p points to (nil: nil).
vptr v_elem(std::uint64_t t, const vptr& p);

// Arrays, slices, strings, maps.
std::int64_t v_len(std::uint64_t t, const vptr& p) noexcept;  // array, slice, string, map, chan (0)
std::int64_t v_cap(std::uint64_t t, const vptr& p) noexcept;  // array, slice
vptr v_index(std::uint64_t t, const vptr& p, std::int64_t i); // element i of an array or slice
// v_mkslice(t, len, cap): storage that holds a new slice of type t with len elements (all zero).
vptr v_mkslice(std::uint64_t t, std::int64_t len, std::int64_t cap);
// v_reslice(t, p, st, i, j, k): the slice of type st that is p[i:j:k] of the slice or array at p (t
// is its type); a string is sliced too (st is t).
vptr v_reslice(std::uint64_t t, const vptr& p, std::uint64_t st, std::int64_t i, std::int64_t j,
               std::int64_t k);
// v_setlen(t, p, n): the slice at p gets length n (n <= cap).
void v_setlen(std::uint64_t t, const vptr& p, std::int64_t n);
// v_extend(t, p, n): a new slice value of the slice at p with n more elements: in the same array
// when they fit, else in a bigger one (as append does).
vptr v_extend(std::uint64_t t, const vptr& p, std::int64_t n);
// v_copy(dt, dp, st, sp): copy(dst, src) of a slice or array (or a string source); the count.
std::int64_t v_copy(std::uint64_t dt, const vptr& dp, std::uint64_t st, const vptr& sp);

vptr v_mapnew(std::uint64_t t);
// v_mapindex(t, p, kp): storage with a copy of the value of the key at kp, or nil when it's not
// there.
vptr v_mapindex(std::uint64_t t, const vptr& p, const vptr& kp);
void v_mapset(std::uint64_t t, const vptr& p, const vptr& kp, const vptr& vp);
void v_mapdel(std::uint64_t t, const vptr& p, const vptr& kp);
void v_mapclear(std::uint64_t t, const vptr& p);
// The entries of a map: v_mapsnap(t, p) is a snapshot (a handle that keeps it alive), v_snaplen its
// size, v_snapkey and v_snapval the objects of entry i (inside the snapshot).
vptr v_mapsnap(std::uint64_t t, const vptr& p);
std::int64_t v_snaplen(std::uint64_t t, const vptr& h);
vptr v_snapkey(std::uint64_t t, const vptr& h, std::int64_t i);
vptr v_snapval(std::uint64_t t, const vptr& h, std::int64_t i);

// v_equal(t, p, q): the two objects of the comparable type t are ==.
bool v_equal(std::uint64_t t, const vptr& p, const vptr& q);
// v_convert(st, sp, dt): the value at sp converted to the type dt as Go's conversion does (numbers,
// strings, byte and rune slices, types of one underlying type): storage of type dt.
vptr v_convert(std::uint64_t st, const vptr& sp, std::uint64_t dt);

// swapper(slice): reflect.Swapper of a slice in an interface{}; the panics are Go's.
gp::func<void(std::int64_t, std::int64_t)> swapper(const gp::iface& slice);

} // namespace gp::reflect
