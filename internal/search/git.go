package search

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

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
