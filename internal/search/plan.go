package search

import (
	"fmt"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// Plan generates an operational change plan for one or more symbols.
func Plan(g *graph.Graph, symbolNames []string, title string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Change plan for %s\n\n", title))

	// 1. Read first
	sb.WriteString("1. Read first:\n")
	readSet := make(map[string]bool)
	var readLines []string

	for _, symName := range symbolNames {
		// Find symbol
		for _, s := range g.Symbols {
			if s.ID == symName || s.Name == symName {
				line := fmt.Sprintf("   - %s:%d %s", s.File, s.Line, s.Name)
				if !readSet[line] {
					readSet[line] = true
					readLines = append(readLines, line)
				}
			}
		}
		// Find immediate callers
		callers := Callers(g, symName, true)
		for _, c := range callers {
			line := fmt.Sprintf("   - %s:%d %s", c.File, c.Line, c.Name)
			if !readSet[line] {
				readSet[line] = true
				readLines = append(readLines, line)
			}
		}
	}
	if len(readLines) > 0 {
		// Limit to 15 to avoid massive spam
		limit := len(readLines)
		if limit > 15 {
			limit = 15
		}
		for i := 0; i < limit; i++ {
			sb.WriteString(readLines[i] + "\n")
		}
		if len(readLines) > 15 {
			sb.WriteString(fmt.Sprintf("   ... and %d more\n", len(readLines)-15))
		}
	} else {
		sb.WriteString("   - (No source or callers found)\n")
	}
	sb.WriteString("\n")

	// 2. Update likely affected tests
	sb.WriteString("2. Update likely affected tests:\n")
	testFiles := make(map[string]bool)
	
	for _, symName := range symbolNames {
		ts := Tests(g, symName)
		for _, t := range ts {
			testFiles[t.File] = true
		}
	}
	if len(testFiles) > 0 {
		for tf := range testFiles {
			sb.WriteString(fmt.Sprintf("   - %s\n", tf))
		}
	} else {
		sb.WriteString("   - (No direct tests found)\n")
	}
	sb.WriteString("\n")

	// 3. Risk Profile
	sb.WriteString("3. Risk:\n")
	
	// Check Public API
	isPublic := "no"
	for _, symName := range symbolNames {
		if len(symName) > 0 {
			// Find actual symbol name (ignoring receiver and ID parts)
			parts := strings.Split(symName, "::")
			short := parts[len(parts)-1]
			parts2 := strings.Split(short, ".")
			name := parts2[len(parts2)-1]
			if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
				isPublic = "yes"
				break
			}
		}
	}
	sb.WriteString(fmt.Sprintf("   - Public API: %s\n", isPublic))

	// Check Routes
	var routeLines []string
	routeSet := make(map[string]bool)
	blastRadius := ImpactMultiple(g, symbolNames, "plan", true)
	blastMap := make(map[string]bool)
	for _, b := range blastRadius {
		blastMap[b.Name] = true
	}
	for _, s := range symbolNames {
		blastMap[s] = true
	}

	for _, route := range g.Routes {
		if blastMap[route.Handler] {
			rt := fmt.Sprintf("%s %s", route.Method, route.Path)
			if !routeSet[rt] {
				routeSet[rt] = true
				routeLines = append(routeLines, rt)
			}
		}
	}
	if len(routeLines) > 0 {
		if len(routeLines) > 3 {
			sb.WriteString(fmt.Sprintf("   - Called by HTTP route: %s, and %d more\n", strings.Join(routeLines[:3], ", "), len(routeLines)-3))
		} else {
			sb.WriteString(fmt.Sprintf("   - Called by HTTP route: %s\n", strings.Join(routeLines, ", ")))
		}
	} else {
		sb.WriteString("   - Called by HTTP route: no\n")
	}

	// Check Envs and SQL via downstream BFS
	downstream := make(map[string]bool)
	queue := make([]string, len(symbolNames))
	copy(queue, symbolNames)
	for _, s := range symbolNames {
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
	envSet := make(map[string]bool)
	for _, env := range g.EnvReads {
		if downstream[env.Function] {
			if !envSet[env.Key] {
				envSet[env.Key] = true
				envLines = append(envLines, env.Key)
			}
		}
	}
	if len(envLines) > 0 {
		if len(envLines) > 3 {
			sb.WriteString(fmt.Sprintf("   - Reads env: %s, and %d more\n", strings.Join(envLines[:3], ", "), len(envLines)-3))
		} else {
			sb.WriteString(fmt.Sprintf("   - Reads env: %s\n", strings.Join(envLines, ", ")))
		}
	} else {
		sb.WriteString("   - Reads env: no\n")
	}

	touchesSQL := "no"
	for _, sql := range g.SQLs {
		if downstream[sql.Function] {
			touchesSQL = "yes"
			break
		}
	}
	sb.WriteString(fmt.Sprintf("   - Touches SQL: %s\n", touchesSQL))

	return sb.String()
}
