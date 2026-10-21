# ADR-129: A recipe condition is an `if` block over build constraints

Status: accepted (2026-10-21; user decision: the `windows:` prefix "looks wrong", write Go-like blocks)

## Context

[ADR-024](024-cxx-recipes.md) let a statement of a `cxx` recipe start with an operating system condition as a
word of its own: `windows: link glfw3dll opengl32`, `linux,darwin: link m`, `!windows: define UNIX`. It reads
like a label, it takes one statement per condition (a Windows recipe with four libraries repeats `windows:` four
times), it knows only the operating system (no `amd64`, no `unix`), and it has a grammar of its own: a comma
is "or", `!` takes a single name. The user asked for blocks, as in Go:

```text
cxx github.com/ocornut/imgui {
	src imgui.cpp imgui_draw.cpp imgui_tables.cpp imgui_widgets.cpp
	include . backends
	if windows {
		link glfw3dll opengl32 gdi32 dwmapi
	}
	if linux {
		pkg-config glfw3
		link GL
	}
}
```

gocpp is before 1.0 and there are no third-party recipes (the project has the demos and the test fixtures), so
the old form can go.

## Decision

### The grammar

Inside the block of a `cxx` statement, a line is one of these (white space between words is free; a `//`
comment may end any line; a word is read as in a `//gocpp:` directive, so a brace in a quoted argument or in a
comment is text):

```text
line      = statement | if | branch | close .
statement = verb word { word } .                 the verbs of ADR-024, unchanged
if        = "if" condition "{" .
branch    = "}" "else" "{" | "}" "else" "if" condition "{" .
close     = "}" .
condition = a //go:build expression: tag, "!", "&&", "||", "(", ")" .
```

- The `{` ends its line, and the statements of the block are on the lines after it; a `}` is the only word of
  its line, or the first of `} else ...`. `if windows { link x }`, `} link m` and `if windows` without a brace are
  errors, so a line has one meaning and a reader (human or tool) never has to look for the end of a block in the
  middle of a line.
- The chain `if` / `else if` / `else` takes the first branch whose condition holds, as in Go. A statement outside
  every block applies to every target.
- **The condition is a `//go:build` constraint**, read by `go/build/constraint` (the loader's own reader of
  build lines) and evaluated by the loader's matcher, the one that decides which files of a package are built
  (`load.BuildTags`, which asks go/build's rule with a one-line file `//go:build <tag>`). So the tags are the same
  ones: the operating system and the architecture of the target (`-os`, `-arch`), `unix`, `gocpp`, the release
  tags `go1.N`, and the tags of `-tags`; `android` also satisfies `linux` and `ios` `darwin`, as in Go. A recipe
  and the files of its package read the same words the same way. `if linux || darwin {`, `if !windows {` and
  `if windows && amd64 {` work.
- **An unknown tag is false, with no error**, exactly as in a `//go:build` line. `if widnows {` is never taken,
  and a rule that a build line does not have would make the same words mean two things. **It is a warning**,
  since a tag that nobody defines is nearly always a typo: the loader knows the target and the tags of `-tags`,
  so when a condition names a tag that is none of a system or an architecture that Go knows (any of them, also
  for another target), `unix`, `gocpp`, `gc`, `gccgo`, `cgo`, a release tag `go1.N` or a tag of `-tags`, it
  warns at the line and column of the tag (`gocpp.mod:8:5: warning: cxx example.com/lib: the tag "widnows" in a
  condition is not an operating system, ...; did you mean windows?`). The guess is the nearest known name within
  two edits. A tag is warned about once for each recipe, and the warning is not an error: the recipe is built.
- **Nesting is allowed, to a depth of 4.** A flat `&&` condition cannot say "inside the unix block, linux gets A
  and every other system B", and the alternative of repeating the outer condition in every inner branch is how
  a typo gets in. Deeper than that a recipe is no longer a list of facts, so the limit is 4, like the other
  limits of a recipe (4096 statements, 4096 arguments, 1024 conditions, 16 KiB a line) which exist because a
  recipe can come from a dependency. The reader keeps an explicit stack, so the depth costs nothing.
- **The old form is removed.** `windows: link gdi32` is an error at its line and column whose message shows the
  block to write: `a condition is written as a block, if windows { ... }, since ADR-129`
  (`linux,darwin:` shows `if linux || darwin { ... }`, `!windows:` shows `if !windows { ... }`). Keeping both
  would make a recipe readable in two dialects for good, and "is `windows: if` allowed?" a question; there is
  nothing to be compatible with.

### What an evaluated recipe is

`gocppmod.ParseRecipe` reads the whole recipe once, for every target, and gives each statement its `Cond`: the
`&&` of the conditions of its blocks, with the earlier branches of an `else` negated
(`!windows && linux && amd64`, `!(windows || (linux && amd64))`). The build asks `Cond.Match` with the tags of
the target (`load.BuildTags`), and the statements that apply, in file order, are the recipe. Everything after
that is as before: the flag allow-list, the paths, the sources.

**The object cache key follows the evaluated recipe**, as it did for the colon form: the key of a library source
is made from its text, the compile arguments (the public and private flags that the applying statements made),
the compiler and the module (ADR-024), and not from the text of `gocpp.mod`. So the same recipe for two targets
has two keys when the applying statements differ, a block that is not taken changes nothing (a recipe with and
without it has the same key for a target that does not take it, and a comment or a reformatting changes
nothing), and a source named only in a branch that is not taken is not even looked up.

### The file reader, `gocpp mod vendor`, `Format`

`gocpp.mod` still keeps a brace block opaque (ADR-024; `Block.Raw`), so other tools read the file without
knowing the language. The reader that finds the end of the block gains one rule: **a line that starts with the
closer of an open block and ends with `{` closes one block and opens the next** (`} else {`,
`} else if windows {`), and a line that starts with that closer and has other words closes it (the recipe reader
reports the words). Before, `} else {` was an opener, one level too deep. The rule works on words, so a brace in
a quote or in a comment counts for nothing; a block that is not closed is an error of the file
(`unterminated block started at gocpp.mod:3:1; the "{" at gocpp.mod:5:12 is not closed`: it now names the
brace that stays open). `Format`, the writer of `gocpp.mod`, writes the lines of a block as they are and
does not touch a recipe, and `gocpp mod vendor` copies the whole module folder, the library's own `gocpp.mod` with its
recipe included, byte for byte; a vendored library and the same library from the module cache build the same
way (both are tested). An older gocpp reading a file with `} else {` in a recipe sees an unterminated block:
that is an error, not a wrong build, and the file is written for the newer gocpp.

### Errors

Every error of the reader of a recipe has the line and the column of the word at fault, in `gocpp.mod` (`gocpp.mod:12:3:`; a
column that counts runes). Each is reported, and the others are still read:

| mistake | the message starts |
|---|---|
| a verb that gocpp does not know | `unknown statement "frobnicate"` |
| the old form | `a condition is written as a block, if windows { ... }, since ADR-129` |
| `if` without a condition, or with no `{` at the end | `if without a condition`, `an if line ends with {` |
| a condition that is not a build constraint | `the condition "windows &&" is not valid: unexpected end of expression`, at the offset |
| `else` without `if`, `else` after `else`, `} else` without `{` | `"else" without "if"`, `else after else` |
| something after a `}` other than `else` | `after a } only else may follow on the line` |
| a `}` with no open block, an `if` that is not closed | `unexpected }`, `the if block is not closed: a } is missing` |
| a block that is not an `if`, and `{ }` ( ) in a statement | `a block in a recipe is an if block`, `blocks inside a recipe ...` |
| `if windows{`, `if //go:build windows {` (the `{` glued or in a comment), `if(windows) {` | `put a space before the "{"`, `the "{" of an if line is not in a comment`, `write a space after "if"` |
| more than 100 errors | `more than 100 errors in the recipe: the rest is not read` |
| more than 4 levels, more than 1024 conditions | `if blocks nest more than 4 deep` |

## Alternatives

- **Keep the colon form next to the blocks** (rejected): two spellings for good; see above.
- **A recipe grammar of its own for conditions** (`if os == "windows"`, `if windows,linux`) (rejected): a third
  expression language in the project, to learn and to keep equal to the tags of the loader; the build
  constraint has the architecture, `unix`, `go1.N` and the tags of `-tags` already.
- **Statements on the same line** (`if windows { link gdi32 }`) (rejected): a second shape of the same thing, and
  the end of the block is no longer the first word of a line, which the opaque reader of the file relies on.
- **Depth 1 only** (rejected for the reason above); **no limit** (rejected: a recipe can be hostile).
- **An error for an unknown tag** (rejected): a build line does not complain, and a recipe written for a
  newer Go or gocpp may name an architecture that this one has not heard of. The warning above catches the
  typo without that.
- **Evaluate in the reader, for one target** (rejected): the reader would need the target, which the reader of a
  file does not have, and the statements with their conditions are easier to test and to print than a list
  that was already cut down.

## Consequences

Good: a recipe reads like Go; a platform's libraries are one block; the conditions mean what they mean in a
`//go:build` line; an error points at the column; the key cannot go stale.

Bad:
- A tag that is a typo of nothing known (`if myfeature`, without `-tags myfeature`) is warned about, not
  refused; a typo of a tag that is defined (`if linuxx`) is warned about too, but a tag that is spelled right
  and is false for this target (`if windows` on Linux) is, as it should be, silent.
- The columns of an error are counted in runes (the file reader's convention), and the printer of diagnostics
  uses them as bytes, so the caret of an error is a few columns off after non-ASCII text on the same line.
- The fixtures that used the colon form (the demos, `tests/interop`, `tests/pkgs`, the cookbook) were
  rewritten in the same change. An old gocpp cannot read the new recipes (see above).
