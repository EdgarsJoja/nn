package main

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/go-git/go-git/v5"
)

type gitInfo struct {
	root   string
	branch string
	status map[string]*git.FileStatus
}

func computeLineStat(headContent string, buffer []string) []byte {
	headStr := strings.TrimSuffix(headContent, "\n")
	headLines := strings.Split(headStr, "\n")
	if len(headLines) == 1 && headLines[0] == "" {
		headLines = nil
	}

	buf := buffer
	if len(buf) > 0 && buf[len(buf)-1] == "" && len(headLines) > 0 && headLines[len(headLines)-1] != "" {
		buf = buf[:len(buf)-1]
	}

	stat := make([]byte, len(buf))
	for i := range stat {
		stat[i] = '+'
	}

	// Forward: match identical lines from the start
	i, j := 0, 0
	for i < len(headLines) && j < len(buf) && headLines[i] == buf[j] {
		stat[j] = ' '
		i++
		j++
	}

	// Backward: match identical lines from the end
	ti, tj := len(headLines)-1, len(buf)-1
	for ti >= i && tj >= j && headLines[ti] == buf[tj] {
		stat[tj] = ' '
		ti--
		tj--
	}

	// Middle section: position-based comparison.
	// Each buffer line is compared against the head line at the same offset
	// within the middle. Matched pair → ' '; different → '~';
	// buffer lines beyond head length → '+'.
	for k := j; k <= tj; k++ {
		headPos := i + (k - j)
		if headPos <= ti {
			if buf[k] == headLines[headPos] {
				stat[k] = ' '
			} else {
				stat[k] = '~'
			}
		}
	}

	return stat
}

func (e *Editor) refreshGit() {
	e.git = nil
	tab := e.activeFile()

	filePath := ""
	if tab != nil {
		filePath = tab.filepath
		if filePath == "" {
			filePath = tab.filename
		}
	}

	var dir string
	if filePath != "" {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return
		}
		dir = filepath.Dir(absPath)
	} else {
		absDir, err := filepath.Abs(e.sidebarDir)
		if err != nil {
			return
		}
		dir = absDir
	}

	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return
	}

	ref, err := repo.Head()
	if err != nil {
		return
	}

	g := &gitInfo{
		branch: ref.Name().Short(),
	}

	wt, err := repo.Worktree()
	if err != nil {
		return
	}
	g.root = wt.Filesystem.Root()

	status, err := wt.Status()
	if err != nil {
		return
	}
	g.status = status
	e.git = g

	if filePath == "" {
		return
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return
	}
	tree, err := commit.Tree()
	if err != nil {
		return
	}

	relPath, err := filepath.Rel(g.root, absPath)
	if err != nil {
		return
	}
	relPath = filepath.ToSlash(relPath)

	blob, err := tree.File(relPath)
	if err != nil {
		tab.gitLineStat = nil
		tab.headContent = ""
		return
	}
	content, err := blob.Contents()
	if err != nil {
		tab.gitLineStat = nil
		tab.headContent = ""
		return
	}

	tab.headContent = content
	tab.gitLineStat = computeLineStat(content, e.buffer)
}

func (e *Editor) updateGitLineStat() {
	tab := e.activeFile()
	if tab == nil || tab.headContent == "" {
		return
	}
	tab.gitLineStat = computeLineStat(tab.headContent, e.buffer)
}

func (e *Editor) sidebarGitColor(name string) (tcell.Color, bool) {
	if e.git == nil || e.git.status == nil {
		return 0, false
	}
	fullPath := filepath.Join(e.sidebarDir, name)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return 0, false
	}
	rel, err := filepath.Rel(e.git.root, absPath)
	if err != nil {
		return 0, false
	}
	rel = filepath.ToSlash(rel)

	s, ok := e.git.status[rel]
	if !ok {
		return 0, false
	}

	// Untracked files should always show red; ignore any gitLineStat override
	if s.Worktree == git.Untracked || s.Staging == git.Untracked {
		return colRed, true
	}

	// For tracked files, check gitLineStat for unsaved changes
	if e.openFiles[e.activeTab].gitLineStat != nil {
		tabPath := e.openFiles[e.activeTab].filepath
		if tabPath == "" {
			tabPath = e.openFiles[e.activeTab].filename
		}
		absTabPath, err := filepath.Abs(tabPath)
		if err == nil {
			tabRel, err := filepath.Rel(e.git.root, absTabPath)
			if err == nil && filepath.ToSlash(tabRel) == rel {
				for _, st := range e.openFiles[e.activeTab].gitLineStat {
					if st != ' ' {
						return colNumber, true
					}
				}
			}
		}
	}

	switch {
	case s.Worktree == git.Modified || s.Staging == git.Modified:
		return colNumber, true
	case s.Worktree == git.Added || s.Staging == git.Added:
		return colGreen, true
	case s.Worktree == git.Deleted || s.Staging == git.Deleted:
		return colRed, true
	}
	return 0, false
}
