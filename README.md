# nn

A terminal-based text editor written in Go, inspired by nano. Arrow-key navigation,
clipboard operations, syntax highlighting, search, undo/redo, file sidebar, and
multi-theme support.

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

### Editor mode

| Key | Action |
|---|---|
| `F1` | Open help popup |
| `Ctrl+F` | Search (Tab/Shift+Tab to cycle matches) |
| `Ctrl+Z` / `Ctrl+Y` | Undo / Redo |
| `Ctrl+S` | Save file |
| `Ctrl+O` | Open file |
| `Ctrl+N` | New file |
| `Ctrl+C` | Copy selection |
| `Ctrl+V` | Paste |
| `Ctrl+X` | Cut selection |
| `Ctrl+A` | Select all |
| `Ctrl+D` | Duplicate line |
| `Ctrl+W` | Close tab |
| `Ctrl+B` | Toggle / focus sidebar |
| `Ctrl+Q` | Quit |
| `Alt+T` | Cycle theme |
| `Alt+← / →` | Switch tabs |
| `↑ ↓ ← →` | Move cursor |
| `Shift+↑ ↓ ← →` | Extend selection |
| `Ctrl+← / →` | Word jump |
| `Home` / `End` | Start / end of line |
| `Shift+Home` / `Shift+End` | Select to line start/end |
| `Page Up` / `Page Down` | Scroll page |
| `Enter` | Newline |
| `Backspace` | Delete backward |
| `Delete` | Delete forward |
| `Shift+Delete` | Delete line |
| `Tab` | Insert tab |
| `Escape` | Clear selection |

### File sidebar

| Key | Action |
|---|---|
| `Ctrl+F` | Filter files by name |
| `↑` / `↓` | Navigate files |
| `←` | Go to parent directory |
| `→` | Enter directory |
| `Home` / `End` | Jump to first / last entry |
| `Page Up` / `Page Down` | Scroll page |
| `Enter` | Open file / enter directory |
| `Delete` | Delete selected file or directory (with confirmation) |
| `Ctrl+R` | Toggle hidden files |
| `Escape` | Clear filter / switch to editor |
| `Tab` / `Ctrl+B` | Switch to editor |
| `Alt+← / →` | Switch tabs |

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

Theme and dotfiles preference are persisted to `~/.config/nn/settings.json`.

## Settings

Persisted to `~/.config/nn/settings.json`:

- `theme` — last selected theme name
- `hide_dotfiles` — whether hidden files are shown in the sidebar

## Sidebar

- File tree navigation with arrow keys and Enter
- Filter files by name with `Ctrl+F`
- Directory position memory (remembers your scroll position per directory)
- Delete files/directories with `Delete` (with confirmation prompt)
- Toggle hidden files with `Ctrl+R` (hidden by default)

## Status Bar

Shows mode indicator, filename, context-sensitive shortcuts, cursor position
(`row:col`), and transient messages. Shortcuts truncate to fit terminal width;
press `F1` for the full keybinding reference.

## Mouse

- Click in editor to position cursor
- Click in sidebar to select file (then press Enter to open)
- Scroll wheel: scrolls editor / navigates sidebar
