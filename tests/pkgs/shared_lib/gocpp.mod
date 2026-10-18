module example.com/jsonapp

require github.com/example/minijson v1.0.0

replace github.com/example/minijson => ./third_party/minijson

// How to build the library (no CMake): its C files, the folder with its headers, a macro that the
// headers and the library share, one only for the library, and a system library for the link.
cxx github.com/example/minijson {
	src src/*.c
	exclude src/*_test.c
	include include
	define MINIJSON_MAX_DEPTH=16
	private-define MINIJSON_BUILDING
	linux,darwin: define MINIJSON_UNIX
	link m
}
