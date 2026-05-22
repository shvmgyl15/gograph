package search

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// safeGitRef is a positive allowlist for git refs to prevent shell injection.
// Allows alphanumeric characters, dots, slashes, hyphens, tildes, and carets.
var safeGitRef = regexp.MustCompile(`^[A-Za-z0-9._/\-~^]+$`)

// ChangesByGitRef returns all graph symbols whose source files are reported as
// changed by "git diff --name-only <ref>". Only ChangeModified status is
// returned — detecting NEW or DELETED symbols requires a full baseline graph
// build from that ref, which is out of scope for this command.
//
// root is the absolute path to the repository root.
func ChangesByGitRef(g *graph.Graph, root, ref string) (*ChangesResult, error) {
	if ref == "" || strings.HasPrefix(ref, "-") || !safeGitRef.MatchString(ref) {
		return nil, fmt.Errorf("invalid git ref %q: must match [A-Za-z0-9._/\\-~^]+", ref)
	}

	out, err := exec.Command("git", "-C", root, "diff", "--name-only", ref).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s: %w", ref, err)
	}

	// Build a set of changed files reported by git (repo-relative paths).
	changedFiles := make(map[string]bool)
	var changedList []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(line, ".go") {
			continue
		}
		if !changedFiles[line] {
			changedFiles[line] = true
			changedList = append(changedList, line)
		}
	}
	sortStrings(changedList)

	result := &ChangesResult{
		GraphAge:     g.GeneratedAt,
		ChangedFiles: changedList,
	}

	// Map changed files to graph symbols.
	seen := make(map[string]bool)
	for _, s := range g.Symbols {
		// Normalize the symbol's file to a repo-relative path for matching.
		rel := s.File
		if abs, err := filepath.Rel(root, s.File); err == nil {
			rel = abs
		}
		rel = filepath.ToSlash(rel)
		if !changedFiles[rel] {
			continue
		}
		if seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		result.Symbols = append(result.Symbols, ChangedSymbol{
			Name:   s.Name,
			File:   rel,
			Line:   s.Line,
			Status: ChangeModified,
		})
	}

	return result, nil
}

// UncommittedSymbols parses git diff to find modified symbols in the current working directory.
func UncommittedSymbols(g *graph.Graph) ([]string, error) {
	out, err := exec.Command("git", "diff", "HEAD", "-U0").Output()
	if err != nil {
		return nil, fmt.Errorf("error running git diff: %v", err)
	}

	diffStr := string(out)
	fileLines := make(map[string][]int)
	var currentFile string

	for _, line := range strings.Split(diffStr, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
		} else if strings.HasPrefix(line, "@@ ") && currentFile != "" {
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				plusPart := strings.TrimPrefix(parts[2], "+")
				sp := strings.Split(plusPart, ",")
				start, _ := strconv.Atoi(sp[0])
				count := 1
				if len(sp) > 1 {
					count, _ = strconv.Atoi(sp[1])
				}
				for i := 0; i < count; i++ {
					fileLines[currentFile] = append(fileLines[currentFile], start+i)
				}
			}
		}
	}

	var modifiedSymbolNames []string
	seenSymbols := make(map[string]bool)

	for file, lines := range fileLines {
		for _, s := range g.Symbols {
			if strings.HasSuffix(s.File, file) {
				for _, line := range lines {
					if line >= s.Line && line <= s.EndLine {
						if !seenSymbols[s.ID] {
							seenSymbols[s.ID] = true
							modifiedSymbolNames = append(modifiedSymbolNames, s.ID)
						}
						break
					}
				}
			}
		}
	}
	return modifiedSymbolNames, nil
}
