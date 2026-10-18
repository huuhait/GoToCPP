The patch files of tools/portstd, one per package: patches/<import path with / written as __>.rules,
for example patches/math__bits.rules. They are search-and-replace rules on Go's source
text, applied before anything else, so an author sees the text of the upstream file. The format is
in docs/guides/porting.md ("Patch files").
