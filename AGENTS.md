# Agents

## Build & Verify

```sh
go build -o nn .
go vet ./...
```

## Package Structure

All `.go` files are `package main` — no subdirectories, no tests.

## File Map

| File | Purpose |
|---|---|
| `main.go` | `Editor` struct (16-74), app init, layout, help, input handling, settings |
| `buffer.go` | `FileTab`, buffer ops, undo/redo, clipboard, indent/comment |
| `keys.go` | Key bindings — `handleEditorKey` / `handleSidebarKey` switches |
| `draw.go` | Screen drawing — editor, sidebar, status bar, help overlay |
| `syntax.go` | Chroma syntax highlighting — `langFromExt()`, `tokenTypeColor()`, tokenizer |
| `theme.go` | 8 themes (Catppuccin, Tokyo Night, Dracula, etc.) |
| `git.go` | Pure-Go git via `go-git/v5` — branch, line-level diff, sidebar colors |

## Undo Rules

- `e.saveUndoState(opInsert)` — char insert
- `e.saveUndoState(opDeleteBk)` — backspace
- `e.saveUndoState(opDeleteFd)` — forward delete
- `e.saveUndoState(opNone)` — all bulk operations (indent, unindent, toggle comment, cut, paste, delete line, duplicate line)

## Syntax Highlighting

- `langFromExt()` maps extensions → Chroma lexer name. No shebang detection — extensionless files get no highlighting.
- Chroma token types map to theme colors in `tokenTypeColor()`. The `Generic*` range is notably used for markdown formatting.
- Token cache debounce: `tokenizeDebounce = 5`. After any bulk buffer modification, **explicitly invalidate** with `e.openFiles[e.activeTab].syntaxTokens = nil` to force re-tokenization on next draw.

## Selection

- `e.selection.Active` controls whether a selection exists.
- When `End.X == 0 && End.Y > Start.Y`, the endpoint is at the start of the line *below* the last visually selected line. Iterate `startY..endY-1` to avoid affecting that line.

## tcell Quirks

- **Ctrl+/** arrives as `tcell.KeyCtrlUnderscore` (ASCII 31, the same as Ctrl+_). Some terminals may send `KeyRune '/'` with Ctrl modifier — check both.
- **Shift+Tab** is `tcell.KeyBacktab`.

## Active Buffer

`e.buffer` / `e.cursor` / `e.filename` / `e.modified` are the **active** tab's state. They're synced to `e.openFiles[e.activeTab]` via `saveCurrentTab()` / `restoreTab()`. Always call `saveCurrentTab()` before switching tabs and `restoreTab()` after.

## Settings

Theme, `hide_dotfiles`, and `sidebar_width` persist to `~/.config/nn/settings.json` via `loadSettings()` / `saveSettings()`.
