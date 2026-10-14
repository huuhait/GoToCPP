#!/bin/sh
# mkwords.sh writes cxx_words.txt: the words that can't be a C++ name in the
# generated code, for g++ or clang++ in -std=c++20 mode, and are not macros.
# These are the C++20 keywords and alternative tokens (and, not, xor, ...)
# and the extension words of the compilers: keywords such as __attribute__,
# __int128, _Complex, __restrict and asm, type traits such as __is_same and
# __remove_cv, and the preprocessor's own words such as _Pragma, __FILE__,
# __has_include and __VA_ARGS__. names.go gives a Go name that is one of them
# a _ suffix. The macros of the runtime's headers are in cxx_macros.txt
# (TestMacroList keeps that list up to date).
#
# The compilers know these words, but they can't list them, so the script
# tries every identifier that it finds in their programs (cc1plus, and
# libclang-cpp, which holds clang's keyword table):
#
#  1. candidates: the identifiers in the output of strings(1) on both
#     programs, and every tail of them that starts with _ (the linker
#     stores "__int128" as the end of "unsigned __int128"), and each __x
#     with __ at the end too (g++ makes __int128__ from __int128 when it
#     starts), that start with __, with _ and a capital letter, or with a
#     lower-case letter (at most 40 characters, no _Z... symbol names), plus
#     the C++20 keywords, in case a program doesn't hold one;
#  2. one file declares each candidate as a variable, as a function that is
#     called and as a local variable, each in its own namespace, and both
#     compilers compile it with -fsyntax-only; a candidate whose lines get
#     an error is a suspect;
#  3. each suspect is compiled again in a file of its own (an error can
#     spill over to the next lines), and it stays if a compiler rejects it;
#  4. the macros that the compilers predefine (-dM -E of an empty file) are
#     left out: cxx_macros.txt has them;
#  5. __VA_ARGS__ and __VA_OPT__ are added: they mean something only in a
#     macro, and g++ warns about them anywhere else.
#
# Run it when the dev shell gets a new g++ or clang++, and look at the
# changes with git diff:
#
#	nix develop -c internal/lower/mkwords.sh
#
# It works from any folder and takes a few minutes. It needs g++, clang++,
# strings and ldd. TestWords checks the list with both compilers in make ci.
set -eu

dir=$(cd "$(dirname "$0")" && pwd)
out=$dir/cxx_words.txt
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

flags="-std=c++20 -fsyntax-only"
cc1plus=$(g++ -print-prog-name=cc1plus)
clang=$(clang++ -print-prog-name=clang)
libclang=$(ldd "$clang" | awk '/libclang-cpp/ {print $3}')
echo "mkwords.sh: reading $cc1plus and ${libclang:-$clang}"

keywords="alignas alignof asm auto bool break case catch char char8_t char16_t
char32_t class concept const consteval constexpr constinit const_cast continue
co_await co_return co_yield decltype default delete do double dynamic_cast
else enum explicit export extern false float for friend goto if inline int
long mutable namespace new noexcept nullptr operator private protected public
register reinterpret_cast requires return short signed sizeof static
static_assert static_cast struct switch template this thread_local throw true
try typedef typeid typename union unsigned using virtual void volatile wchar_t
while and and_eq bitand bitor compl not not_eq or or_eq xor xor_eq"

{
	strings -n 2 "$cc1plus" "${libclang:-$clang}"
	printf '%s\n' $keywords
} | grep -oE '[A-Za-z_][A-Za-z0-9_]*' |
	awk '{
		print
		for (i = 2; i <= length($0); i++) if (substr($0, i, 1) == "_") print substr($0, i)
		if ($0 ~ /^__.*[^_]$/) print $0 "__"
	}' |
	grep -E '^(__|_[A-Z]|[a-z])' | grep -v '^_Z' | awk 'length($0) <= 40' | sort -u >"$work/cands"
echo "mkwords.sh: $(wc -l <"$work/cands") candidates"

# decl prints the three uses of the name $1 (in namespaces a$2, b$2, c$2).
decl() {
	printf 'namespace a%s { long %s = 0; }\n' "$2" "$1"
	printf 'namespace b%s { long %s(long x); long use() { return %s(1); } }\n' "$2" "$1" "$1"
	printf 'namespace c%s { void f() { long %s = 1; (void)%s; } }\n' "$2" "$1" "$1"
}

n=0
while read -r name; do
	n=$((n + 1))
	decl "$name" "$n"
done <"$work/cands" >"$work/all.cpp"
{
	g++ $flags -fmax-errors=0 "$work/all.cpp" 2>&1 || true
	clang++ -Qunused-arguments $flags -ferror-limit=0 "$work/all.cpp" 2>&1 || true
} | sed -n 's/^.*all\.cpp:\([0-9]*\):.*$/\1/p' | sort -un |
	awk '{print int(($1 - 1) / 3) + 1}' | sort -un >"$work/lines"
awk 'NR == FNR {bad[$1] = 1; next} FNR in bad' "$work/lines" "$work/cands" >"$work/suspects"
echo "mkwords.sh: $(wc -l <"$work/suspects") suspects"

: >"$work/words"
while read -r name; do
	decl "$name" "" >"$work/one.cpp"
	if ! g++ $flags "$work/one.cpp" >/dev/null 2>&1 ||
		! clang++ -Qunused-arguments $flags "$work/one.cpp" >/dev/null 2>&1; then
		echo "$name" >>"$work/words"
	fi
done <"$work/suspects"

: >"$work/empty.cpp"
for cc in g++ clang++; do
	"$cc" -std=c++20 -dM -E "$work/empty.cpp"
done | awk '$1 == "#define" {sub(/\(.*/, "", $2); print $2}' | sort -u >"$work/predefined"

{
	cat <<'EOF'
# The words that can't be a C++ name in the generated code (g++ and clang++,
# -std=c++20) and are not macros: the C++20 keywords and alternative tokens,
# and the compilers' extension words. Generated by mkwords.sh; do not edit.
# One word per line.
EOF
	{
		grep -vxFf "$work/predefined" "$work/words"
		printf '%s\n' __VA_ARGS__ __VA_OPT__
	} | LC_ALL=C sort -u
} >"$out"
echo "mkwords.sh: wrote $(grep -vc '^#' "$out") words to $out"
