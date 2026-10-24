// Copyright 2026 The gocpp Authors. Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflect

import "unsafe"

// The native functions of package reflect (runtime/include/gocpp/reflect.hpp, ADR-035). A type is the
// address of its descriptor (a uintptr; 0 is no type). A value is a type and an unsafe.Pointer to
// the storage of the object, which keeps the storage alive.

//gocpp:native gp::reflect::type_of
func typeOf(x any) uintptr

//gocpp:native gp::reflect::t_kind
func tKind(t uintptr) int

//gocpp:native gp::reflect::t_name
func tName(t uintptr) string

//gocpp:native gp::reflect::t_pkgpath
func tPkgPath(t uintptr) string

//gocpp:native gp::reflect::t_string
func tString(t uintptr) string

//gocpp:native gp::reflect::t_cppname
func tCppName(t uintptr) string

//gocpp:native gp::reflect::t_size
func tSize(t uintptr) uintptr

//gocpp:native gp::reflect::t_align
func tAlign(t uintptr) uintptr

//gocpp:native gp::reflect::t_elem
func tElem(t uintptr) uintptr

//gocpp:native gp::reflect::t_key
func tKey(t uintptr) uintptr

//gocpp:native gp::reflect::t_len
func tLen(t uintptr) int

//gocpp:native gp::reflect::t_chandir
func tChanDir(t uintptr) int

//gocpp:native gp::reflect::t_numfield
func tNumField(t uintptr) int

//gocpp:native gp::reflect::f_name
func fName(t uintptr, i int) string

//gocpp:native gp::reflect::f_pkgpath
func fPkgPath(t uintptr, i int) string

//gocpp:native gp::reflect::f_tag
func fTag(t uintptr, i int) string

//gocpp:native gp::reflect::f_type
func fType(t uintptr, i int) uintptr

//gocpp:native gp::reflect::f_embedded
func fEmbedded(t uintptr, i int) bool

//gocpp:native gp::reflect::f_offset
func fOffset(t uintptr, i int) uintptr

//gocpp:native gp::reflect::t_numin
func tNumIn(t uintptr) int

//gocpp:native gp::reflect::t_numout
func tNumOut(t uintptr) int

//gocpp:native gp::reflect::t_in
func tIn(t uintptr, i int) uintptr

//gocpp:native gp::reflect::t_out
func tOut(t uintptr, i int) uintptr

//gocpp:native gp::reflect::t_variadic
func tVariadic(t uintptr) bool

//gocpp:native gp::reflect::t_comparable
func tComparable(t uintptr) bool

//gocpp:native gp::reflect::t_nmethod
func tNumMethod(t uintptr) int

//gocpp:native gp::reflect::m_name
func mName(t uintptr, i int) string

//gocpp:native gp::reflect::m_pkgpath
func mPkgPath(t uintptr, i int) string

//gocpp:native gp::reflect::m_sig
func mSig(t uintptr, i int) string

//gocpp:native gp::reflect::m_type
func mType(t uintptr, i int) uintptr

//gocpp:native gp::reflect::t_implements
func tImplements(t, u uintptr) bool

//gocpp:native gp::reflect::t_assignable
func tAssignable(t, u uintptr) bool

//gocpp:native gp::reflect::t_convertible
func tConvertible(t, u uintptr) bool

//gocpp:native gp::reflect::t_ptrto
func tPtrTo(t uintptr) uintptr

//gocpp:native gp::reflect::t_sliceof
func tSliceOf(t uintptr) uintptr

//gocpp:native gp::reflect::rtype_of
func rtypeOf(t uintptr) unsafe.Pointer

//gocpp:native gp::reflect::v_unpack
func vUnpack(x any) (uintptr, unsafe.Pointer)

//gocpp:native gp::reflect::v_ifaceelem
func vIfaceElem(t uintptr, p unsafe.Pointer) (uintptr, unsafe.Pointer)

//gocpp:native gp::reflect::v_interface
func vInterface(t uintptr, p unsafe.Pointer) any

//gocpp:native gp::reflect::v_int
func vInt(t uintptr, p unsafe.Pointer) int64

//gocpp:native gp::reflect::v_uint
func vUint(t uintptr, p unsafe.Pointer) uint64

//gocpp:native gp::reflect::v_float
func vFloat(t uintptr, p unsafe.Pointer) float64

//gocpp:native gp::reflect::v_complex
func vComplex(t uintptr, p unsafe.Pointer) complex128

//gocpp:native gp::reflect::v_string
func vString(t uintptr, p unsafe.Pointer) string

//gocpp:native gp::reflect::v_bool
func vBool(t uintptr, p unsafe.Pointer) bool

//gocpp:native gp::reflect::v_bytes
func vBytes(t uintptr, p unsafe.Pointer) []byte

//gocpp:native gp::reflect::v_setint
func vSetInt(t uintptr, p unsafe.Pointer, x int64)

//gocpp:native gp::reflect::v_setuint
func vSetUint(t uintptr, p unsafe.Pointer, x uint64)

//gocpp:native gp::reflect::v_setfloat
func vSetFloat(t uintptr, p unsafe.Pointer, x float64)

//gocpp:native gp::reflect::v_setcomplex
func vSetComplex(t uintptr, p unsafe.Pointer, x complex128)

//gocpp:native gp::reflect::v_setstring
func vSetString(t uintptr, p unsafe.Pointer, x string)

//gocpp:native gp::reflect::v_setbool
func vSetBool(t uintptr, p unsafe.Pointer, x bool)

//gocpp:native gp::reflect::v_isnil
func vIsNil(t uintptr, p unsafe.Pointer) bool

//gocpp:native gp::reflect::v_unsafeptr
func vUnsafePtr(t uintptr, p unsafe.Pointer) unsafe.Pointer

//gocpp:native gp::reflect::v_assign
func vAssign(t uintptr, dst, src unsafe.Pointer)

//gocpp:native gp::reflect::v_assign_iface
func vAssignIface(dt uintptr, dst unsafe.Pointer, st uintptr, src unsafe.Pointer)

//gocpp:native gp::reflect::v_new
func vNew(t uintptr) unsafe.Pointer

//gocpp:native gp::reflect::v_ptrto
func vPtrTo(pt uintptr, target unsafe.Pointer) unsafe.Pointer

//gocpp:native gp::reflect::v_offset
func vOffset(p unsafe.Pointer, off uintptr) unsafe.Pointer

//gocpp:native gp::reflect::v_elem
func vElem(t uintptr, p unsafe.Pointer) unsafe.Pointer

//gocpp:native gp::reflect::v_len
func vLen(t uintptr, p unsafe.Pointer) int

//gocpp:native gp::reflect::v_cap
func vCap(t uintptr, p unsafe.Pointer) int

//gocpp:native gp::reflect::v_index
func vIndex(t uintptr, p unsafe.Pointer, i int) unsafe.Pointer

//gocpp:native gp::reflect::v_mkslice
func vMkSlice(t uintptr, n, c int) unsafe.Pointer

//gocpp:native gp::reflect::v_reslice
func vReslice(t uintptr, p unsafe.Pointer, st uintptr, i, j, k int) unsafe.Pointer

//gocpp:native gp::reflect::v_setlen
func vSetLen(t uintptr, p unsafe.Pointer, n int)

//gocpp:native gp::reflect::v_extend
func vExtend(t uintptr, p unsafe.Pointer, n int) unsafe.Pointer

//gocpp:native gp::reflect::v_copy
func vCopy(dt uintptr, dp unsafe.Pointer, st uintptr, sp unsafe.Pointer) int

//gocpp:native gp::reflect::v_mapnew
func vMapNew(t uintptr) unsafe.Pointer

//gocpp:native gp::reflect::v_mapindex
func vMapIndex(t uintptr, p, kp unsafe.Pointer) unsafe.Pointer

//gocpp:native gp::reflect::v_maphas
func vMapHas(t uintptr, p, kp unsafe.Pointer) bool

//gocpp:native gp::reflect::v_mapset
func vMapSet(t uintptr, p, kp, vp unsafe.Pointer)

//gocpp:native gp::reflect::v_mapdel
func vMapDel(t uintptr, p, kp unsafe.Pointer)

//gocpp:native gp::reflect::v_mapclear
func vMapClear(t uintptr, p unsafe.Pointer)

//gocpp:native gp::reflect::v_mapsnap
func vMapSnap(t uintptr, p unsafe.Pointer) unsafe.Pointer

//gocpp:native gp::reflect::v_snaplen
func vSnapLen(t uintptr, h unsafe.Pointer) int

//gocpp:native gp::reflect::v_snapkey
func vSnapKey(t uintptr, h unsafe.Pointer, i int) unsafe.Pointer

//gocpp:native gp::reflect::v_snapval
func vSnapVal(t uintptr, h unsafe.Pointer, i int) unsafe.Pointer

//gocpp:native gp::reflect::v_equal
func vEqual(t uintptr, p, q unsafe.Pointer) bool

//gocpp:native gp::reflect::v_convert
func vConvert(st uintptr, sp unsafe.Pointer, dt uintptr) unsafe.Pointer

//gocpp:native gp::reflect::swapper
func swapper(slice any) func(i, j int)
