package icheck

// The message templates of the checker, one per kind of error. Every template
// has an entry in `gocpp explain` (internal/explain/messages.go; its test
// TestInteropCheckerMessages walks Templates), so a new message needs one.
const (
	fmtUndefined    = "undefined: %s (the imported headers declare no %s)"
	fmtNoMember     = "%s undefined (type %s has no field or method %s)"
	fmtNoOverload   = "no overload of %s matches the arguments (%s)"
	fmtNoOverloadAt = "no overload of %s matches the call"
	fmtAmbiguous    = "call to %s is ambiguous: several overloads match equally well"
	fmtCannotUse    = "cannot use %s (%s) as %s value in %s"
	fmtTooMany      = "too many arguments in call to %s (have %s, want %s)"
	fmtNotEnough    = "not enough arguments in call to %s (have %s, want %s)"
	fmtNotFunction  = "invalid operation: cannot call non-function %s (type %s)"
	fmtBinary       = "invalid operation: %s (operator %s is not defined on %s and %s)"
	fmtUnary        = "invalid operation: operator %s not defined on %s (type %s)"
	fmtConstMethod  = "cannot call method %s on %s (type %s): the method is not marked %s"
	fmtNonStatic    = "cannot call %s without a value: it is a method, not a static function"
	fmtAddress      = "invalid operation: cannot take address of %s (value of type %s)"
	fmtNotAssign    = "cannot assign to %s (neither addressable nor a map index expression)"
	fmtConstAssign  = "cannot assign to %s: it is const"
	fmtIncomplete   = "invalid use of %s: the type is only declared in the headers, not defined"
	fmtDeleted      = "%s uses a function that C++ has deleted (%s)"
	fmtConvert      = "cannot convert %s (%s) to type %s"
	fmtNoValue      = "%s (no value) used as value"
	fmtPrivate      = "%s undefined (%s is a %s member of type %s)"
	fmtMethodValue  = "cannot use %s as a value: a C++ member function must be called"
	fmtNotValidCXX  = "%s is not valid C++: %s"
)

// Templates lists the message templates, for the test that every message has an
// explanation.
var Templates = []string{
	fmtUndefined, fmtNoMember, fmtNoOverload, fmtNoOverloadAt, fmtAmbiguous, fmtCannotUse,
	fmtTooMany, fmtNotEnough, fmtNotFunction, fmtBinary, fmtUnary, fmtConstMethod, fmtNonStatic,
	fmtAddress, fmtNotAssign, fmtConstAssign, fmtIncomplete, fmtDeleted, fmtConvert, fmtNoValue,
	fmtNotValidCXX, fmtPrivate, fmtMethodValue,
}
