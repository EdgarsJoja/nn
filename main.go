package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

	showHelp   bool
	helpLines  []string
	helpOffset int

	sidebarDir   string
	sidebarFiles []string
	sidebarIdx   int
	sidebarOff   int
	showSidebar  bool
	sidebarWidth int
	hideDotfiles bool
	sidebarFilter string
	sidebarAllFiles []string
	sidebarDirIdx map[string]int

	initialDir string

	mode      string
	inputMode string

	message  string
	msgTimer int

	running   bool
	themeIdx  int
	openFiles []*FileTab
	activeTab int
	tabOffset int

	searchQuery   string
	searchMatches []Point
	searchIdx     int

	undoStack []undoState
	redoStack []undoState
	lastOp    int

	pendingDelPath string
	git           *gitInfo

	gitDirty          bool
	tokenizeDebounce  int

	fuzzyResults []fuzzyResult
	fuzzyIdx     int
	fuzzyOff     int
	showFuzzy    bool
	fuzzyFiles   []fuzzyFileInfo
	fuzzyQuery   string
	fuzzyPrevQuery string
	fuzzyPrevCandidates []int

	textSearchResults []TextSearchResult
	textSearchIdx     int
	textSearchOff     int
	textSearchHOff    int
	showTextSearch    bool
	textSearchQuery   string
	textSearchFiles   []string
	textSearchCache   map[string][]string
	textSearchTimer   *time.Timer

	showThemePicker   bool
	themePickerIdx    int
	themePickerOff    int
	themePickerPrevIdx int

	showDiff  bool
	diffHunks []diffHunk
	diffLines []diffLine
	diffIdx   int
	diffOff   int
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
		e.refreshDir()
	case "open":
		e.loadFile(val)
	case "new":
		e.saveCurrentTab()
		absPath, _ := filepath.Abs(val)
		e.openFiles = append(e.openFiles, &FileTab{
			filename: val,
			filepath: absPath,
			buffer:   []string{""},
		})
		e.restoreTab(len(e.openFiles) - 1)
		e.refreshDir()
		e.mode = "editor"
		e.app.SetFocus(e.editorBox)
		e.msg("new file: " + val)
	case "confirm":
		if val == "y" || val == "Y" {
			if err := os.RemoveAll(e.pendingDelPath); err != nil {
				e.msg("error: " + err.Error())
			} else {
				e.msg("deleted " + filepath.Base(e.pendingDelPath))
			}
			e.refreshDir()
		}
		e.pendingDelPath = ""
	}
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
	case "confirm":
		e.pendingDelPath = ""
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

func (e *Editor) restoreStatusBar() {
	if e.inputMode != "" {
		e.inputMode = ""
		e.mainFlex.RemoveItem(e.inputField)
		e.mainFlex.AddItem(e.statusBox, 1, 0, false)
	}
}

func (e *Editor) cmdSave() {
	if e.filename == "" {
		e.showInput("save", "save as: ")
		return
	}
	e.saveFile(e.filename)
	e.refreshGit()
	e.refreshDir()
}

func (e *Editor) cmdOpen()   { e.showInput("open", "open: ") }
func (e *Editor) cmdNew()    { e.showInput("new", "new file: ") }
func (e *Editor) cmdSearch() { e.showInput("search", "search: ") }

func (e *Editor) cmdRevealInSidebar() {
	if e.filename == "" {
		return
	}
	dir := filepath.Dir(e.filename)
	if dir == "." {
		abs, _ := os.Getwd()
		dir = abs
	}
	e.sidebarDirIdx[e.sidebarDir] = e.sidebarIdx
	e.sidebarDir = dir
	e.sidebarIdx = 0
	e.sidebarOff = 0
	if !e.showSidebar {
		e.showSidebar = true
		e.rebuildSidebarVisibility()
	}
	e.mode = "sidebar"
	e.app.SetFocus(e.sidebar)
	e.refreshDir()
	e.msg("revealed in sidebar")
}

func (e *Editor) cmdTextSearch() {
	e.restoreStatusBar()
	e.textSearchQuery = ""
	e.textSearchResults = nil
	e.textSearchIdx = 0
	e.textSearchOff = 0
	e.textSearchHOff = 0
	e.textSearchFiles = nil
	e.textSearchCache = nil
	e.showTextSearch = true
	e.mode = "editor"
	e.app.SetFocus(e.editorBox)
	e.msg("text search: type to search across files")
}

func (e *Editor) textSearchOpen() {
	if len(e.textSearchResults) == 0 {
		e.msg("text search: no matches")
		return
	}
	r := e.textSearchResults[e.textSearchIdx]
	path := filepath.Join(e.initialDir, r.filePath)
	e.showTextSearch = false
	e.mode = "editor"
	e.app.SetFocus(e.editorBox)
	e.loadFile(path)
	for y := range e.buffer {
		if y+1 == r.lineNum {
			e.cursor.Y = y
			e.cursor.X = r.matchCol
			e.cursorInBounds()
			e.scrollCursor()
			break
		}
	}
}

func (e *Editor) textSearchCancel() {
	if e.textSearchTimer != nil {
		e.textSearchTimer.Stop()
		e.textSearchTimer = nil
	}
	e.showTextSearch = false
	e.textSearchResults = nil
	e.textSearchQuery = ""
	e.msg("")
}

func (e *Editor) textSearchUp() {
	if e.textSearchIdx > 0 {
		e.textSearchIdx--
	}
	if e.textSearchIdx < e.textSearchOff {
		e.textSearchOff = e.textSearchIdx
	}
}

func (e *Editor) textSearchDown() {
	if e.textSearchIdx < len(e.textSearchResults)-1 {
		e.textSearchIdx++
	}
	rows := e.textSearchRowCount()
	if e.textSearchIdx >= e.textSearchOff+rows {
		e.textSearchOff = e.textSearchIdx - rows + 1
	}
}

func (e *Editor) textSearchPgUp() {
	rows := e.textSearchRowCount()
	e.textSearchIdx -= rows
	if e.textSearchIdx < 0 {
		e.textSearchIdx = 0
	}
	e.textSearchOff = e.textSearchIdx
}

func (e *Editor) textSearchPgDn() {
	rows := e.textSearchRowCount()
	last := len(e.textSearchResults) - 1
	e.textSearchIdx += rows
	if e.textSearchIdx > last {
		e.textSearchIdx = last
	}
	if e.textSearchIdx >= e.textSearchOff+rows {
		e.textSearchOff = e.textSearchIdx - rows + 1
	}
}

func (e *Editor) textSearchHome() {
	e.textSearchIdx = 0
	e.textSearchOff = 0
}

func (e *Editor) textSearchEnd() {
	e.textSearchIdx = len(e.textSearchResults) - 1
	rows := e.textSearchRowCount()
	if e.textSearchIdx >= e.textSearchOff+rows {
		e.textSearchOff = e.textSearchIdx - rows + 1
	}
}

func (e *Editor) textSearchRowCount() int {
	_, _, _, h := e.editorBox.GetRect()
	boxH := h - 4
	if boxH < 4 {
		boxH = 4
	}
	n := (boxH - 3) / 5
	if n < 1 {
		n = 1
	}
	return n
}

func (e *Editor) textSearchLeft() {
	if e.textSearchHOff > 0 {
		e.textSearchHOff -= 8
		if e.textSearchHOff < 0 {
			e.textSearchHOff = 0
		}
	}
}

func (e *Editor) textSearchRight() {
	if e.textSearchIdx >= 0 && e.textSearchIdx < len(e.textSearchResults) {
		lineLen := len([]rune(e.textSearchResults[e.textSearchIdx].line))
		_, _, w, _ := e.editorBox.GetRect()
		boxW := w - 4
		if boxW < 40 {
			boxW = 40
		}
		innerW := boxW - 4
		if lineLen > innerW && e.textSearchHOff < lineLen-innerW {
			e.textSearchHOff += 8
			if e.textSearchHOff > lineLen-innerW {
				e.textSearchHOff = lineLen - innerW
			}
		}
	}
}

func (e *Editor) cmdThemePicker() {
	e.restoreStatusBar()
	e.themePickerIdx = e.themeIdx
	e.themePickerOff = 0
	e.themePickerPrevIdx = e.themeIdx
	e.showThemePicker = true
	e.mode = "editor"
	e.app.SetFocus(e.editorBox)
}

func (e *Editor) themePickerUp() {
	if e.themePickerIdx > 0 {
		e.themePickerIdx--
	}
	if e.themePickerIdx < e.themePickerOff {
		e.themePickerOff = e.themePickerIdx
	}
	e.applyThemePreview()
}

func (e *Editor) themePickerDown() {
	if e.themePickerIdx < len(themes)-1 {
		e.themePickerIdx++
	}
	rows := e.themePickerRowCount()
	if e.themePickerIdx >= e.themePickerOff+rows {
		e.themePickerOff = e.themePickerIdx - rows + 1
	}
	e.applyThemePreview()
}

func (e *Editor) themePickerRowCount() int {
	_, _, _, h := e.editorBox.GetRect()
	rows := h - 6
	if rows < 3 {
		rows = 3
	}
	return rows
}

func (e *Editor) applyThemePreview() {
	applyTheme(themes[e.themePickerIdx])
	e.editorBox.SetBackgroundColor(colBase)
	e.sidebar.SetBackgroundColor(colMantle)
	e.statusBox.SetBackgroundColor(colSurface1)
	e.inputField.SetFieldBackgroundColor(colSurface0)
}

func (e *Editor) themePickerConfirm() {
	e.themeIdx = e.themePickerIdx
	e.showThemePicker = false
	e.saveSettings()
	e.msg("theme: " + themes[e.themeIdx].Name)
}

func (e *Editor) themePickerCancel() {
	applyTheme(themes[e.themePickerPrevIdx])
	e.editorBox.SetBackgroundColor(colBase)
	e.sidebar.SetBackgroundColor(colMantle)
	e.statusBox.SetBackgroundColor(colSurface1)
	e.inputField.SetFieldBackgroundColor(colSurface0)
	e.showThemePicker = false
	e.themeIdx = e.themePickerPrevIdx
}

func (e *Editor) debounceTextSearch() {
	if e.textSearchTimer != nil {
		e.textSearchTimer.Stop()
	}
	e.textSearchTimer = time.AfterFunc(150*time.Millisecond, func() {
		e.app.QueueUpdateDraw(func() {
			if e.showTextSearch {
				e.searchInFiles(e.textSearchQuery)
			}
		})
	})
	if e.textSearchQuery == "" {
		e.textSearchResults = nil
		e.textSearchIdx = 0
		e.textSearchOff = 0
	}
}

func (e *Editor) cmdHelp() {
	e.showHelp = !e.showHelp
	if e.showHelp {
		e.helpOffset = 0
	}
}

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

func (e *Editor) centerOnCursor() {
	_, _, _, height := e.editorBox.GetRect()
	ch := height - 1
	e.offset.Y = e.cursor.Y - ch/2
	if e.offset.Y < 0 {
		e.offset.Y = 0
	}
	_, _, width, _ := e.editorBox.GetRect()
	ew := width
	if e.showSidebar {
		ew--
	}
	lnW := e.maxLineNumW() + 3
	ew -= lnW
	if ew < 1 {
		ew = 1
	}
	line := []rune(e.buffer[e.cursor.Y])
	cursorDisp := bufToDisp(line, e.cursor.X)
	wantDisp := cursorDisp - ew/2
	if wantDisp < 0 {
		wantDisp = 0
	}
	e.offset.X = dispToBuf(line, wantDisp)
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
	e.pendingDelPath = p
	e.showInput("confirm", "delete "+filepath.Base(p)+"? (y/n) ")
}

func (e *Editor) onSidebarSelectItem(idx int) {
	if idx < 0 || idx >= len(e.sidebarFiles) {
		return
	}
	name := e.sidebarFiles[idx]
	p := filepath.Join(e.sidebarDir, name)
	info, err := os.Stat(p)
	if err == nil && info.IsDir() {
		abs, _ := filepath.Abs(e.sidebarDir)
		e.sidebarDirIdx[abs] = e.sidebarIdx
		e.sidebarDir = p
		abs, _ = filepath.Abs(p)
		if saved, ok := e.sidebarDirIdx[abs]; ok {
			e.sidebarIdx = saved
		} else {
			e.sidebarIdx = 0
		}
		e.refreshDir()
		if e.sidebarIdx == 0 && len(e.sidebarFiles) > 1 && strings.HasSuffix(e.sidebarFiles[1], "/") {
			e.sidebarIdx = 1
		}
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
	Theme        string `json:"theme"`
	HideDotfiles  bool  `json:"hide_dotfiles"`
	SidebarWidth  int   `json:"sidebar_width"`
}

func (e *Editor) saveSettings() {
	path, err := settingsPath()
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	s := settings{
		Theme:        themes[e.themeIdx].Name,
		HideDotfiles: e.hideDotfiles,
		SidebarWidth: e.sidebarWidth,
	}
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
	e.hideDotfiles = s.HideDotfiles
	if s.SidebarWidth >= 15 {
		e.sidebarWidth = s.SidebarWidth
	}
	for i, t := range themes {
		if t.Name == s.Theme {
			e.themeIdx = i
			return
		}
	}
}

func (e *Editor) resizeSidebar(delta int) {
	e.sidebarWidth += delta * 3
	if e.sidebarWidth < 15 {
		e.sidebarWidth = 15
	}
	if e.sidebarWidth > 60 {
		e.sidebarWidth = 60
	}
	e.buildLayout()
	if e.mode == "sidebar" {
		e.app.SetFocus(e.sidebar)
	}
	e.saveSettings()
}

func (e *Editor) Init() {
	e.app = tview.NewApplication()
	e.app.EnableMouse(true)
	e.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if e.showFuzzy {
			switch event.Key() {
			case tcell.KeyRune:
				e.fuzzyQuery += string(event.Rune())
				e.updateFuzzy(e.fuzzyQuery)
				return nil
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if len(e.fuzzyQuery) > 0 {
					e.fuzzyQuery = e.fuzzyQuery[:len(e.fuzzyQuery)-1]
				}
				e.updateFuzzy(e.fuzzyQuery)
				return nil
			case tcell.KeyEnter:
				e.fuzzyOpen()
				return nil
			case tcell.KeyEscape:
				e.fuzzyCancel()
				return nil
			case tcell.KeyUp:
				e.fuzzyUp()
				return nil
			case tcell.KeyDown:
				e.fuzzyDown()
				return nil
			case tcell.KeyPgUp:
				e.fuzzyPgUp()
				return nil
			case tcell.KeyPgDn:
				e.fuzzyPgDn()
				return nil
			case tcell.KeyHome:
				e.fuzzyHome()
				return nil
			case tcell.KeyEnd:
				e.fuzzyEnd()
				return nil
			}
			return nil
		}
		if e.showTextSearch {
			switch event.Key() {
			case tcell.KeyRune:
				e.textSearchQuery += string(event.Rune())
				e.debounceTextSearch()
				return nil
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if len(e.textSearchQuery) > 0 {
					e.textSearchQuery = e.textSearchQuery[:len(e.textSearchQuery)-1]
				}
				e.debounceTextSearch()
				return nil
			case tcell.KeyEnter:
				e.textSearchOpen()
				return nil
			case tcell.KeyEscape:
				e.textSearchCancel()
				return nil
			case tcell.KeyUp:
				e.textSearchUp()
				return nil
			case tcell.KeyDown:
				e.textSearchDown()
				return nil
			case tcell.KeyPgUp:
				e.textSearchPgUp()
				return nil
			case tcell.KeyPgDn:
				e.textSearchPgDn()
				return nil
			case tcell.KeyHome:
				e.textSearchHome()
				return nil
			case tcell.KeyEnd:
				e.textSearchEnd()
				return nil
			case tcell.KeyLeft:
				e.textSearchLeft()
				return nil
			case tcell.KeyRight:
				e.textSearchRight()
				return nil
			}
			return nil
		}
		if e.showThemePicker {
			switch event.Key() {
			case tcell.KeyUp:
				e.themePickerUp()
				return nil
			case tcell.KeyDown:
				e.themePickerDown()
				return nil
			case tcell.KeyEnter:
				e.themePickerConfirm()
				return nil
			case tcell.KeyEscape:
				e.themePickerCancel()
				return nil
			}
			return nil
		}
		if e.showDiff {
			switch event.Key() {
			case tcell.KeyEscape:
				e.showDiff = false
				e.msg("")
				return nil
			case tcell.KeyUp:
				e.diffUp()
				return nil
			case tcell.KeyDown:
				e.diffDown()
				return nil
			case tcell.KeyPgUp:
				e.diffPgUp()
				return nil
			case tcell.KeyPgDn:
				e.diffPgDn()
				return nil
			case tcell.KeyHome:
				e.diffHome()
				return nil
			case tcell.KeyEnd:
				e.diffEnd()
				return nil
			case tcell.KeyEnter:
				e.diffRevertHunk()
				return nil
			}
			return nil
		}
		if e.showHelp {
			switch event.Key() {
			case tcell.KeyEscape, tcell.KeyF1:
				e.showHelp = false
				return nil
			case tcell.KeyUp:
				if e.helpOffset > 0 {
					e.helpOffset--
				}
				return nil
			case tcell.KeyDown:
				e.helpOffset++
				return nil
			case tcell.KeyPgUp:
				e.helpOffset -= 10
				return nil
			case tcell.KeyPgDn:
				e.helpOffset += 10
				return nil
			case tcell.KeyHome:
				e.helpOffset = 0
				return nil
			case tcell.KeyEnd:
				e.helpOffset = len(e.helpLines)
				return nil
			}
			return nil
		}
		if event.Key() == tcell.KeyCtrlC {
			if e.mode == "editor" {
				e.copySel()
			}
			return nil
		}
		if event.Key() == tcell.KeyF1 {
			e.cmdHelp()
			return nil
		}
		if event.Key() == tcell.KeyCtrlK || (event.Key() == tcell.KeyCtrlF && event.Modifiers()&tcell.ModShift != 0) {
			if !e.showTextSearch {
				e.cmdTextSearch()
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
	e.initialDir, _ = os.Getwd()
	e.hideDotfiles = true
	e.running = true
	e.themeIdx = 0
	e.sidebarDirIdx = map[string]int{}
	e.loadSettings()
	e.activeTab = 0
	e.openFiles = []*FileTab{{buffer: []string{""}}}
	applyTheme(themes[e.themeIdx])

	e.makeWidgets()
	e.refreshDir()
	e.buildLayout()
	e.app.SetRoot(e.pages, true)
	e.refreshGit()
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

	e.helpLines = []string{
		"  KEYBOARD SHORTCUTS",
		"  ─────────────────────────",
		"",
		"  GLOBAL",
		"    F1                     Show this help",
		"    Ctrl+P                 Fuzzy file finder",
		"    Ctrl+K / Ctrl+Shift+F  Text search across files",
		"    Ctrl+S                 Save file",
		"    Ctrl+N                 New file",
		"    Ctrl+B                 Toggle sidebar",
		"    Ctrl+R                 Toggle dotfiles",
		"    Ctrl+Q                 Quit",
		"    Alt+T                  Theme picker",
		"    Alt+D                  Diff view (revert hunks)",
		"    Alt+Left / Right       Switch tabs",
		"    Shift+Alt+Left/Right   Resize sidebar",
		"",
		"  EDITOR MODE",
		"    Ctrl+F                 Search",
		"    Ctrl+Z / Ctrl+Y        Undo / Redo",
		"    Ctrl+O                 Open file",
		"    Ctrl+C / V / X         Copy / Paste / Cut",
		"    Ctrl+A                 Select all",
		"    Ctrl+D                 Duplicate line",
		"    Ctrl+/                 Toggle comment",
		"    Ctrl+W                 Close tab",
		"    Ctrl+T                 Reveal file in sidebar",
		"    Up / Down / Left/Right Move cursor",
		"    Shift+Arrow            Extend selection",
		"    Ctrl+Left / Right      Word jump",
		"    Home / End             Line start / end",
		"    PgUp / PgDn            Scroll page",
		"    Enter                  Newline",
		"    Backspace              Delete backward",
		"    Delete / Shift+Del     Delete forward / line",
		"    Tab / Shift+Tab        Indent / Unindent",
		"    Escape                 Clear selection",
		"",
		"  SIDEBAR MODE",
		"    Up / Down              Navigate files",
		"    Enter                  Open file / directory",
		"    Left / Right           Parent / enter dir",
		"    Ctrl+F                 Filter files",
		"    Delete                 Delete file",
		"    Escape                 Clear filter / focus editor",
		"    Tab                    Focus editor",
		"    Home / End / PgUp/Dn   Navigate list",
		"",
		"  SEARCH / FILTER MODE",
		"    Tab / Shift+Tab        Next / Previous match",
		"    Enter                  Confirm",
		"    Escape                 Cancel",
		"",
		"  ─────────────────────────",
		"  Up/Down scroll  ·  Escape close",
	}

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
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if !e.running {
				return
			}
			e.app.QueueUpdateDraw(func() {
				e.refreshGit()
			})
		}
	}()
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
	if e.openFiles[0].filename == "" {
		e.mode = "sidebar"
		e.app.SetFocus(e.sidebar)
	} else {
		e.app.SetFocus(e.editorBox)
	}
	if err := e.Run(); err != nil {
		log.Fatal(err)
	}
}
