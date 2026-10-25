package abi

// FuncPCABI0 is a compiler intrinsic.
func FuncPCABI0(f interface{}) uintptr

// FuncPCABIInternal is a compiler intrinsic.
func FuncPCABIInternal(f interface{}) uintptr

// NotAnIntrinsic has a body.
func NotAnIntrinsic() uintptr { return 0 }
