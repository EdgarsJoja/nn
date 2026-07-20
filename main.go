package main

import (
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

	mode      string
	inputMode string

	message  string
	msgTimer int

	running   bool
	themeIdx  int
	openFiles []*FileTab
	activeTab int
}

func (e *Editor) msg(text string) {
	e.message = text
	e.msgTimer = 80
}

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
	e.inputMode = ""
	e.pages.SwitchToPage("main")
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

func (e *Editor) cmdOpen()  { e.showInput("open", "open: ") }
func (e *Editor) cmdNew()   { e.showInput("new", "new file: ") }

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
	e.pages.RemovePage("main")
	e.buildLayout()
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
