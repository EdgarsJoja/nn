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

	m, n := len(headLines), len(buf)

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if headLines[i-1] == buf[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	stat := make([]byte, n)
	for i := range stat {
		stat[i] = '+'
	}

	type edit struct {
		kind byte // 'M' match, 'I' insert, 'D' delete
		bi   int  // buffer index (for M/I)
	}
	var ops []edit

	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && headLines[i-1] == buf[j-1] {
			ops = append(ops, edit{'M', j - 1})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = append(ops, edit{'I', j - 1})
			j--
		} else if i > 0 {
			ops = append(ops, edit{'D', -1})
			i--
		}
	}

	// Reverse and pair deletions with nearby insertions
	delCount := 0
	for k := len(ops) - 1; k >= 0; k-- {
		switch ops[k].kind {
		case 'M':
			stat[ops[k].bi] = ' '
			delCount = 0
		case 'I':
			if delCount > 0 {
				stat[ops[k].bi] = '~'
				delCount--
			} else {
				stat[ops[k].bi] = '+'
			}
		case 'D':
			delCount++
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
		stat := make([]byte, len(e.buffer))
		for i := range stat {
			stat[i] = '+'
		}
		tab.gitLineStat = stat
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

	// If the active file matches this sidebar entry and has unsaved changes,
	// always show modified regardless of what wt.Status() says
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

	s, ok := e.git.status[rel]
	if !ok {
		return 0, false
	}
	switch {
	case s.Worktree == git.Untracked || s.Staging == git.Untracked:
		return colRed, true
	case s.Worktree == git.Modified || s.Staging == git.Modified:
		return colNumber, true
	case s.Worktree == git.Added || s.Staging == git.Added:
		return colGreen, true
	case s.Worktree == git.Deleted || s.Staging == git.Deleted:
		return colRed, true
	}
	return 0, false
}
