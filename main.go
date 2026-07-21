package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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
	sidebarFilter string
	sidebarAllFiles []string

	mode      string
	inputMode string

	message  string
	msgTimer int

	running   bool
	themeIdx  int
	openFiles []*FileTab
	activeTab int

	searchQuery   string
	searchMatches []Point
	searchIdx     int
}

func (e *Editor) msg(text string) {
	e.message = text
	e.msgTimer = 80
}

func (e *Editor) showInput(mode, prompt string) {
	e.inputMode = mode
	e.inputField.SetLabel(prompt)
	e.inputField.SetText("")
	e.mainFlex.RemoveItem(e.statusBox)
	e.mainFlex.AddItem(e.inputField, 1, 0, true)
	e.app.SetFocus(e.inputField)
}

func (e *Editor) submitInput() {
	val := e.inputField.GetText()
	e.mainFlex.RemoveItem(e.inputField)
	e.mainFlex.AddItem(e.statusBox, 1, 0, false)
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
		e.saveCurrentTab()
		e.openFiles = append(e.openFiles, &FileTab{
			filename: val,
			buffer:   []string{""},
		})
		e.restoreTab(len(e.openFiles) - 1)
		e.refreshDir()
		e.msg("new file: " + val)
	}
	e.inputMode = ""
}

func (e *Editor) cancelInput() {
	switch e.inputMode {
	case "search":
		e.searchQuery = ""
		e.searchMatches = nil
		e.searchIdx = 0
	case "filesearch":
		e.sidebarFilter = ""
		e.sidebarFiles = e.sidebarAllFiles
		if e.sidebarIdx >= len(e.sidebarFiles) {
			e.sidebarIdx = 0
		}
	}
	e.inputMode = ""
	e.mainFlex.RemoveItem(e.inputField)
	e.mainFlex.AddItem(e.statusBox, 1, 0, false)
	if e.mode == "sidebar" {
		e.app.SetFocus(e.sidebar)
	} else {
		e.app.SetFocus(e.editorBox)
	}
}

func (e *Editor) cmdSave() {
	if e.filename == "" {
		e.showInput("save", "save as: ")
		return
	}
	e.saveFile(e.filename)
}

func (e *Editor) cmdOpen()   { e.showInput("open", "open: ") }
func (e *Editor) cmdNew()    { e.showInput("new", "new file: ") }
func (e *Editor) cmdSearch() { e.showInput("search", "search: ") }

func (e *Editor) updateSearch(query string) {
	e.searchQuery = query
	e.searchMatches = e.findMatches(query)
	if len(e.searchMatches) > 0 {
		e.searchIdx = 0
		e.cursor = e.searchMatches[0]
		e.cursorInBounds()
		e.scrollCursor()
	} else {
		e.searchIdx = 0
	}
}

func (e *Editor) searchPrev() {
	if len(e.searchMatches) == 0 {
		return
	}
	e.searchIdx = (e.searchIdx - 1 + len(e.searchMatches)) % len(e.searchMatches)
	e.cursor = e.searchMatches[e.searchIdx]
	e.cursorInBounds()
	e.scrollCursor()
}

func (e *Editor) searchNext() {
	if len(e.searchMatches) == 0 {
		return
	}
	e.searchIdx = (e.searchIdx + 1) % len(e.searchMatches)
	e.cursor = e.searchMatches[e.searchIdx]
	e.cursorInBounds()
	e.scrollCursor()
}

func (e *Editor) updateFileFilter(query string) {
	e.sidebarFilter = query
	if query == "" {
		e.sidebarFiles = e.sidebarAllFiles
	} else {
		lower := strings.ToLower(query)
		e.sidebarFiles = nil
		for _, f := range e.sidebarAllFiles {
			if strings.Contains(strings.ToLower(f), lower) {
				e.sidebarFiles = append(e.sidebarFiles, f)
			}
		}
	}
	if e.sidebarIdx >= len(e.sidebarFiles) {
		e.sidebarIdx = 0
	}
	e.sidebarOff = 0
}

func (e *Editor) findMatches(query string) []Point {
	if query == "" {
		return nil
	}
	var matches []Point
	lower := strings.ToLower(query)
	for y, line := range e.buffer {
		lowerLine := strings.ToLower(line)
		start := 0
		for {
			idx := strings.Index(lowerLine[start:], lower)
			if idx == -1 {
				break
			}
			matches = append(matches, Point{X: start + idx, Y: y})
			start += idx + 1
		}
	}
	return matches
}

func (e *Editor) refreshDir() {
	entries, err := os.ReadDir(e.sidebarDir)
	if err != nil {
		e.msg("error: " + err.Error())
		return
	}
	e.sidebarAllFiles = nil
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
	e.sidebarAllFiles = append(e.sidebarAllFiles, "../")
	e.sidebarAllFiles = append(e.sidebarAllFiles, dirs...)
	e.sidebarAllFiles = append(e.sidebarAllFiles, files...)
	if e.sidebarFilter != "" {
		e.updateFileFilter(e.sidebarFilter)
	} else {
		e.sidebarFiles = e.sidebarAllFiles
	}
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
	return len([]rune(e.buffer[y]))
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

func (e *Editor) getEditHeight() int {
	_, _, _, h := e.editorBox.GetRect()
	return h - 1
}

func (e *Editor) getSidebarHeight() int {
	_, _, _, height := e.sidebar.GetRect()
	return height - 1
}

func (e *Editor) rebuildSidebarVisibility() {
	inputWasActive := e.inputMode != ""
	e.pages.RemovePage("main")
	e.buildLayout()
	if inputWasActive {
		e.mainFlex.RemoveItem(e.statusBox)
		e.mainFlex.AddItem(e.inputField, 1, 0, true)
		e.app.SetFocus(e.inputField)
	}
	e.pages.SendToFront("main")
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

func settingsPath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "nn", "settings.json"), nil
}

type settings struct {
	Theme string `json:"theme"`
}

func (e *Editor) saveSettings() {
	path, err := settingsPath()
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	s := settings{Theme: themes[e.themeIdx].Name}
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0644)
}

func (e *Editor) loadSettings() {
	path, err := settingsPath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var s settings
	if err := json.Unmarshal(data, &s); err != nil {
		return
	}
	for i, t := range themes {
		if t.Name == s.Theme {
			e.themeIdx = i
			return
		}
	}
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
	e.loadSettings()
	e.activeTab = 0
	e.openFiles = []*FileTab{{buffer: []string{""}}}
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
	e.inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if e.inputMode == "filesearch" {
			switch event.Key() {
			case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
				e.handleSidebarKey(event)
				return nil
			case tcell.KeyEnter:
				e.onSidebarSelectItem(e.sidebarIdx)
				e.sidebarFilter = ""
				e.sidebarFiles = e.sidebarAllFiles
				e.mainFlex.RemoveItem(e.inputField)
				e.mainFlex.AddItem(e.statusBox, 1, 0, false)
				e.inputMode = ""
				if e.mode == "editor" {
					e.app.SetFocus(e.editorBox)
				} else {
					e.app.SetFocus(e.sidebar)
				}
				return nil
			case tcell.KeyEscape:
				e.cancelInput()
				return nil
			}
		}
		return event
	})
	e.inputField.SetDoneFunc(func(key tcell.Key) {
		switch e.inputMode {
	case "search":
		switch key {
		case tcell.KeyTab:
			e.searchNext()
		case tcell.KeyBacktab:
			e.searchPrev()
		case tcell.KeyEnter:
			e.searchQuery = ""
			e.searchMatches = nil
			e.searchIdx = 0
			e.mainFlex.RemoveItem(e.inputField)
			e.mainFlex.AddItem(e.statusBox, 1, 0, false)
			e.inputMode = ""
			e.app.SetFocus(e.editorBox)
		case tcell.KeyEscape:
			e.cancelInput()
		}
		default:
			if key == tcell.KeyEnter {
				e.submitInput()
			} else if key == tcell.KeyEscape {
				e.cancelInput()
			}
		}
	})
	e.inputField.SetChangedFunc(func(text string) {
		switch e.inputMode {
		case "search":
			e.updateSearch(text)
		case "filesearch":
			e.updateFileFilter(text)
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

	e.pages.AddPage("main", e.mainFlex, true, true)
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
	log.SetPrefix("nn: ")

	e := &Editor{}
	e.Init()
	if len(os.Args) > 1 {
		e.loadFile(os.Args[1])
	}
	if err := e.Run(); err != nil {
		log.Fatal(err)
	}
}
