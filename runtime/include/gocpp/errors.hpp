// gocpp/errors.hpp: the native functions of the standard package errors (Phase 05 step 5.7).
// Go's errors package is written in .gp (stdlib/errors); what it does with internal/reflectlite
// is here, over the type descriptors (gocpp/type.hpp), and its .gp source calls these through
// //gocpp:native (stdlib/errors/wrap.gp, internal/load/native.go):
//
//     reflectlite.TypeOf(err).Comparable()              gp::errors::comparable(err)
//     the checks at the start of As                     gp::errors::check_target(target)
//     TypeOf(err).AssignableTo(targetType) and          gp::errors::assign(err, target)
//     targetVal.Elem().Set(ValueOf(err))
//
// Every function takes an error or the `any` of As's second parameter, whatever the interface
// types are, so they work on the interface values (gocpp/iface.hpp).
#pragma once

#include <gocpp/error.hpp>
#include <gocpp/iface.hpp>

namespace gp::errors {

// comparable(err): the dynamic type of err can be compared with == (its descriptor has an equal
// hook). A struct that holds an interface counts, as in Go, and panics when its dynamic values
// can't be compared. The nil interface is not comparable here (Is never asks).
bool comparable(const gp::iface& err) noexcept;

// check_target(target): target must be a non-nil pointer to a type that implements error or to an
// interface type; else it panics with Go's texts: "errors: target cannot be nil", "errors:
// target must be a non-nil pointer", "errors: *target must be interface or implement error".
void check_target(const gp::iface& target);

// assign(err, target): the dynamic type of err is assignable to the type that target (accepted
// by check_target) points to: the same type, a type with the same underlying type when one of the
// two is unnamed (reflect's AssignableTo), or an interface type that it implements. If it is,
// *target = err (as an interface value of the target's interface type, or the value itself) and
// the result is true.
bool assign(const gp::iface& err, const gp::iface& target);

} // namespace gp::errors
