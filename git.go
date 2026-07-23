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
	dir    string
	repo   *git.Repository
	wt     *git.Worktree
}

func computeLineStat(headContent string, buffer []string) []byte {
	headLines := strings.Split(headContent, "\n")
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
	hp, bp := 0, 0
	for hp < len(headLines) && bp < len(buf) && headLines[hp] == buf[bp] {
		stat[bp] = ' '
		hp++
		bp++
	}

	// Backward scan with a safety limit: only match up to the
	// number of remaining lines in the shorter tail. This prevents
	// matching shifted duplicate lines (e.g. blank lines).
	ti, tj := len(headLines)-1, len(buf)-1
	maxBack := ti - hp + 1
	if tj-bp+1 < maxBack {
		maxBack = tj - bp + 1
	}

	backCount := 0
	for ti >= hp && tj >= bp && headLines[ti] == buf[tj] && backCount < maxBack {
		ti--
		tj--
		backCount++
	}

	// At this point ti and tj have been decremented from the last
	// non-matching position. Re-increment to get the true match boundaries.
	ti++
	tj++

	// Middle section: process from bp to tj-1 (head from hp to ti-1)
	// Use lookahead matching to handle insertions/deletions
	for bp < tj && hp < ti {
		if buf[bp] == headLines[hp] {
			stat[bp] = ' '
			hp++
			bp++
			continue
		}

		// Look ahead in head for buf[bp]
		foundHead := -1
		for k := hp + 1; k < ti && k <= hp+10; k++ {
			if headLines[k] == buf[bp] {
				foundHead = k
				break
			}
		}

		// Look ahead in buf for headLines[hp]
		foundBuf := -1
		for k := bp + 1; k < tj && k <= bp+10; k++ {
			if buf[k] == headLines[hp] {
				foundBuf = k
				break
			}
		}

		if foundHead != -1 && foundBuf != -1 {
			// Both match ahead — mark as changed
			stat[bp] = '~'
			hp++
			bp++
		} else if foundHead != -1 {
			// buf[bp] matches a later head line → head lines were deleted
			stat[bp] = '~'
			hp++
		} else if foundBuf != -1 {
			// headLines[hp] matches a later buf line → buf lines were added
			stat[bp] = '+'
			bp++
		} else {
			// Neither matches ahead — lines are different
			stat[bp] = '~'
			hp++
			bp++
		}
	}

	// Remaining buffer lines are added
	for bp < tj {
		stat[bp] = '+'
		bp++
	}

	// Tail from tj onward is all matched
	for bp < len(buf) {
		stat[bp] = ' '
		bp++
	}

	return stat
}

func (e *Editor) refreshGit() {
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
		e.git = nil
		return
	}

	ref, err := repo.Head()
	if err != nil {
		e.git = nil
		return
	}

	wt, err := repo.Worktree()
	if err != nil {
		e.git = nil
		return
	}

	status, err := wt.Status()
	if err != nil {
		e.git = nil
		return
	}

	e.git = &gitInfo{
		root:   wt.Filesystem.Root(),
		branch: ref.Name().Short(),
		status: status,
		dir:    dir,
		repo:   repo,
		wt:     wt,
	}

	e.updateGitFileInfo()
}

func (e *Editor) refreshGitTab() {
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

	if e.git != nil && e.git.repo != nil && e.git.dir == dir {
		e.updateGitFileInfo()
		return
	}

	e.refreshGit()
}

func (e *Editor) updateGitFileInfo() {
	tab := e.activeFile()
	if tab == nil || e.git == nil || e.git.repo == nil {
		return
	}

	filePath := tab.filepath
	if filePath == "" {
		filePath = tab.filename
	}
	if filePath == "" {
		return
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return
	}

	relPath, err := filepath.Rel(e.git.root, absPath)
	if err != nil {
		return
	}
	relPath = filepath.ToSlash(relPath)

	ref, err := e.git.repo.Head()
	if err != nil {
		return
	}
	e.git.branch = ref.Name().Short()

	commit, err := e.git.repo.CommitObject(ref.Hash())
	if err != nil {
		return
	}
	tree, err := commit.Tree()
	if err != nil {
		return
	}

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
