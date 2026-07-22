# nn

A terminal-based text editor written in Go, inspired by nano. Arrow-key navigation,
clipboard operations, syntax highlighting, search, undo/redo, fuzzy file finder,
file sidebar, git integration, and multi-theme support.

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

### Global (editor + sidebar)

| Key | Action |
|---|---|
| `F1` | Open help popup |
| `Ctrl+P` | Fuzzy file finder |
| `Ctrl+S` | Save file |
| `Ctrl+N` | New file |
| `Ctrl+B` | Toggle / hide sidebar |
| `Ctrl+R` | Toggle hidden files in sidebar |
| `Ctrl+Q` | Quit |
| `Alt+T` | Cycle theme |
| `Alt+← / →` | Switch tabs |
| `Shift+Alt+← / →` | Resize sidebar |

### Editor mode

| Key | Action |
|---|---|
| `Ctrl+F` | Search (Tab/Shift+Tab to cycle matches) |
| `Ctrl+Z` / `Ctrl+Y` | Undo / Redo |
| `Ctrl+O` | Open file |
| `Ctrl+C` / `V` / `X` | Copy / Paste / Cut |
| `Ctrl+A` | Select all |
| `Ctrl+D` | Duplicate line |
| `Ctrl+/` | Toggle comment |
| `Ctrl+W` | Close tab |
| `↑ ↓ ← →` | Move cursor |
| `Shift+↑ ↓ ← →` | Extend selection |
| `Ctrl+← / →` | Word jump |
| `Home` / `End` | Start / end of line |
| `Page Up` / `Page Down` | Scroll page |
| `Enter` | Newline |
| `Backspace` | Delete backward |
| `Delete` / `Shift+Delete` | Delete forward / delete line |
| `Tab` / `Shift+Tab` | Indent / unindent (selection) |
| `Escape` | Clear selection |

### File sidebar

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate files |
| `Enter` | Open file / enter directory |
| `←` | Go to parent directory |
| `→` | Enter directory |
| `Ctrl+F` | Filter files by name |
| `Delete` | Delete selected file (with confirmation) |
| `Escape` | Clear filter / switch to editor |
| `Tab` | Switch to editor |
| `Home` / `End` | First / last entry |
| `Page Up` / `Page Down` | Scroll page |

### Search mode (Ctrl+F in editor)

| Key | Action |
|---|---|
| `Tab` | Next match |
| `Shift+Tab` | Previous match |
| `Enter` / `Escape` | Close search |

### File filter mode (Ctrl+F in sidebar)

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate filtered list |
| `Enter` | Open selected file |
| `Escape` | Cancel filter |

## Fuzzy File Finder (Ctrl+P)

Opens a searchable overlay to quickly jump to any file in the project tree.
- Start typing to filter files by name
- `↑` / `↓` to navigate results
- `Enter` to open the selected file
- `Escape` to cancel

Fuzzy matching scores files by consecutive character matches and word boundaries,
with depth and dependency-directory penalties. Results are ranked and capped at 200.

## Syntax Highlighting

Chroma-based highlighting for: Go, Python, JavaScript, TypeScript, JSX, Rust,
C/C++, Java, Ruby, Bash, YAML, JSON, TOML, Markdown, HTML, CSS, SQL, PHP,
XML (and SVG, XSL), Lua, Zig, Swift, Kotlin, Scala, Elixir, R, Svelte, Vue,
Protocol Buffers, Terraform, Dockerfile, Nix, Fish, OCaml, and more.

## Themes

| Key | Action |
|---|---|
| `Alt+T` | Cycle through available themes |

Available themes: Catppuccin Mocha, Catppuccin Latte, Tokyo Night,
Tokyo Night Day, Dracula, One Dark, Ayu Light, Gruvbox Dark.

Theme, dotfiles preference, and sidebar width are persisted to
`~/.config/nn/settings.json` on quit.

## Settings

Persisted to `~/.config/nn/settings.json`:

- `theme` — last selected theme name
- `hide_dotfiles` — whether hidden files are shown in the sidebar
- `sidebar_width` — sidebar width in characters

## Sidebar

- File tree navigation with arrow keys and Enter
- Filter files by name with `Ctrl+F`
- Directory position memory (remembers your scroll position per directory)
- Delete files with `Delete` (with confirmation prompt)
- Toggle hidden files with `Ctrl+R` (hidden by default)
- Resize with `Shift+Alt+← / →` (clamped 15–60)

## Status Bar

Shows mode indicator (`EDITOR` / `SIDEBAR`), filename (`•` when modified),
transient messages (green background), `F1 help` hint, git branch,
and cursor position (`row:col`). Press `F1` for the full keybinding reference.

## Git Integration

- Git status computed via pure Go (`go-git/v5`), no external `git` binary
- Branch name shown in the status bar as `@branch`
- Sidebar filenames colored by git status:
  - Red — untracked or deleted
  - Orange — modified (or unsaved active file)
  - Green — staged/added
- Editor gutter shows a thin bar indicator (`▎`) per line:
  - Orange — line modified
  - Green — line added
  - Gray — unchanged
- Line-level diff uses LCS (longest common subsequence) for accurate markers
- Git status refreshes on save, file open, tab switch, undo/redo, and
  every 10 seconds via background ticker

## Mouse

- Click in editor to position cursor
- Click in sidebar to select file (then press Enter to open)
- Scroll wheel: scrolls editor / navigates sidebar
