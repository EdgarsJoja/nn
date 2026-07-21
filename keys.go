package main

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
)

func (e *Editor) handleEditorKey(event *tcell.EventKey) *tcell.EventKey {
	key := event.Key()
	mod := event.Modifiers()
	hasShift := mod&tcell.ModShift != 0
	hasCtrl := mod&tcell.ModCtrl != 0
	hasAlt := mod&tcell.ModAlt != 0

	if key == tcell.KeyRune && mod&tcell.ModAlt != 0 && (event.Rune() == 't' || event.Rune() == 'T') {
		e.cycleTheme()
		return nil
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
	case tcell.KeyCtrlW:
		e.closeTab()
		return nil
	case tcell.KeyCtrlR:
		e.hideDotfiles = !e.hideDotfiles
		e.refreshDir()
		e.saveSettings()
		return nil
	case tcell.KeyCtrlQ:
		e.app.Stop()
		e.running = false
		return nil
	case tcell.KeyCtrlF:
		e.cmdSearch()
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
		if hasAlt {
			e.switchTab(-1)
		} else if hasCtrl {
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
		if hasAlt {
			e.switchTab(1)
		} else if hasCtrl {
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
		e.cursor.Y -= height - 1
		e.cursorInBounds()
		e.selection = Selection{}
	case tcell.KeyPgDn:
		_, _, _, height := e.editorBox.GetRect()
		e.cursor.Y += height - 1
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

func (e *Editor) handleSidebarKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyRune && event.Modifiers()&tcell.ModAlt != 0 && (event.Rune() == 't' || event.Rune() == 'T') {
		e.cycleTheme()
		return nil
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
	case tcell.KeyLeft:
		if event.Modifiers()&tcell.ModAlt != 0 {
			e.switchTab(-1)
		} else {
			absDir, _ := filepath.Abs(e.sidebarDir)
			parent := filepath.Dir(absDir)
			if parent != absDir {
				e.sidebarDirIdx[absDir] = e.sidebarIdx
				e.sidebarDir = parent
				if saved, ok := e.sidebarDirIdx[parent]; ok {
					e.sidebarIdx = saved
				} else {
					e.sidebarIdx = 0
				}
				e.sidebarOff = 0
				e.refreshDir()
			}
		}
		return nil
	case tcell.KeyRight:
		if event.Modifiers()&tcell.ModAlt != 0 {
			e.switchTab(1)
		} else {
			e.sidebarEnterDir()
		}
		return nil
	case tcell.KeyHome:
		e.sidebarIdx = 0
	case tcell.KeyEnd:
		e.sidebarIdx = len(e.sidebarFiles) - 1
	case tcell.KeyPgUp:
		rows := e.getSidebarHeight()
		e.sidebarIdx -= rows
		if e.sidebarIdx < 0 {
			e.sidebarIdx = 0
		}
	case tcell.KeyPgDn:
		rows := e.getSidebarHeight()
		e.sidebarIdx += rows
		if e.sidebarIdx >= len(e.sidebarFiles) {
			e.sidebarIdx = len(e.sidebarFiles) - 1
		}
	case tcell.KeyEnter:
		e.sidebarEnterDir()
		return nil
	case tcell.KeyCtrlR:
		e.hideDotfiles = !e.hideDotfiles
		e.refreshDir()
		e.saveSettings()
		return nil
	case tcell.KeyDelete:
		e.deleteSidebarFile()
		return nil
	case tcell.KeyEscape:
		if e.sidebarFilter != "" {
			e.sidebarFilter = ""
			e.sidebarFiles = e.sidebarAllFiles
			if e.sidebarIdx >= len(e.sidebarFiles) {
				e.sidebarIdx = 0
			}
			return nil
		}
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
	case tcell.KeyCtrlF:
		e.showInput("filesearch", "filter: ")
		return nil
	case tcell.KeyCtrlS:
		if e.filename == "" {
			e.showInput("save", "save as: ")
		} else {
			e.saveFile(e.filename)
		}
		return nil
	}

	rows := e.getSidebarHeight()
	if e.sidebarIdx < e.sidebarOff {
		e.sidebarOff = e.sidebarIdx
	}
	if e.sidebarIdx >= e.sidebarOff+rows {
		e.sidebarOff = e.sidebarIdx - rows + 1
	}

	return event
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

func (e *Editor) deleteBackward() {
	if e.selection.Active {
		e.deleteSelection()
		return
	}
	if e.cursor.X > 0 {
		line := e.buffer[e.cursor.Y]
		e.buffer[e.cursor.Y] = line[:e.cursor.X-1] + line[e.cursor.X:]
		e.cursor.X--
		e.setModified()
	} else if e.cursor.Y > 0 {
		prev := len([]rune(e.buffer[e.cursor.Y-1]))
		e.buffer[e.cursor.Y-1] += e.buffer[e.cursor.Y]
		e.buffer = append(e.buffer[:e.cursor.Y], e.buffer[e.cursor.Y+1:]...)
		e.cursor.Y--
		e.cursor.X = prev
		e.setModified()
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
		e.setModified()
	} else if e.cursor.Y < len(e.buffer)-1 {
		e.buffer[e.cursor.Y] += e.buffer[e.cursor.Y+1]
		e.buffer = append(e.buffer[:e.cursor.Y+1], e.buffer[e.cursor.Y+2:]...)
		e.setModified()
	}
}

func (e *Editor) deleteLine() {
	if len(e.buffer) == 1 {
		e.buffer[0] = ""
		e.cursor.X = 0
		e.setModified()
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
	e.setModified()
	e.selection = Selection{}
}

func (e *Editor) duplicateLine() {
	line := e.buffer[e.cursor.Y]
	dup := line
	e.buffer = append(e.buffer, "")
	copy(e.buffer[e.cursor.Y+2:], e.buffer[e.cursor.Y+1:])
	e.buffer[e.cursor.Y+1] = dup
	e.cursor.Y++
	e.cursor.X = 0
	e.setModified()
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
