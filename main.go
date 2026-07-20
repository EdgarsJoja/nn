package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Point struct{ X, Y int }

type Selection struct {
	Start, End Point
	Active     bool
}

type Editor struct {
	app   *tview.Application
	pages *tview.Pages

	buffer    []string
	cursor    Point
	offset    Point
	selection Selection
	clipboard string
	filename  string
	modified  bool

	editorBox  *tview.Box
	statusBox  *tview.Box
	sidebar    *tview.Box
	inputField *tview.InputField
	mainFlex   *tview.Flex

	sidebarDir   string
	sidebarFiles []string
	sidebarIdx   int
	sidebarOff   int
	showSidebar  bool
	sidebarWidth int
	hideDotfiles bool

	mode      string
	inputMode string

	message  string
	msgTimer int

	running  bool
	themeIdx int
}

type Theme struct {
	Name, Base, Mantle, Surface0, Surface1, Surface2, Text, Subtext0, Blue, Green, Overlay0 string
}

var themes = []Theme{
	{"Catppuccin Mocha", "#1e1e2e", "#181825", "#313244", "#45475a", "#585b70", "#cdd6f4", "#a6adc8", "#89b4fa", "#a6e3a1", "#6c7086"},
	{"Catppuccin Latte", "#eff1f5", "#e6e9ef", "#ccd0da", "#bcc0cc", "#acb0be", "#4c4f69", "#6c6f85", "#1e66f5", "#40a02b", "#9ca0b0"},
	{"Tokyo Night", "#1a1b26", "#16161e", "#24283b", "#2f3546", "#3b4261", "#c0caf5", "#a9b1d6", "#7aa2f7", "#9ece6a", "#565f89"},
	{"Tokyo Night Day", "#e1e2e7", "#d4d5db", "#c4c5cd", "#b4b5be", "#a4a5ae", "#3760bf", "#6172b0", "#2e7de9", "#587539", "#848cb5"},
	{"Dracula", "#282a36", "#21222c", "#343746", "#44475a", "#555879", "#f8f8f2", "#cfcfc2", "#8be9fd", "#50fa7b", "#6272a4"},
	{"One Dark", "#282c34", "#21252b", "#2c313a", "#353b45", "#3e4451", "#abb2bf", "#828997", "#61afef", "#98c379", "#5c6370"},
	{"Ayu Light", "#fafafa", "#f0f0f0", "#e6e6e6", "#d9d9d9", "#cccccc", "#5c6166", "#8a9199", "#39bae6", "#86b300", "#abb0b6"},
	{"Gruvbox Dark", "#282828", "#1d2021", "#32302f", "#3c3836", "#504945", "#ebdbb2", "#a89984", "#458588", "#98971a", "#928374"},
}

var (
	colBase, colMantle, colSurface0, colSurface1 tcell.Color
	colSurface2, colText, colSubtext0             tcell.Color
	colBlue, colGreen, colOverlay0                tcell.Color
)

func applyTheme(t Theme) {
	colBase = tcell.GetColor(t.Base)
	colMantle = tcell.GetColor(t.Mantle)
	colSurface0 = tcell.GetColor(t.Surface0)
	colSurface1 = tcell.GetColor(t.Surface1)
	colSurface2 = tcell.GetColor(t.Surface2)
	colText = tcell.GetColor(t.Text)
	colSubtext0 = tcell.GetColor(t.Subtext0)
	colBlue = tcell.GetColor(t.Blue)
	colGreen = tcell.GetColor(t.Green)
	colOverlay0 = tcell.GetColor(t.Overlay0)
}

func (e *Editor) Init() {
	e.app = tview.NewApplication()
	e.app.EnableMouse(true)
	e.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			if e.mode == "editor" {
				e.copySel()
			}
			return nil
		}
		return event
	})

	e.buffer = []string{""}
	e.cursor = Point{}
	e.offset = Point{}
	e.mode = "editor"
	e.showSidebar = true
	e.sidebarWidth = 28
	e.sidebarDir = "."
	e.running = true
	e.themeIdx = 0
	applyTheme(themes[e.themeIdx])

	e.makeWidgets()
	e.refreshDir()
	e.buildLayout()
	e.app.SetRoot(e.pages, true)
	e.app.SetFocus(e.editorBox)
}

func (e *Editor) makeWidgets() {
	e.editorBox = tview.NewBox()
	e.editorBox.SetBackgroundColor(colBase)
	e.editorBox.SetDrawFunc(e.drawEditor)
	e.editorBox.SetInputCapture(e.handleEditorKey)

	e.sidebar = tview.NewBox()
	e.sidebar.SetBackgroundColor(colMantle)
	e.sidebar.SetDrawFunc(e.drawSidebar)
	e.sidebar.SetInputCapture(e.handleSidebarKey)

	e.statusBox = tview.NewBox()
	e.statusBox.SetBackgroundColor(colSurface1)

	e.inputField = tview.NewInputField()
	e.inputField.SetLabelColor(colGreen)
	e.inputField.SetFieldBackgroundColor(colSurface0)
	e.inputField.SetFieldTextColor(colText)
	e.inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			e.submitInput()
		} else if key == tcell.KeyEscape {
			e.cancelInput()
		}
	})

	e.pages = tview.NewPages()
}

func (e *Editor) buildLayout() {
	content := tview.NewFlex().SetDirection(tview.FlexColumn)
	if e.showSidebar {
		content.AddItem(e.sidebar, e.sidebarWidth, 0, false)
	}
	content.AddItem(e.editorBox, 0, 1, true)

	e.mainFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	e.mainFlex.AddItem(content, 0, 1, true)
	e.mainFlex.AddItem(e.statusBox, 1, 0, false)

	inputFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	inputFlex.AddItem(nil, 0, 1, false)
	inputRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	inputRow.AddItem(nil, 0, 1, false)
	inputRow.AddItem(e.inputField, 50, 0, true)
	inputRow.AddItem(nil, 0, 1, false)
	inputFlex.AddItem(inputRow, 1, 0, true)
	inputFlex.AddItem(nil, 0, 1, false)

	e.pages.AddPage("main", e.mainFlex, true, true)
	e.pages.AddPage("input", inputFlex, true, false)
}

func (e *Editor) msg(text string) {
	e.message = text
	e.msgTimer = 80
}

func (e *Editor) cycleTheme() {
	e.themeIdx = (e.themeIdx + 1) % len(themes)
	applyTheme(themes[e.themeIdx])
	e.editorBox.SetBackgroundColor(colBase)
	e.sidebar.SetBackgroundColor(colMantle)
	e.statusBox.SetBackgroundColor(colSurface1)
	e.inputField.SetFieldBackgroundColor(colSurface0)
	e.msg("theme: " + themes[e.themeIdx].Name)
}

func (e *Editor) refreshDir() {
	entries, err := os.ReadDir(e.sidebarDir)
	if err != nil {
		e.msg("error: " + err.Error())
		return
	}
	e.sidebarFiles = nil
	var dirs, files []string
	for _, entry := range entries {
		name := entry.Name()
		if e.hideDotfiles && strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			dirs = append(dirs, name+"/")
		} else {
			files = append(files, name)
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)
	e.sidebarFiles = append(e.sidebarFiles, "../")
	e.sidebarFiles = append(e.sidebarFiles, dirs...)
	e.sidebarFiles = append(e.sidebarFiles, files...)
	if e.sidebarIdx >= len(e.sidebarFiles) {
		e.sidebarIdx = 0
	}
}

func (e *Editor) sidebarPath() string {
	if e.sidebarIdx < 0 || e.sidebarIdx >= len(e.sidebarFiles) {
		return ""
	}
	return filepath.Join(e.sidebarDir, e.sidebarFiles[e.sidebarIdx])
}

func (e *Editor) lineLen(y int) int {
	if y < 0 || y >= len(e.buffer) {
		return 0
	}
	return len(e.buffer[y])
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (e *Editor) cursorInBounds() {
	e.cursor.Y = clamp(e.cursor.Y, 0, len(e.buffer)-1)
	e.cursor.X = clamp(e.cursor.X, 0, e.lineLen(e.cursor.Y))
}

func (e *Editor) maxLineNumW() int {
	n := len(e.buffer)
	w := 1
	for n >= 10 {
		n /= 10
		w++
	}
	if w < 2 {
		w = 2
	}
	return w
}

func (e *Editor) loadFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		e.msg("error: " + err.Error())
		return
	}
	content := string(data)
	e.buffer = strings.Split(content, "\n")
	if len(e.buffer) == 0 {
		e.buffer = []string{""}
	}
	e.filename = path
	e.cursor = Point{}
	e.offset = Point{}
	e.modified = false
	e.selection = Selection{}
	e.msg("opened " + filepath.Base(path))
}

func (e *Editor) saveFile(path string) {
	data := []byte(strings.Join(e.buffer, "\n"))
	if err := os.WriteFile(path, data, 0644); err != nil {
		e.msg("error: " + err.Error())
		return
	}
	e.filename = path
	e.modified = false
	e.msg("saved " + filepath.Base(path))
}

func (e *Editor) cmdSave() {
	if e.filename == "" {
		e.showInput("save", "save as: ")
		return
	}
	e.saveFile(e.filename)
}

func (e *Editor) cmdOpen()  { e.showInput("open", "open: ") }
func (e *Editor) cmdNew()   { e.showInput("new", "new file: ") }

func (e *Editor) showInput(mode, prompt string) {
	e.inputMode = mode
	e.inputField.SetLabel(prompt)
	e.inputField.SetText("")
	e.pages.SwitchToPage("input")
	e.app.SetFocus(e.inputField)
}

func (e *Editor) submitInput() {
	val := e.inputField.GetText()
	e.pages.SwitchToPage("main")
	if e.mode == "sidebar" {
		e.app.SetFocus(e.sidebar)
	} else {
		e.app.SetFocus(e.editorBox)
	}

	switch e.inputMode {
	case "save":
		e.saveFile(val)
	case "open":
		e.loadFile(val)
	case "new":
		e.buffer = []string{""}
		e.cursor = Point{}
		e.offset = Point{}
		e.filename = val
		e.modified = false
		e.selection = Selection{}
		e.refreshDir()
		e.msg("new file: " + val)
	}
	e.inputMode = ""
}

func (e *Editor) cancelInput() {
	e.inputMode = ""
	e.pages.SwitchToPage("main")
	if e.mode == "sidebar" {
		e.app.SetFocus(e.sidebar)
	} else {
		e.app.SetFocus(e.editorBox)
	}
}

func (e *Editor) deleteSidebarFile() {
	p := e.sidebarPath()
	if p == "" {
		return
	}
	if err := os.RemoveAll(p); err != nil {
		e.msg("error: " + err.Error())
		return
	}
	e.msg("deleted " + filepath.Base(p))
	e.refreshDir()
}

func (e *Editor) onSidebarSelectItem(idx int) {
	if idx < 0 || idx >= len(e.sidebarFiles) {
		return
	}
	name := e.sidebarFiles[idx]
	p := filepath.Join(e.sidebarDir, name)
	info, err := os.Stat(p)
	if err == nil && info.IsDir() {
		e.sidebarDir = p
		e.sidebarIdx = 0
		e.refreshDir()
		e.app.SetFocus(e.sidebar)
		return
	}
	e.loadFile(p)
	e.mode = "editor"
	e.app.SetFocus(e.editorBox)
}

func (e *Editor) sidebarEnterDir() {
	e.onSidebarSelectItem(e.sidebarIdx)
}

func (e *Editor) selectedText() string {
	if !e.selection.Active {
		return ""
	}
	start, end := e.selection.Start, e.selection.End
	if start.Y > end.Y || (start.Y == end.Y && start.X > end.X) {
		start, end = end, start
	}
	var lines []string
	for y := start.Y; y <= end.Y && y < len(e.buffer); y++ {
		switch {
		case y == start.Y && y == end.Y:
			lines = append(lines, e.buffer[y][start.X:end.X])
		case y == start.Y:
			lines = append(lines, e.buffer[y][start.X:])
		case y == end.Y:
			lines = append(lines, e.buffer[y][:end.X])
		default:
			lines = append(lines, e.buffer[y])
		}
	}
	return strings.Join(lines, "\n")
}

func (e *Editor) deleteSelection() {
	if !e.selection.Active {
		return
	}
	start, end := e.selection.Start, e.selection.End
	if start.Y > end.Y || (start.Y == end.Y && start.X > end.X) {
		start, end = end, start
	}
	if start.Y == end.Y {
		line := e.buffer[start.Y]
		e.buffer[start.Y] = line[:start.X] + line[end.X:]
	} else {
		first := e.buffer[start.Y][:start.X]
		last := e.buffer[end.Y][end.X:]
		e.buffer[start.Y] = first + last
		e.buffer = append(e.buffer[:start.Y+1], e.buffer[end.Y+1:]...)
	}
	e.cursor = start
	e.selection = Selection{}
	e.modified = true
}

func (e *Editor) insertText(text string) {
	e.deleteSelection()
	for _, ch := range text {
		if ch == '\n' {
			line := e.buffer[e.cursor.Y]
			rest := line[e.cursor.X:]
			e.buffer[e.cursor.Y] = line[:e.cursor.X]
			tail := make([]string, len(e.buffer[e.cursor.Y+1:]))
			copy(tail, e.buffer[e.cursor.Y+1:])
			e.buffer = append(e.buffer[:e.cursor.Y+1], "")
			e.buffer = append(e.buffer, tail...)
			e.buffer[e.cursor.Y+1] = rest
			e.cursor.Y++
			e.cursor.X = 0
		} else {
			line := e.buffer[e.cursor.Y]
			e.buffer[e.cursor.Y] = line[:e.cursor.X] + string(ch) + line[e.cursor.X:]
			e.cursor.X++
		}
	}
	e.modified = true
}

func (e *Editor) copySel() {
	if t := e.selectedText(); t != "" {
		e.clipboard = t
		e.msg("copied")
	}
}

func (e *Editor) cutSel() {
	if t := e.selectedText(); t != "" {
		e.clipboard = t
		e.deleteSelection()
		e.msg("cut")
	}
}

func (e *Editor) pasteClip() {
	if e.clipboard == "" {
		return
	}
	e.insertText(e.clipboard)
}

func (e *Editor) deleteBackward() {
	if e.selection.Active {
		e.deleteSelection()
		return
	}
	if e.cursor.X > 0 {
		line := e.buffer[e.cursor.Y]
		e.buffer[e.cursor.Y] = line[:e.cursor.X-1] + line[e.cursor.X:]
		e.cursor.X--
		e.modified = true
	} else if e.cursor.Y > 0 {
		prev := len(e.buffer[e.cursor.Y-1])
		e.buffer[e.cursor.Y-1] += e.buffer[e.cursor.Y]
		e.buffer = append(e.buffer[:e.cursor.Y], e.buffer[e.cursor.Y+1:]...)
		e.cursor.Y--
		e.cursor.X = prev
		e.modified = true
	}
}

func (e *Editor) deleteForward() {
	if e.selection.Active {
		e.deleteSelection()
		return
	}
	line := e.buffer[e.cursor.Y]
	if e.cursor.X < len(line) {
		e.buffer[e.cursor.Y] = line[:e.cursor.X] + line[e.cursor.X+1:]
		e.modified = true
	} else if e.cursor.Y < len(e.buffer)-1 {
		e.buffer[e.cursor.Y] += e.buffer[e.cursor.Y+1]
		e.buffer = append(e.buffer[:e.cursor.Y+1], e.buffer[e.cursor.Y+2:]...)
		e.modified = true
	}
}

func (e *Editor) deleteLine() {
	if len(e.buffer) == 1 {
		e.buffer[0] = ""
		e.cursor.X = 0
		e.modified = true
		e.selection = Selection{}
		return
	}
	y := e.cursor.Y
	e.buffer = append(e.buffer[:y], e.buffer[y+1:]...)
	if y >= len(e.buffer) {
		e.cursor.Y = len(e.buffer) - 1
	}
	e.cursor.X = 0
	e.cursorInBounds()
	e.modified = true
	e.selection = Selection{}
}

func (e *Editor) duplicateLine() {
	line := e.buffer[e.cursor.Y]
	cp := make([]byte, len(line))
	copy(cp, line)
	dup := string(cp)
	e.buffer = append(e.buffer, "")
	copy(e.buffer[e.cursor.Y+2:], e.buffer[e.cursor.Y+1:])
	e.buffer[e.cursor.Y+1] = dup
	e.cursor.Y++
	e.cursor.X = 0
	e.modified = true
	e.selection = Selection{}
}

func (e *Editor) moveWordLeft() {
	if e.cursor.X <= 0 {
		if e.cursor.Y > 0 {
			e.cursor.Y--
			e.cursor.X = e.lineLen(e.cursor.Y)
		}
		return
	}
	x := e.cursor.X - 1
	line := e.buffer[e.cursor.Y]
	for x > 0 && (line[x] == ' ' || line[x] == '\t') {
		x--
	}
	for x > 0 && line[x-1] != ' ' && line[x-1] != '\t' {
		x--
	}
	e.cursor.X = x
}

func (e *Editor) moveWordRight() {
	line := e.buffer[e.cursor.Y]
	if e.cursor.X >= len(line) {
		if e.cursor.Y < len(e.buffer)-1 {
			e.cursor.Y++
			e.cursor.X = 0
		}
		return
	}
	x := e.cursor.X
	for x < len(line) && line[x] != ' ' && line[x] != '\t' {
		x++
	}
	for x < len(line) && (line[x] == ' ' || line[x] == '\t') {
		x++
	}
	e.cursor.X = x
}

func (e *Editor) selectExtend(p Point) {
	if !e.selection.Active {
		e.selection.Start = e.cursor
		e.selection.Active = true
	}
	e.selection.End = p
}

func (e *Editor) inSelection(p Point) bool {
	if !e.selection.Active {
		return false
	}
	start, end := e.selection.Start, e.selection.End
	if start.Y > end.Y || (start.Y == end.Y && start.X > end.X) {
		start, end = end, start
	}
	switch {
	case p.Y > start.Y && p.Y < end.Y:
		return true
	case p.Y == start.Y && p.Y == end.Y:
		return p.X >= start.X && p.X < end.X
	case p.Y == start.Y:
		return p.X >= start.X
	case p.Y == end.Y:
		return p.X < end.X
	default:
		return false
	}
}

func (e *Editor) drawSidebar(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			screen.SetContent(x+dx, y+dy, ' ', nil, tcell.StyleDefault.Background(colMantle))
		}
	}

	header := " Files "
	for dx, ch := range header {
		if dx < width {
			screen.SetContent(x+dx, y, ch, nil, tcell.StyleDefault.Background(colSurface0).Foreground(colBlue))
		}
	}

	rows := height - 1
	for i := 0; i < rows && i+e.sidebarOff < len(e.sidebarFiles); i++ {
		idx := i + e.sidebarOff
		name := e.sidebarFiles[idx]
		rowY := y + i + 1
		disp := " " + name
		if len(disp) > width {
			disp = disp[:width]
		}
		disp += strings.Repeat(" ", width-len(disp))

		if e.mode == "sidebar" && idx == e.sidebarIdx {
			for dx, ch := range disp {
				screen.SetContent(x+dx, rowY, ch, nil, tcell.StyleDefault.Background(colSurface1).Foreground(colText))
			}
		} else {
			for dx, ch := range disp {
				screen.SetContent(x+dx, rowY, ch, nil, tcell.StyleDefault.Background(colMantle).Foreground(colSubtext0))
			}
		}
	}

	return x, y, width, height
}

func (e *Editor) drawEditor(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
	sw := 0

	editX := x
	if e.showSidebar {
		sw = e.sidebarWidth
		editX = sw + 1
	}
	editW := width - (editX - x)

	lnW := e.maxLineNumW()
	gutterW := lnW + 2

	for dy := 0; dy < height; dy++ {
		by := dy + e.offset.Y
		screenY := y + dy

		if by >= len(e.buffer) {
			break
		}

		for gx := 0; gx < gutterW && gx < editW; gx++ {
			screen.SetContent(editX+gx, screenY, ' ', nil, tcell.StyleDefault.Background(colBase).Foreground(colOverlay0))
		}
		lnStr := fmt.Sprintf("%*d ", lnW, by+1)
		for gi, ch := range lnStr {
			if gi >= gutterW {
				break
			}
			screen.SetContent(editX+gi, screenY, ch, nil, tcell.StyleDefault.Background(colBase).Foreground(colOverlay0))
		}

		line := e.buffer[by]
		textStart := editX + gutterW
		for dx := 0; dx < editW-gutterW && dx+e.offset.X < len(line); dx++ {
			ch := rune(line[dx+e.offset.X])
			st := tcell.StyleDefault.Background(colBase).Foreground(colText)
			if e.selection.Active && e.inSelection(Point{X: dx + e.offset.X, Y: by}) {
				st = tcell.StyleDefault.Background(colSurface1).Foreground(colText)
			}
			screen.SetContent(textStart+dx, screenY, ch, nil, st)
		}
		if e.selection.Active && e.inSelection(Point{X: len(line), Y: by}) {
			fillStart := textStart + clamp(len(line)-e.offset.X, 0, editW-gutterW)
			for fx := fillStart; fx < textStart+editW-gutterW; fx++ {
				screen.SetContent(fx, screenY, ' ', nil, tcell.StyleDefault.Background(colSurface1))
			}
		}
	}

	// Draw cursor
	if e.mode == "editor" {
		cx := editX + gutterW + e.cursor.X - e.offset.X
		cy := y + e.cursor.Y - e.offset.Y
		if cx >= editX && cx < editX+editW && cy >= y && cy < y+height {
			screen.ShowCursor(cx, cy)
		}
	} else {
		screen.HideCursor()
	}

	return x, y, width, height
}

func (e *Editor) drawStatusBar(screen tcell.Screen) {
	w, h := screen.Size()
	statusY := h - 1

	for cx := 0; cx < w; cx++ {
		screen.SetContent(cx, statusY, ' ', nil, tcell.StyleDefault.Background(colSurface1))
	}

	modeTag := " NORMAL "
	modeBg := colSurface1
	modeFg := colText
	if e.mode == "sidebar" {
		modeTag = " SIDEBAR "
		modeBg = colBlue
		modeFg = colBase
	}
	for cx, ch := range modeTag {
		screen.SetContent(cx, statusY, ch, nil, tcell.StyleDefault.Background(modeBg).Foreground(modeFg))
	}

	shortcuts := ""
	if e.mode == "sidebar" {
		dot := "hide"
		if e.hideDotfiles {
			dot = "show"
		}
		shortcuts = " ^H " + dot + " dotfiles │ Del delete │ Enter open │ Esc edit "
	} else {
		shortcuts = " ^S save │ ^O open │ ^N new │ ^C copy │ ^V paste │ ^X cut │ ^D dup │ ^B files │ Alt+T theme "
	}

	right := ""
	if e.msgTimer > 0 && e.message != "" {
		right = " " + e.message + " "
		e.msgTimer--
	} else {
		right = fmt.Sprintf(" %d:%d ", e.cursor.Y+1, e.cursor.X+1)
	}

	modeEnd := len(modeTag)
	nameX := modeEnd + 2

	name := "[No Name]"
	if e.filename != "" {
		name = filepath.Base(e.filename)
	}
	if e.modified {
		name += " •"
	}

	rightW := len(right)
	shortcutsW := len(shortcuts)
	availW := w - nameX - rightW

	shortcutsX := nameX
	if availW > len(name)+2 {
		for cx, ch := range name {
			sx := nameX + cx
			if sx >= w {
				break
			}
			screen.SetContent(sx, statusY, ch, nil, tcell.StyleDefault.Background(colSurface1).Foreground(colText))
		}
		shortcutsX = nameX + len(name) + 1
	} else {
		shortcutsX = nameX
	}

	if shortcutsX+shortcutsW+rightW < w {
		for cx, ch := range shortcuts {
			sx := shortcutsX + cx
			if sx >= w {
				break
			}
			st := tcell.StyleDefault.Background(colSurface1).Foreground(colSubtext0)
			if ch == '│' {
				st = tcell.StyleDefault.Background(colSurface1).Foreground(colOverlay0)
			}
			screen.SetContent(sx, statusY, ch, nil, st)
		}
	}

	if e.msgTimer > -1 && e.message != "" {
		for cx, ch := range right {
			sx := (w - rightW) + cx
			if sx >= w {
				break
			}
			screen.SetContent(sx, statusY, ch, nil, tcell.StyleDefault.Background(colGreen).Foreground(colBase))
		}
	} else {
		for cx, ch := range right {
			sx := (w - rightW) + cx
			if sx >= w {
				break
			}
			screen.SetContent(sx, statusY, ch, nil, tcell.StyleDefault.Background(colSurface1).Foreground(colSubtext0))
		}
	}
}

func (e *Editor) handleEditorKey(event *tcell.EventKey) *tcell.EventKey {
	key := event.Key()
	mod := event.Modifiers()
	hasShift := mod&tcell.ModShift != 0
	hasCtrl := mod&tcell.ModCtrl != 0

	if key == tcell.KeyRune && mod&tcell.ModAlt != 0 {
		switch event.Rune() {
		case 'h', 'H':
			e.hideDotfiles = !e.hideDotfiles
			e.refreshDir()
			return nil
		case 't', 'T':
			e.cycleTheme()
			return nil
		}
	}

	switch key {
	case tcell.KeyCtrlV:
		e.pasteClip()
		return nil
	case tcell.KeyCtrlX:
		e.cutSel()
		return nil
	case tcell.KeyCtrlS:
		e.cmdSave()
		return nil
	case tcell.KeyCtrlO:
		e.cmdOpen()
		return nil
	case tcell.KeyCtrlN:
		e.cmdNew()
		return nil
	case tcell.KeyCtrlA:
		lastY := len(e.buffer) - 1
		lastX := e.lineLen(lastY)
		e.selection = Selection{Start: Point{}, End: Point{X: lastX, Y: lastY}, Active: true}
		return nil
	case tcell.KeyCtrlD:
		e.duplicateLine()
		return nil
	case tcell.KeyCtrlQ:
		e.app.Stop()
		e.running = false
		return nil
	case tcell.KeyCtrlB:
		if !e.showSidebar {
			e.showSidebar = true
			e.mode = "sidebar"
			e.rebuildSidebarVisibility()
			e.app.SetFocus(e.sidebar)
		} else {
			e.mode = "sidebar"
			e.app.SetFocus(e.sidebar)
		}
		return nil
	case tcell.KeyEscape:
		e.selection = Selection{}
		return nil
	case tcell.KeyUp:
		if !hasShift && e.selection.Active {
			e.selection = Selection{}
		}
		if hasShift {
			e.selectExtend(Point{X: e.cursor.X, Y: e.cursor.Y - 1})
		}
		e.cursor.Y--
		e.cursorInBounds()
		if hasShift {
			e.selection.End = e.cursor
		}
	case tcell.KeyDown:
		if !hasShift && e.selection.Active {
			e.selection = Selection{}
		}
		if hasShift {
			e.selectExtend(Point{X: e.cursor.X, Y: e.cursor.Y + 1})
		}
		e.cursor.Y++
		e.cursorInBounds()
		if hasShift {
			e.selection.End = e.cursor
		}
	case tcell.KeyLeft:
		if hasCtrl {
			if !hasShift && e.selection.Active {
				e.selection = Selection{}
			}
			if hasShift && !e.selection.Active {
				e.selection.Start = e.cursor
				e.selection.Active = true
			}
			e.moveWordLeft()
			if hasShift {
				e.selection.End = e.cursor
			}
		} else {
			if !hasShift && e.selection.Active {
				e.selection = Selection{}
			}
			if hasShift {
				e.selectExtend(Point{X: e.cursor.X - 1, Y: e.cursor.Y})
			}
			e.cursor.X--
			if e.cursor.X < 0 {
				if e.cursor.Y > 0 {
					e.cursor.Y--
					e.cursor.X = e.lineLen(e.cursor.Y)
				} else {
					e.cursor.X = 0
				}
			}
			e.cursorInBounds()
			if hasShift {
				e.selection.End = e.cursor
			}
		}
	case tcell.KeyRight:
		if hasCtrl {
			if !hasShift && e.selection.Active {
				e.selection = Selection{}
			}
			if hasShift && !e.selection.Active {
				e.selection.Start = e.cursor
				e.selection.Active = true
			}
			e.moveWordRight()
			if hasShift {
				e.selection.End = e.cursor
			}
		} else {
			if !hasShift && e.selection.Active {
				e.selection = Selection{}
			}
			if hasShift {
				e.selectExtend(Point{X: e.cursor.X + 1, Y: e.cursor.Y})
			}
			e.cursor.X++
			if e.cursor.X > e.lineLen(e.cursor.Y) {
				if e.cursor.Y < len(e.buffer)-1 {
					e.cursor.Y++
					e.cursor.X = 0
				} else {
					e.cursor.X = e.lineLen(e.cursor.Y)
				}
			}
			e.cursorInBounds()
			if hasShift {
				e.selection.End = e.cursor
			}
		}
	case tcell.KeyHome:
		e.cursor.X = 0
		if hasShift && e.selection.Active {
			e.selection.End = e.cursor
		} else {
			e.selection = Selection{}
		}
	case tcell.KeyEnd:
		e.cursor.X = e.lineLen(e.cursor.Y)
		if hasShift && e.selection.Active {
			e.selection.End = e.cursor
		} else {
			e.selection = Selection{}
		}
	case tcell.KeyPgUp:
		_, _, _, height := e.editorBox.GetRect()
		e.cursor.Y -= height
		e.cursorInBounds()
		e.selection = Selection{}
	case tcell.KeyPgDn:
		_, _, _, height := e.editorBox.GetRect()
		e.cursor.Y += height
		e.cursorInBounds()
		e.selection = Selection{}
	case tcell.KeyEnter:
		e.insertText("\n")
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		e.deleteBackward()
	case tcell.KeyDelete:
		if hasShift {
			e.deleteLine()
		} else {
			e.deleteForward()
		}
	case tcell.KeyTab:
		e.insertText("\t")
	case tcell.KeyRune:
		e.insertText(string(event.Rune()))
	default:
		return event
	}

	e.cursorInBounds()
	e.scrollCursor()
	return nil
}

func (e *Editor) scrollCursor() {
	_, _, width, height := e.editorBox.GetRect()
	sw := 0
	if e.showSidebar {
		sw = e.sidebarWidth + 2
	}
	ew := width - sw
	lnW := e.maxLineNumW() + 2
	ew -= lnW
	if ew < 1 {
		ew = 1
	}

	if e.cursor.Y < e.offset.Y {
		e.offset.Y = e.cursor.Y
	}
	if e.cursor.Y >= e.offset.Y+height {
		e.offset.Y = e.cursor.Y - height + 1
	}
	if e.cursor.X < e.offset.X {
		e.offset.X = e.cursor.X
	}
	if e.cursor.X >= e.offset.X+ew {
		e.offset.X = e.cursor.X - ew + 1
	}
}

func (e *Editor) handleSidebarKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyRune && event.Modifiers()&tcell.ModAlt != 0 {
		switch event.Rune() {
		case 'h', 'H':
			e.hideDotfiles = !e.hideDotfiles
			e.refreshDir()
			return nil
		case 't', 'T':
			e.cycleTheme()
			return nil
		}
	}

	switch event.Key() {
	case tcell.KeyUp:
		if e.sidebarIdx > 0 {
			e.sidebarIdx--
		}
	case tcell.KeyDown:
		if e.sidebarIdx < len(e.sidebarFiles)-1 {
			e.sidebarIdx++
		}
	case tcell.KeyHome:
		e.sidebarIdx = 0
	case tcell.KeyEnd:
		e.sidebarIdx = len(e.sidebarFiles) - 1
	case tcell.KeyEnter:
		e.sidebarEnterDir()
		return nil
	case tcell.KeyDelete:
		e.deleteSidebarFile()
		return nil
	case tcell.KeyCtrlH:
		e.hideDotfiles = !e.hideDotfiles
		e.refreshDir()
		return nil
	case tcell.KeyEscape:
		e.mode = "editor"
		e.app.SetFocus(e.editorBox)
		return nil
	case tcell.KeyCtrlB:
		e.showSidebar = false
		e.mode = "editor"
		e.rebuildSidebarVisibility()
		e.app.SetFocus(e.editorBox)
		return nil
	case tcell.KeyTab:
		e.mode = "editor"
		e.app.SetFocus(e.editorBox)
		return nil
	case tcell.KeyCtrlQ:
		e.app.Stop()
		e.running = false
		return nil
	case tcell.KeyCtrlS:
		if e.filename == "" {
			e.showInput("save", "save as: ")
		} else {
			e.saveFile(e.filename)
		}
		return nil
	}

	// Scroll sidebar
	rows := e.getSidebarHeight()
	if e.sidebarIdx < e.sidebarOff {
		e.sidebarOff = e.sidebarIdx
	}
	if e.sidebarIdx >= e.sidebarOff+rows {
		e.sidebarOff = e.sidebarIdx - rows + 1
	}

	return event
}

func (e *Editor) getSidebarHeight() int {
	_, _, _, height := e.sidebar.GetRect()
	return height - 1
}

func (e *Editor) rebuildSidebarVisibility() {
	e.pages.RemovePage("main")
	e.buildLayout()
	e.pages.SendToFront("main")
}

func (e *Editor) Run() error {
	e.statusBox.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		e.drawStatusBar(screen)
		return x, y, width, height
	})
	return e.app.Run()
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("nnano: ")

	e := &Editor{}
	e.Init()
	if len(os.Args) > 1 {
		e.loadFile(os.Args[1])
	}
	if err := e.Run(); err != nil {
		log.Fatal(err)
	}
}
