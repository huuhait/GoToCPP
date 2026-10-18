module example.com/twolibs

require (
	github.com/example/minijson v1.0.0
	github.com/example/tinyc v0.1.0
)

replace github.com/example/minijson => ./third_party/minijson

replace github.com/example/tinyc => ./third_party/tinyc

cxx github.com/example/minijson {
	src src/*.c
	exclude src/*_test.c
	include include
	define MINIJSON_MAX_DEPTH=16
	private-define MINIJSON_BUILDING
	link m
}

cxx github.com/example/tinyc {
	src src/*.c
	include include
	define TINYC_FACTOR=3
}
