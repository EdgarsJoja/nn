# nn

A terminal-based text editor written in Go, inspired by nano and NvChad. Arrow-key navigation,
Ctrl+C/V/X copy-paste-cut, Shift+arrow text selection, and a toggleable sidebar for file browsing.

Built with [tview](https://github.com/rivo/tview) and [tcell](https://github.com/gdamore/tcell).

## Build

```sh
go build -o nn .
```

Requires Go 1.24+.

## Run

```sh
./nn              # start with empty buffer
./nn file.txt     # open existing file
```

## Key Bindings

### Global

| Key | Action |
|---|---|
| `Ctrl+Q` | Quit |
| `Ctrl+S` | Save file |
| `Ctrl+O` | Open file (shows filename prompt) |
| `Ctrl+N` | New file (shows filename prompt, creates empty buffer) |
| `Ctrl+B` | Toggle / focus sidebar |
| `Escape` | Clear selection / exit sidebar |

### Editor mode (default)

| Key | Action |
|---|---|
| `↑ ↓ ← →` | Move cursor |
| `Shift+↑ ↓ ← →` | Extend selection |
| `Shift+Home` / `Shift+End` | Select to line start/end |
| `Home` / `End` | Start / end of line |
| `Page Up` / `Page Down` | Scroll page |
| `Ctrl+C` | Copy selection |
| `Ctrl+V` | Paste |
| `Ctrl+X` | Cut selection |
| `Ctrl+A` | Select all |
| `Ctrl+D` | Delete current line |
| `Enter` | Newline |
| `Backspace` | Delete backward |
| `Delete` | Delete forward |
| `Tab` | Insert tab |

### File sidebar

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate files |
| `Home` / `End` | Jump to first / last entry |
| `Enter` | Open file / enter directory |
| `Delete` | Delete selected file or directory |
| `Escape` / `Tab` / `Ctrl+B` | Switch to editor / hide sidebar |

### Mouse

- Click in editor to position cursor
- Click in sidebar to select file (then press Enter to open)
- Scroll wheel: scrolls editor / navigates sidebar

## Theme

Catppuccin Mocha-inspired dark theme with:
- Line numbers in the editor gutter
- Mode indicator in the status bar (`NORMAL` / `SIDEBAR`)
- Visual selection highlighting
