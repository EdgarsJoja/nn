# Agents

## Build & Verify

```sh
go build ./...
go vet ./...
```

## Code Conventions

- All source files are in `package main` — no subdirectories.
- Editor state lives in the `Editor` struct (`main.go:16-74`).
- Per-tab state lives in `FileTab` (`buffer.go:24-35`).
- Key bindings go in `keys.go` in the `handleEditorKey` / `handleSidebarKey` switch.
- Buffer manipulation methods go in `buffer.go`.
- Screen drawing goes in `draw.go`.
- Syntax / language detection goes in `syntax.go`.
- Themes go in `theme.go`.
- Git integration goes in `git.go`.

## Undo

- Bulk operations (indent, unindent, toggle comment, delete line, duplicate line)
  call `e.saveUndoState(opNone)` before making changes.
- Character insertions call `e.saveUndoState(opInsert)`.
- Backspace calls `e.saveUndoState(opDeleteBk)`.
- Forward delete calls `e.saveUndoState(opDeleteFd)`.
- After modifying the buffer, call `e.setModified()`.
- For operations that modify many lines at once, also invalidate the syntax
  token cache with `e.openFiles[e.activeTab].syntaxTokens = nil`.

## Syntax Highlighting

- Language detection: `langFromExt()` in `syntax.go` maps filenames/extensions to
  Chroma lexer names.
- Token colors: `tokenTypeColor()` maps Chroma `TokenType` to theme colors.
- The token cache is invalidated by setting `tab.syntaxTokens = nil` or using
  the debounce mechanism (`tokenizeDebounce = 5`).

## Selection

- `e.selection.Active` controls whether there is an active selection.
- When the selection end has `X == 0` and `Y > start.Y`, the endpoint is at the
  start of the line *below* the last visually selected line — operations should
  adjust `endY` down by 1 to avoid affecting that line.
