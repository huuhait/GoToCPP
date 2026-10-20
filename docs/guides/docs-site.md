# The documentation site

The pages you are reading are a static site made from the Markdown files of `docs/` by
`tools/docsite`, a program of this repository. It needs no network and no library: the Markdown
renderer, the page template and the search are in the tool.

```sh
make docs                    # docs/ into site/ (git-ignored), after checking every link
make docs-check              # only check: exit status 1 on a broken link, an unsupported construct
                             # or an entry of docs/site/known-links.txt that is not broken any more
go run ./tools/docsite -h    # the flags: -docs, -out, -check, -repo, -branch
```

Open `site/index.html` in a browser; the site works from a folder (`file://`) and from any static
host. The search box filters the titles and headings of all pages and the names of the standard
library.

## What is in it

- Every `docs/**/*.md` file is a page. `README.md` becomes the `index.html` of its folder (unless the
  folder has an `index.md`), and every link between pages is rewritten from `.md` to `.html`.
- `docs/site/nav.txt` is the sidebar: sections (`# Title`), pages in order (`path`, or
  `path | Title`), and `~ glob` for pages that belong to a section but are not listed. A page that is
  in no section is an error, so a new page cannot be forgotten.
- The **standard library reference** is generated, not written: for each package of `stdlib/`
  (without the internal ones) the tool calls `internal/doc`, the code of `gocpp doc -all`, and makes
  one page with anchors for every declaration (`stdlib/strings.html#Builder.WriteString`). Link to
  the generated pages with their `.html` path, for example `../stdlib/strings.html#Builder`.
- A link that leaves `docs/` (to `examples/`, `tools/`, `plans/`) must lead to a file of the
  repository, and is written as a link to that file on the web (`-repo` and `-branch` say where).
- The output folder (`-out`) is replaced as a whole: the new site is written next to it and swapped in when
  it is complete. A folder that is not empty must carry the `.docsite` file of an earlier run, and a folder
  under `docs/`, one that holds `docs/` or the repository, and a symbolic link are refused.

## What is checked

`go test ./tools/docsite` checks:

- **Links**: every relative link and every anchor of every page, on the Markdown and again on the
  written HTML (`TestSiteLinks`, `TestOutputLinks`); the sidebar covers every page.
- **Code**: in the tour, in the guides for Go and C++ developers, for the interop examples and for Windows, every
  `go` block must say what it is, on the line before it:

  | Marker | Meaning | Checked by |
  |---|---|---|
  | `<!-- program: 08 id=x go exit=1 check -->` | a whole program; `08` is the phase it needs (`GOCPP_GATE_PHASE`); `id=x` names it; `go` says that it is valid Go and must print the same with the go command; `exit=N` is its exit status; `check` means "do not run it" | `gocpp check`, `gocpp run` and the output block, and `go run` |
  | `<!-- output -->` | the `text` block that follows is the program's output | `TestSnippetsRun` |
  | `<!-- emitted: x -->` | the `cpp` block that follows shows lines of what `gocpp emit` prints for program `x` (`...` stands for left-out lines) | `TestSnippetEmitted` |
  | `<!-- rejected: x -->` | the `text` block that follows quotes lines that `gocpp check` prints for program `x` without `GOCPP_GATE_PHASE` (the error that names the phase) | `TestSnippetsRejected` |
  | `<!-- excerpt: examples/x/main.gp -->` | the block that follows shows lines of that file, in order | `TestSnippetExcerpts` |
  | `<!-- illustration: reason -->` | a fragment, or code that cannot run here | nothing: the reason says why |

  A program marked with a phase above the compiler's own must be rejected without the switch, so the
  note that tells readers to set `GOCPP_GATE_PHASE` is true (`TestSnippetsPhase`). When the last
  phase is finished, `TestGateNotesCurrent` fails until those notes are removed.
- **Facts about gocpp** that the pages state: every `gocpp <command>` named in a code span or an sh block
  must exist (a planned command answers "not available yet" and may not be mentioned as working,
  `TestCommandsExist`); the standard packages listed as present (an HTML comment `stdlib: present` with the names) are exactly
  the packages of `stdlib/` and import, the ones listed as missing (`stdlib: missing`) fail to
  import with "not available yet" even with `GOCPP_GATE_PHASE=15` (`TestStdlibClaims`), and the package
  count in the text is the real one; "Phases 00 to NN are finished" is `internal/version.Phase`
  (`TestStatusText`); the `GOCPP_GATE_PHASE=NN` that a lesson names is the phase of its program, which
  must be rejected one phase lower (`TestLessonGateNotes`, `TestSnippetsPhase`); the platforms of the FAQ are
  the hosts of `internal/toolchain` (`TestPlatforms`).
- **Renderer**: a construct the renderer cannot render faithfully (an unclosed comment or code fence, a
  footnote, an undefined `[x][ref]`, a table row with more cells than its header, which GitHub also
  drops) is an error of `docsite -check`, not a silent loss. Escape a `|` inside a code span of a table
  cell as `\|`.
- The interop spec and the cookbook have their own marker convention (`<!-- test: PATH -->`), checked
  by `TestDocSnippets` in `tests/`.

## Writing a page

Write Markdown as for GitHub: headings, tables, fenced code, lists, links and images. The renderer
knows that; raw HTML is limited to a few inline tags. A page needs a level-1 heading (it is the title).
Link to other pages with relative `.md` paths and to anchors with GitHub's names (lower case,
punctuation dropped, spaces as hyphens). Mark a feature that needs a switch today as
**experimental**, and name the real switch. Add the page to `docs/site/nav.txt`.
