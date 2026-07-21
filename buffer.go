package main

import (
	"os"
	"path/filepath"
	"strings"
)

type Point struct{ X, Y int }

type Selection struct {
	Active bool
	Start  Point
	End    Point
}

type undoState struct {
	buffer    []string
	cursor    Point
	selection Selection
	modified  bool
}

type FileTab struct {
	filename     string
	filepath     string
	buffer       []string
	cursor       Point
	offset       Point
	selection    Selection
	modified     bool
	syntaxTokens []syntaxToken
	gitLineStat  []byte
	headContent  string
}

func (e *Editor) activeFile() *FileTab {
	return e.openFiles[e.activeTab]
}

const maxUndo = 100

const (
	opNone     = 0
	opInsert   = 1
	opDeleteBk = 2
	opDeleteFd = 3
)

func (e *Editor) saveUndoState(op int) {
	if op == e.lastOp && len(e.undoStack) > 0 {
		return
	}
	state := undoState{
		buffer:    append([]string{}, e.buffer...),
		cursor:    e.cursor,
		selection: e.selection,
		modified:  e.modified,
	}
	e.undoStack = append(e.undoStack, state)
	if len(e.undoStack) > maxUndo {
		e.undoStack = e.undoStack[1:]
	}
	e.redoStack = nil
	e.lastOp = op
}

func (e *Editor) undo() {
	if len(e.undoStack) == 0 {
		return
	}
	redo := undoState{
		buffer:    append([]string{}, e.buffer...),
		cursor:    e.cursor,
		selection: e.selection,
		modified:  e.modified,
	}
	e.redoStack = append(e.redoStack, redo)

	state := e.undoStack[len(e.undoStack)-1]
	e.undoStack = e.undoStack[:len(e.undoStack)-1]

	e.buffer = state.buffer
	e.cursor = state.cursor
	e.selection = state.selection
	e.modified = state.modified
	e.openFiles[e.activeTab].buffer = e.buffer
	e.openFiles[e.activeTab].cursor = e.cursor
	e.openFiles[e.activeTab].selection = e.selection
	e.openFiles[e.activeTab].modified = e.modified
	e.openFiles[e.activeTab].syntaxTokens = nil
	e.lastOp = opNone
	e.cursorInBounds()
	e.updateGitLineStat()
}

func (e *Editor) redo() {
	if len(e.redoStack) == 0 {
		return
	}
	undo := undoState{
		buffer:    append([]string{}, e.buffer...),
		cursor:    e.cursor,
		selection: e.selection,
		modified:  e.modified,
	}
	e.undoStack = append(e.undoStack, undo)

	state := e.redoStack[len(e.redoStack)-1]
	e.redoStack = e.redoStack[:len(e.redoStack)-1]

	e.buffer = state.buffer
	e.cursor = state.cursor
	e.selection = state.selection
	e.modified = state.modified
	e.openFiles[e.activeTab].buffer = e.buffer
	e.openFiles[e.activeTab].cursor = e.cursor
	e.openFiles[e.activeTab].selection = e.selection
	e.openFiles[e.activeTab].modified = e.modified
	e.openFiles[e.activeTab].syntaxTokens = nil
	e.lastOp = opNone
	e.cursorInBounds()
	e.updateGitLineStat()
}

func (e *Editor) setModified() {
	e.modified = true
	if e.activeTab < len(e.openFiles) {
		e.openFiles[e.activeTab].modified = true
	}
	e.gitDirty = true
	e.tokenizeDebounce = 5
}

func (e *Editor) saveCurrentTab() {
	if e.activeTab < len(e.openFiles) {
		e.openFiles[e.activeTab].buffer = e.buffer
		e.openFiles[e.activeTab].cursor = e.cursor
		e.openFiles[e.activeTab].offset = e.offset
		e.openFiles[e.activeTab].selection = e.selection
		e.openFiles[e.activeTab].filename = e.filename
		e.openFiles[e.activeTab].modified = e.modified
		e.openFiles[e.activeTab].syntaxTokens = nil
	}
}

func (e *Editor) restoreTab(idx int) {
	if idx < 0 || idx >= len(e.openFiles) {
		return
	}
	t := e.openFiles[idx]
	e.buffer = t.buffer
	e.cursor = t.cursor
	e.offset = t.offset
	e.selection = t.selection
	e.filename = t.filename
	e.modified = t.modified
	e.activeTab = idx
}

func (e *Editor) switchTab(dir int) {
	if len(e.openFiles) < 2 {
		return
	}
	e.saveCurrentTab()
	next := (e.activeTab + dir) % len(e.openFiles)
	if next < 0 {
		next = len(e.openFiles) - 1
	}
	e.restoreTab(next)
	e.msg("tab: " + filepath.Base(e.openFiles[next].filename))
	e.refreshGitTab()
}

func (e *Editor) closeTab() {
	if len(e.openFiles) <= 1 {
		e.buffer = []string{""}
		e.cursor = Point{}
		e.offset = Point{}
		e.filename = ""
		e.modified = false
		e.selection = Selection{}
		e.openFiles[0].buffer = []string{""}
		e.openFiles[0].filename = ""
		e.openFiles[0].modified = false
		e.msg("closed tab")
		return
	}
	e.saveCurrentTab()
	idx := e.activeTab
	e.openFiles = append(e.openFiles[:idx], e.openFiles[idx+1:]...)
	if e.activeTab >= len(e.openFiles) {
		e.activeTab = len(e.openFiles) - 1
	}
	e.restoreTab(e.activeTab)
	e.msg("closed tab")
}

func (e *Editor) loadFile(path string) {
	for i, tab := range e.openFiles {
		if tab.filename == path {
			e.saveCurrentTab()
			e.restoreTab(i)
			e.msg("switched to " + filepath.Base(path))
			e.refreshGit()
			return
		}
	}

	if len(e.openFiles) == 1 && e.openFiles[0].filename == "" && len(e.openFiles[0].buffer) == 1 && e.openFiles[0].buffer[0] == "" {
		tab := e.openFiles[0]
		data, err := os.ReadFile(path)
		if err != nil {
			e.msg("error: " + err.Error())
			return
		}
		tab.buffer = strings.Split(string(data), "\n")
		if len(tab.buffer) == 0 {
			tab.buffer = []string{""}
		}
		tab.filename = path
		tab.syntaxTokens = nil
		e.undoStack = nil
		e.redoStack = nil
		e.restoreTab(0)
		e.msg("opened " + filepath.Base(path))
		e.refreshGit()
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		e.msg("error: " + err.Error())
		return
	}
	e.saveCurrentTab()
	content := string(data)
	buf := strings.Split(content, "\n")
	if len(buf) == 0 {
		buf = []string{""}
	}
	e.openFiles = append(e.openFiles, &FileTab{
		filename:     path,
		buffer:       buf,
		syntaxTokens: nil,
	})
	e.undoStack = nil
	e.redoStack = nil
	e.restoreTab(len(e.openFiles) - 1)
	e.msg("opened " + filepath.Base(path))
	e.refreshGit()
}

func (e *Editor) saveFile(path string) {
	data := []byte(strings.Join(e.buffer, "\n"))
	if err := os.WriteFile(path, data, 0644); err != nil {
		e.msg("error: " + err.Error())
		return
	}
	e.filename = path
	e.modified = false
	e.saveCurrentTab()
	e.msg("saved " + filepath.Base(path))
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
	e.setModified()
}

func (e *Editor) insertText(text string) {
	e.saveUndoState(opInsert)
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
	e.setModified()
}

func (e *Editor) copySel() {
	if t := e.selectedText(); t != "" {
		e.clipboard = t
		e.msg("copied")
	}
}

func (e *Editor) cutSel() {
	e.saveUndoState(opNone)
	if t := e.selectedText(); t != "" {
		e.clipboard = t
		e.deleteSelection()
		e.msg("cut")
	}
}

func (e *Editor) pasteClip() {
	e.saveUndoState(opNone)
	e.deleteSelection()
	if e.clipboard == "" {
		return
	}
	lines := strings.Split(e.clipboard, "\n")
	if len(lines) == 1 {
		line := e.buffer[e.cursor.Y]
		e.buffer[e.cursor.Y] = line[:e.cursor.X] + lines[0] + line[e.cursor.X:]
		e.cursor.X += len([]rune(lines[0]))
	} else {
		line := e.buffer[e.cursor.Y]
		rest := line[e.cursor.X:]
		e.buffer[e.cursor.Y] = line[:e.cursor.X]
		e.buffer[e.cursor.Y] += lines[0]
		tail := make([]string, len(e.buffer[e.cursor.Y+1:]))
		copy(tail, e.buffer[e.cursor.Y+1:])
		e.buffer = append(e.buffer[:e.cursor.Y+1], "", "")
		e.buffer = append(e.buffer, tail...)
		for i := 1; i < len(lines)-1; i++ {
			e.buffer = append(e.buffer[:e.cursor.Y+1+i], "", "")
			e.buffer = append(e.buffer, e.buffer[e.cursor.Y+1+i+1:]...)
			e.buffer[e.cursor.Y+1+i] = lines[i]
		}
		e.buffer[e.cursor.Y+len(lines)-1] = lines[len(lines)-1] + rest
		e.cursor.Y += len(lines) - 1
		e.cursor.X = len([]rune(lines[len(lines)-1]))
	}
	e.setModified()
}

func (e *Editor) openFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	tab := &FileTab{
		filename:     filepath.Base(path),
		filepath:     path,
		buffer:       lines,
		cursor:       Point{},
		offset:       Point{},
		syntaxTokens: nil,
	}
	e.openFiles = append(e.openFiles, tab)
	e.activeTab = len(e.openFiles) - 1
	e.undoStack = nil
	e.redoStack = nil
	e.restoreTab(e.activeTab)
	e.refreshGit()
	return nil
}
