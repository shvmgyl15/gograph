package search

import (
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// EndpointSlice is the full vertical slice for one HTTP endpoint.
type EndpointSlice struct {
	Route       string       `json:"route"`
	Handler     string       `json:"handler"`
	HandlerFile string       `json:"handler_file"`
	HandlerLine int          `json:"handler_line"`
	CallChain   []ChainStep  `json:"call_chain"`
	SQL         []SQLStep    `json:"sql,omitempty"`
	EnvReads    []string     `json:"env_reads,omitempty"`
	Limitations []string     `json:"limitations"`
}

// ChainStep is one symbol in the downstream call chain.
type ChainStep struct {
	Depth   int      `json:"depth"`
	Symbol  string   `json:"symbol"`
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Callees []string `json:"callees,omitempty"`
}

// SQLStep is a SQL query emitted somewhere in the call chain.
type SQLStep struct {
	Query    string `json:"query"`
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

const endpointMaxDepth = 5

// Endpoint traces the full vertical slice for an HTTP route.
//
// query can be:
//   - A full route pattern:   "POST /api/users"
//   - A path fragment:        "/users"  (matches all methods containing that path)
//   - A handler symbol name:  "CreateUser"
//
// # Route Resolution Limitation — Read Before Using
//
// gograph resolves routes by reading the literal string passed as the first
// argument to HTTP registration calls (.GET, .POST, .PUT, .DELETE, etc.).
// This works reliably for flat router registrations like:
//
//	router.POST("/api/users", CreateUser)   → recorded as POST /api/users
//
// However, it CANNOT resolve grouped/chained routes where the full path is
// assembled at runtime from a variable prefix. For example, with Gin groups:
//
//	v1 := router.Group("/api/v1")
//	users := v1.Group("/users")
//	users.POST("/", CreateUser)   → recorded as POST /  (prefix is lost)
//
// In this case, searching for "POST /api/v1/users" returns no results because
// that string never appears as a literal in the AST. The assembled path exists
// only at runtime.
//
// # Recommended Usage
//
// When route-grouping is used (Gin, Echo, Chi, fiber groups), always prefer
// searching by handler symbol name rather than route pattern:
//
//	gograph endpoint "CreateUser"   // always works, regardless of routing style
//	gograph endpoint "/api/users"   // only works if path is a flat literal
//
// Returns all matching slices (one per matched route, usually one).
func Endpoint(g *graph.Graph, query string, depth int) []EndpointSlice {
	if depth <= 0 {
		depth = endpointMaxDepth
	}

	limitations := []string{
		"Call chain uses heuristic AST call-graph, not SSA data-flow.",
		"Calls through interfaces or dynamic dispatch may not appear.",
	}

	// Find matching routes.
	matched := matchRoutes(g, query)
	if len(matched) == 0 {
		return nil
	}

	// Build a symbol lookup map for file/line resolution.
	symMap := make(map[string]graph.SymbolNode)
	for _, s := range g.Symbols {
		symMap[s.Name] = s
		if s.Receiver != "" {
			symMap["("+s.Receiver+")."+s.Name] = s
		}
	}

	// Build a callee index: callerName → list of calleeRaw
	calleeIndex := make(map[string][]string)
	for _, c := range g.Calls {
		calleeIndex[c.CallerName] = append(calleeIndex[c.CallerName], c.CalleeRaw)
	}

	// Build SQL index: functionName → sql steps
	sqlIndex := make(map[string][]SQLStep)
	for _, sq := range g.SQLs {
		sqlIndex[sq.Function] = append(sqlIndex[sq.Function], SQLStep{
			Query:    sq.Query,
			Function: sq.Function,
			File:     sq.File,
			Line:     sq.Line,
		})
	}

	// Build env index: functionName → env key
	envIndex := make(map[string][]string)
	for _, e := range g.EnvReads {
		envIndex[e.Function] = append(envIndex[e.Function], e.Key)
	}

	var slices []EndpointSlice
	for _, route := range matched {
		slice := buildSlice(route, symMap, calleeIndex, sqlIndex, envIndex, depth, limitations)
		slices = append(slices, slice)
	}
	return slices
}

// matchRoutes finds HTTPRoute entries matching the query string.
func matchRoutes(g *graph.Graph, query string) []graph.HTTPRoute {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []graph.HTTPRoute
	for _, r := range g.Routes {
		routeStr := strings.ToLower(r.Method + " " + r.Path)
		handlerStr := strings.ToLower(r.Handler)
		if strings.Contains(routeStr, q) || strings.Contains(handlerStr, q) {
			out = append(out, r)
		}
	}
	return out
}

// buildSlice assembles the EndpointSlice for a single matched route.
func buildSlice(
	route graph.HTTPRoute,
	symMap map[string]graph.SymbolNode,
	calleeIndex map[string][]string,
	sqlIndex map[string][]SQLStep,
	envIndex map[string][]string,
	maxDepth int,
	limitations []string,
) EndpointSlice {
	slice := EndpointSlice{
		Route:       route.Method + " " + route.Path,
		Handler:     route.Handler,
		HandlerFile: route.File,
		HandlerLine: route.Line,
		Limitations: limitations,
	}

	// BFS through the call chain.
	visited := make(map[string]bool)
	visited[route.Handler] = true
	queue := []string{route.Handler}
	sqlSeen := make(map[string]bool)
	envSeen := make(map[string]bool)

	for depth := 1; depth <= maxDepth && len(queue) > 0; depth++ {
		var nextQueue []string
		for _, caller := range queue {
			callees := calleeIndex[caller]
			// Deduplicate callees for display
			seen := make(map[string]bool)
			var uniqueCallees []string
			for _, c := range callees {
				if !seen[c] {
					seen[c] = true
					uniqueCallees = append(uniqueCallees, c)
				}
			}

			sym := symMap[caller]
			step := ChainStep{
				Depth:   depth - 1,
				Symbol:  caller,
				File:    sym.File,
				Line:    sym.Line,
				Callees: uniqueCallees,
			}
			// Only append handler step at depth 0 (handled separately above)
			// For depth >= 1, append actual chain steps.
			if depth > 1 {
				slice.CallChain = append(slice.CallChain, step)
			}

			// Collect SQL
			for _, sq := range sqlIndex[caller] {
				key := sq.Function + sq.Query
				if !sqlSeen[key] {
					sqlSeen[key] = true
					slice.SQL = append(slice.SQL, sq)
				}
			}

			// Collect env reads
			for _, env := range envIndex[caller] {
				if !envSeen[env] {
					envSeen[env] = true
					slice.EnvReads = append(slice.EnvReads, env)
				}
			}

			// Enqueue unvisited callees
			for _, c := range uniqueCallees {
				if !visited[c] {
					visited[c] = true
					nextQueue = append(nextQueue, c)
				}
			}
		}

		// Add next-level symbols as chain steps
		for _, sym := range nextQueue {
			s := symMap[sym]
			callees := calleeIndex[sym]
			seen := make(map[string]bool)
			var unique []string
			for _, c := range callees {
				if !seen[c] {
					seen[c] = true
					unique = append(unique, c)
				}
			}
			slice.CallChain = append(slice.CallChain, ChainStep{
				Depth:   depth,
				Symbol:  sym,
				File:    s.File,
				Line:    s.Line,
				Callees: unique,
			})
		}
		queue = nextQueue
	}

	return slice
}
