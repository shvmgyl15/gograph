package search

import (
	"fmt"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// Review generates a post-edit review report for modified symbols.
func Review(g *graph.Graph, symbolNames []string, title string) string {
	if len(symbolNames) == 0 {
		return "No modified symbols found to review."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Code Review for %s\n\n", title))

	// Find the subset of symbols actually present in the graph
	var validSymbols []string
	symbolMap := make(map[string]*graph.SymbolNode)
	for i := range g.Symbols {
		s := &g.Symbols[i]
		symbolMap[s.ID] = s
		// also map by name so we can do review "ValidateToken" directly
		if _, exists := symbolMap[s.Name]; !exists {
			symbolMap[s.Name] = s
		}
	}

	for _, name := range symbolNames {
		if _, ok := symbolMap[name]; ok {
			validSymbols = append(validSymbols, name)
		}
	}

	sb.WriteString("1. What changed?\n")
	for _, name := range validSymbols {
		sym := symbolMap[name]
		sb.WriteString(fmt.Sprintf("   - %s:%d %s (%s)\n", sym.File, sym.Line, sym.Name, sym.Kind))
	}
	if len(symbolNames) > len(validSymbols) {
		sb.WriteString(fmt.Sprintf("   - ... plus %d symbols deleted or unparseable.\n", len(symbolNames)-len(validSymbols)))
	}
	sb.WriteString("\n")

	sb.WriteString("2. Which changed symbols lack mapped tests?\n")
	uncoveredTests := 0
	for _, name := range validSymbols {
		if len(Tests(g, name)) == 0 {
			sb.WriteString(fmt.Sprintf("   - %s\n", name))
			uncoveredTests++
		}
	}
	if uncoveredTests == 0 {
		sb.WriteString("   - None. All changed symbols have mapped tests.\n")
	}
	sb.WriteString("\n")

	sb.WriteString("3. Complexity & Architectural Risk (Current State)\n")
	riskFound := false
	for _, name := range validSymbols {
		sym := symbolMap[name]
		if sym.Kind == "function" || sym.Kind == "method" {
			results := Complexity(g, name)
			if len(results) > 0 {
				score := results[0].Score
				if score > 10 {
					sb.WriteString(fmt.Sprintf("   - [HIGH COMPLEXITY] %s: score=%d\n", name, score))
					riskFound = true
				}
			}
		}
	}
	if !riskFound {
		sb.WriteString("   - No high complexity functions modified.\n")
	}
	sb.WriteString("\n")

	sb.WriteString("4. Did public API or route surface change?\n")
	apiChanged := false
	for _, name := range validSymbols {
		parts := strings.Split(name, "::")
		short := parts[len(parts)-1]
		parts2 := strings.Split(short, ".")
		short2 := parts2[len(parts2)-1]
		if len(short2) > 0 && short2[0] >= 'A' && short2[0] <= 'Z' {
			sb.WriteString(fmt.Sprintf("   - [PUBLIC API] %s\n", name))
			apiChanged = true
		}
		for _, r := range g.Routes {
			if r.Handler == name {
				sb.WriteString(fmt.Sprintf("   - [HTTP ROUTE] %s %s -> %s\n", r.Method, r.Path, name))
				apiChanged = true
			}
		}
	}
	if !apiChanged {
		sb.WriteString("   - No public API or HTTP routes modified.\n")
	}
	sb.WriteString("\n")

	sb.WriteString("5. Downstream Execution Risks (What do these changes touch?)\n")
	// BFS for ALL modified symbols
	downstream := make(map[string]bool)
	queue := make([]string, len(validSymbols))
	copy(queue, validSymbols)
	for _, s := range validSymbols {
		downstream[s] = true
	}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, call := range g.Calls {
			if call.CallerName == curr {
				if !downstream[call.CalleeRaw] {
					downstream[call.CalleeRaw] = true
					queue = append(queue, call.CalleeRaw)
				}
			}
		}
	}

	var envLines []string
	seenEnvs := make(map[string]bool)
	for _, env := range g.EnvReads {
		if downstream[env.Function] {
			if !seenEnvs[env.Key] {
				seenEnvs[env.Key] = true
				envLines = append(envLines, env.Key)
			}
		}
	}

	if len(envLines) > 0 {
		sb.WriteString(fmt.Sprintf("   - Reads Environment Variables: %s\n", strings.Join(envLines, ", ")))
	} else {
		sb.WriteString("   - Reads Environment Variables: no\n")
	}

	touchesSQL := false
	for _, sql := range g.SQLs {
		if downstream[sql.Function] {
			touchesSQL = true
			break
		}
	}
	sb.WriteString(fmt.Sprintf("   - Touches SQL: %v\n", touchesSQL))

	touchesErrors := false
	for _, err := range g.Errors {
		if downstream[err.Function] {
			touchesErrors = true
			break
		}
	}
	sb.WriteString(fmt.Sprintf("   - Emits Custom Errors/Panics: %v\n", touchesErrors))

	touchesConcurrency := false
	for _, c := range g.Concurrency {
		if downstream[c.Function] {
			touchesConcurrency = true
			break
		}
	}
	sb.WriteString(fmt.Sprintf("   - Uses Concurrency Primitives: %v\n", touchesConcurrency))

	return sb.String()
}
