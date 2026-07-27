---
name: production-build
description: Build a stripped, statically linked production binary with the smallest possible size
---

## What to do

Run the production build command and verify the output:

```sh
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o nn .
```

- `CGO_ENABLED=0` — statically linked, no libc dependency
- `-trimpath` — removes absolute file paths from the binary
- `-ldflags="-s -w"` — strips DWARF debug info and symbol table (saves ~3 MB)

After building, run:
- `ls -lh nn` and `file nn` to confirm it's a stripped static ELF
- `go vet ./...` to check for issues

## When to use

Use when the user asks for a production build, release build, smaller binary, or stripping the binary.

## Details

**Result**: 15 MB → **12 MB** (static, stripped).

**Code breakdown** (of the 4.3 MB `.text` section):

| Dependency | Size | Notes |
|---|---|---|
| chroma/v2 lexers | 345 KB | 200+ language lexer rule tables |
| go-git/v5 | 250 KB | Pure-Go git implementation |
| regexp2 | 230 KB | Regex engine (used by chroma) |
| tview | 151 KB | TUI framework |
| tcell | 103 KB | Terminal graphics |
| std/crypto (FIPS) | 753 KB | Go 1.25 FIPS crypto bundle |
| runtime | 437 KB | GC, scheduler, maps |
| main package | 137 KB | Your actual code (3.4 %) |

**Further size reduction options** (if needed):

1. **UPX**: `upx -9 nn` compresses to ~4-5 MB. Adds ~50-100ms startup decompression.
2. **Chroma lexer trimming**: ~345 KB from lexer rule tables. Only register the ~15 languages you edit.
3. **Replace go-git with `os/exec` git calls**: Saves ~250 KB. Adds runtime dependency on `git`.
