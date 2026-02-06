package fsutil

import (
	"io/fs"
	"os"
	"sort"
	"strings"
)

// ListMarkdownFilesFS lists all .md files in a directory from an fs.FS.
// Returns sorted filenames (not paths), or nil if directory doesn't exist.
func ListMarkdownFilesFS(fsys fs.FS, dir string) []string {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}

// ListMarkdownFiles lists all .md files in a directory from the real filesystem.
// Returns sorted filenames (not paths), or nil if directory doesn't exist.
func ListMarkdownFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}
