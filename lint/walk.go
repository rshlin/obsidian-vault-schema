package lint

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// walkMarkdown returns every ".md" file under root, sorted, skipping any
// directory whose name starts with "." (vault control directories like
// .git, .obsidian — this tool has no vault-specific exclude list, so "starts
// with a dot" is the one convention-free rule that still keeps it out of
// version control and editor state).
func walkMarkdown(root string) ([]string, error) {
	return walkMarkdownExcluding(root, nil)
}

// walkMarkdownExcluding is walkMarkdown plus a set of directory names
// (matched exactly against DirEntry.Name(), at any depth) to skip in addition
// to the dot-prefix rule — how --exclude-dir keeps a vault's non-note content
// (plugin sources, tooling directories) out of the walk entirely.
func walkMarkdownExcluding(root string, excludeDirs map[string]bool) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			if path != root && excludeDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}
