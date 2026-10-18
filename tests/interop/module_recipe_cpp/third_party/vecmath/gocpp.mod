module github.com/example/vecmath

cxx c++17

// The recipe of the library, shipped by its author.
cxx github.com/example/vecmath {
	src src/*.cpp
	linux: src platform/os_linux.cpp
	windows: src platform/os_windows.cpp
	darwin: src platform/os_darwin.cpp
	include include
	define VECMATH_WIDE      // the layout of Vec3 in the header: both sides
	private-define VECMATH_BUILDING
}
