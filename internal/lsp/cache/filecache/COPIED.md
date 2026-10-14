# Ideas copied from gopls

`filecache.go` follows the design of gopls's file cache,
`golang.org/x/tools/gopls/internal/filecache/filecache.go` (gopls v0.21.1, BSD-3-Clause, the
license of `internal/lsp/protocol/LICENSE`). No code was copied as it is; these ideas were:

- one folder per version of the program, whose name has the hash of the running executable
  (`executableHash` is the same as gopls's `hashExecutable`), so that a new binary never reads
  the entries of an old one, and a trim that also removes the entries of old versions;
- one file per entry, named by the hex key and the kind, under a folder of the first byte of the
  key;
- a check of the content of each entry when it is read (gopls checks the SHA-256 hash of its
  content-addressed file), so that a file cut short or damaged is a miss;
- the trim: remove the entries not used for a maximum age, then the oldest by modification time
  until the cache is under its size budget, then the empty folders; Get marks an entry as used by
  setting its modification time at most once an hour (gopls and the go command do the same).

What is different: gocpp writes each entry to a temporary file and renames it over the entry
(gopls writes an index file and a content-addressed file without renames), it keeps the length
and the hash in a header of the entry itself, and it trims at most once per `TrimEvery` for all
processes together (a `trim.txt` file with the time of the last trim, like the go command's
build cache), instead of gopls's loop every 5 minutes to 6 hours per process. Its trim looks only
at the names that the cache writes (version folders, key folders, entries and their temporary
files) and gives the entries of other versions one day, where gopls walks its whole root.
