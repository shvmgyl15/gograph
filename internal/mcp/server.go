package mcp

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

// MCPResponse is the stable structured data payload returned by complex tools.
type MCPResponse struct {
	Query          string               `json:"query,omitempty"`
	Summary        string               `json:"summary,omitempty"`
	Source         string               `json:"source,omitempty"`
	Node           *search.Result       `json:"node,omitempty"`
	Callers        []search.Result      `json:"callers,omitempty"`
	Callees        []search.Result      `json:"callees,omitempty"`
	Findings       []search.Result      `json:"findings,omitempty"`
	InspectFirst   []search.Result      `json:"inspect_first,omitempty"`
	ChangedSymbols []search.Result      `json:"changed_symbols,omitempty"`
	Definitions    []search.Result      `json:"definitions,omitempty"`
	Sites          []search.Result      `json:"sites,omitempty"`
	Paths          []search.TraceResult `json:"paths,omitempty"`
	Files          []string             `json:"files,omitempty"`
	Symbols        []string             `json:"symbols,omitempty"`
	Routes         []string             `json:"routes,omitempty"`
	Tests          []string             `json:"tests,omitempty"`
	TestResults    []search.Result      `json:"test_results,omitempty"`
	SQL            []string             `json:"sql,omitempty"`
	Env            []string             `json:"env,omitempty"`
	Errors         []string             `json:"errors,omitempty"`
	Globals        []string             `json:"globals,omitempty"`
	Risk           map[string]any       `json:"risk,omitempty"`
	Limitations    []string             `json:"limitations,omitempty"`
}

// ExposeToolsForTesting allows tests to access internal tool handlers. Set to a non-nil map before calling NewServer.
var ExposeToolsForTesting map[string]func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

// NewServer creates and returns the MCP server with all tools registered.
func NewServer(g *graph.Graph, rebuild func() (*graph.Graph, error), buildGraph func(string) (*graph.Graph, error)) *server.MCPServer {
	// TODO: Centralize version source with internal/cli.Version to avoid duplication.
	s := server.NewMCPServer(
		"gograph",
		"1.4.59",
		server.WithToolCapabilities(true),
	)

	addTool := func(tool mcp.Tool, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
		// Override mark3labs/mcp-go defaults because gograph tools are purely static analysis (read-only and safe)
		readOnly := true
		destructive := false
		idempotent := true
		openWorld := false

		tool.Annotations.ReadOnlyHint = &readOnly
		tool.Annotations.DestructiveHint = &destructive
		tool.Annotations.IdempotentHint = &idempotent
		tool.Annotations.OpenWorldHint = &openWorld

		s.AddTool(tool, handler)
		if ExposeToolsForTesting != nil {
			ExposeToolsForTesting[tool.Name] = handler
		}
	}

	// Tool: gograph_capabilities
	capabilitiesTool := mcp.NewTool("gograph_capabilities",
		mcp.WithDescription("Discover the available gograph MCP tools, their purposes, recommended workflows, and limitations. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool first to understand what gograph can do. Do NOT call this tool repeatedly once capabilities are cached. COMPLETENESS: Returns a structured checklist of all 50 available tools, recommended agent workflows, and limitations."),
	)
	addTool(capabilitiesTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		resp := map[string]any{
			"summary": "gograph MCP capabilities",
			"tools": []map[string]string{
				{"name": "gograph_capabilities", "purpose": "Discover available tools and workflows."},
				{"name": "gograph_query", "purpose": "Search the repository for symbols, packages, files, or imports using a keyword term."},
				{"name": "gograph_focus", "purpose": "Extract targeted context for a Go package, including files, symbols, internal calls, and dependencies.", "when_to_use": "Use when an agent needs package-level orientation before editing or reviewing package-scoped changes."},
				{"name": "gograph_callers", "purpose": "Find what calls a specific function or method."},
				{"name": "gograph_callees", "purpose": "Find what functions or methods are called from inside the specified function."},
				{"name": "gograph_implementers", "purpose": "Find all structs that implement the specified interface. Use test_only=true to limit to test/mock files."},
				{"name": "gograph_fields", "purpose": "Extract all fields from a specific struct, including their types and struct tags."},
				{"name": "gograph_source", "purpose": "Extract the exact source code for a specific function, method, struct, or interface."},
				{"name": "gograph_orphans", "purpose": "Find functions and methods unreachable from any entry point (main, HTTP routes, exported symbols). Uses full BFS reachability — matches CLI behavior."},
				{"name": "gograph_impact", "purpose": "Traverse the call graph backwards to find all symbols that eventually call the target symbol. Also supports uncommitted=true (blast radius of uncommitted changes) and since=<ref> (blast radius of all changes since a git ref)."},
				{"name": "gograph_boundaries", "purpose": "Verify package architecture constraints against boundaries.json."},
				{"name": "gograph_endpoint", "purpose": "Full vertical slice for one HTTP endpoint: route, handler, downstream call chain (BFS), SQL emitted, env reads."},
				{"name": "gograph_api", "purpose": "Compare the public-facing contract and integration surface drift against a baseline git reference."},
				{"name": "gograph_routes", "purpose": "Extract all HTTP REST API routes found in the codebase."},
				{"name": "gograph_context", "purpose": "Bundles node details, callers, callees, tests, source, and architectural role into one structured response. Set uncommitted=true to bundle all uncommitted symbols in one call."},
				{"name": "gograph_plan", "purpose": "Safe edit planning before code changes. Set with_context=true to bundle full context for each inspect_first symbol — eliminates follow-up context calls."},
				{"name": "gograph_review", "purpose": "Post-edit or symbol-focused review. Summarizes what changed and its risk profile."},
				{"name": "gograph_errorflow", "purpose": "Trace likely error paths up to entry points (HTTP routes or CLI commands)."},
				{"name": "gograph_imports", "purpose": "Find all files that import a specific external package."},
				{"name": "gograph_dependents", "purpose": "Find all packages that import the named package (inverse of deps). Essential before package-level refactors."},
				{"name": "gograph_node", "purpose": "Full AST metadata for one symbol: kind, file, line, signature, doc, struct fields."},
				{"name": "gograph_envs", "purpose": "List every os.Getenv / viper.Get* read in the codebase. Optional filter by key name."},
				{"name": "gograph_interfaces", "purpose": "Find interfaces satisfied by a named struct (duck-typing). Inverse of gograph_implementers."},
				{"name": "gograph_tests", "purpose": "Find test functions that exercise a named symbol. Omit symbol to list all test edges."},
				{"name": "gograph_hotspot", "purpose": "Rank functions by incoming call count (fan-in). Shows the most-depended-on code to study first."},
				{"name": "gograph_deps", "purpose": "Import dependency tree of a package. Use transitive=true for the full BFS closure."},
				{"name": "gograph_changes", "purpose": "Symbols modified/added/deleted since last build. Use git_ref to compare against a git reference."},
				{"name": "gograph_path", "purpose": "Shortest call chain between two symbols (BFS). Confirms whether a handler reaches a given function."},
				{"name": "gograph_stale", "purpose": "Check whether graph.json is older than any source file. Run before structural analysis."},
				{"name": "gograph_complexity", "purpose": "Cyclomatic complexity per function, sorted highest first. Labels: LOW / MEDIUM / HIGH / VERY HIGH."},
				{"name": "gograph_coupling", "purpose": "Fan-in, fan-out, and instability per package. Instability range [0,1]: 0=stable, 1=unstable."},
				{"name": "gograph_returnusage", "purpose": "Show how each caller uses the return value of a function (discarded/assigned/partially_ignored/returned/passed). Run before changing a return signature."},
				{"name": "gograph_arity", "purpose": "Find functions with too many arguments (long parameter list smell). Default minimum: 5."},
				{"name": "gograph_concurrency", "purpose": "Map goroutine spawns, channel ops, mutex locks, WaitGroups, and sync.Once. Optional filter by kind."},
				{"name": "gograph_fixtures", "purpose": "Find test helper structs and functions in test files for a package."},
				{"name": "gograph_godobj", "purpose": "Find god-object struct candidates scored by method count, field count, and outgoing calls."},
				{"name": "gograph_skeleton", "purpose": "Full repository API signatures with bodies stripped. WARNING: can be very large on big repos."},
				{"name": "gograph_mutate", "purpose": "Find functions that mutate a specific struct field."},
				{"name": "gograph_sql", "purpose": "Extract database SQL queries found in the codebase."},
				{"name": "gograph_errors", "purpose": "Extract custom error messages and panics."},
				{"name": "gograph_embeds", "purpose": "Find what structs embed the given target struct."},
				{"name": "gograph_public", "purpose": "Show only the exported (public) symbols of a specific package."},
				{"name": "gograph_usages", "purpose": "Find every place a named type appears in function signatures (param/return) and struct fields. Run before changing an interface to see its full consumption blast radius."},
				{"name": "gograph_literals", "purpose": "Find all composite-literal initialization sites for a named struct. Run before adding a required field — every site returned breaks at compile time."},
				{"name": "gograph_constructors", "purpose": "Find factory functions returning the named struct."},
				{"name": "gograph_schema", "purpose": "Find structs mapped to a database table or schema via struct tags."},
				{"name": "gograph_globals", "purpose": "Find package-level variables and functions mutating them."},
				{"name": "gograph_mocks", "purpose": "Alias for gograph_implementers with test_only=true. Kept for compatibility."},
				{"name": "gograph_explain", "purpose": "LLM-ready architectural summary. Synthesizes callers, callees, complexity, SQL, env, routes, concurrency, tests, and interface satisfaction into one narrative with an opinionated role classification."},
			},
			"recommended_workflows": map[string][]string{
				"before_edit":   {"gograph_context", "gograph_plan"},
				"after_edit":    {"gograph_review", "gograph_api", "gograph_boundaries"},
				"error_changes": {"gograph_errorflow", "gograph_review"},
				"api_changes":   {"gograph_api", "gograph_review"},
			},
			"limitations": []string{
				"gograph is static analysis.",
				"MCP tools do not execute target repository code.",
				"MCP tools do not add network access.",
				"Errorflow uses heuristic static call-graph and AST reference analysis. It does not perform SSA or full data-flow tracking.",
			},
		}
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_query
	queryTool := mcp.NewTool("gograph_query",
		mcp.WithDescription("Search the Go repository for symbols, packages, files, or imports using a keyword term. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool during the initial exploration phase when you have a keyword or feature name but do not know which files or packages contain the relevant code. Do NOT use if you already know the exact symbol name (use gograph_source or gograph_node instead). COMPLETENESS: Returns a structured list of matching symbols, files, and imports, along with their location and kind. Example: 'Graph', 'Serve'."),
		mcp.WithString("term", mcp.Required(), mcp.Description("The keyword search term to locate in symbols, files, and imports (e.g., 'AuthService', 'token', 'router')")),
	)
	addTool(queryTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		term, ok := args["term"].(string)
		if !ok {
			return mcp.NewToolResultError("term must be a string"), nil
		}
		results := search.Query(g, []string{term})
		return formatResults(results), nil
	})

	// Tool: gograph_focus
	focusTool := mcp.NewTool("gograph_focus",
		mcp.WithDescription("Extract targeted call-graph and dependency context for a single Go package. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when you need to understand the architecture, files, internal calls, and dependencies of a specific package directory before making edits. Do NOT use for global searches (use gograph_query instead). COMPLETENESS: Requires 'package' parameter. Returns a detailed listing of all files, defined symbols, calls, and internal dependencies within the target package. Example package: 'internal/search'."),
		mcp.WithString("package", mcp.Required(), mcp.Description("The package path or name to focus on (e.g., 'internal/auth')")),
	)
	addTool(focusTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		pkg, ok := args["package"].(string)
		if !ok {
			return mcp.NewToolResultError("package must be a string"), nil
		}
		results := search.Focus(g, pkg)
		return formatResults(results), nil
	})

	// Tool: gograph_callers
	callersTool := mcp.NewTool("gograph_callers",
		mcp.WithDescription("Find and list all functions or methods that call the specified target function. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when performing impact analysis, tracing call hierarchies, or identifying who consumes a particular API. Do NOT use if you want downstream callees (use gograph_callees instead). COMPLETENESS: Requires 'function' parameter. Returns a structured list of calling function symbols and their exact file locations. Example function: 'BuildGraph'."),
		mcp.WithString("function", mcp.Required(), mcp.Description("The name of the target function to find callers for (e.g., 'BuildGraph', 'Serve')")),
	)
	addTool(callersTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		fn, ok := args["function"].(string)
		if !ok {
			return mcp.NewToolResultError("function must be a string"), nil
		}
		results := search.Callers(g, fn, true)
		return formatResults(results), nil
	})

	// Tool: gograph_callees
	calleesTool := mcp.NewTool("gograph_callees",
		mcp.WithDescription("Find and list all downstream functions or methods invoked from inside the specified caller function. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when you need to understand the downstream execution flow, dependencies, and external calls of a single function. Do NOT use if you want upstream callers (use gograph_callers instead). COMPLETENESS: Requires 'function' parameter. Returns a complete array of callees, including their package paths and signatures. Example function: 'Serve'."),
		mcp.WithString("function", mcp.Required(), mcp.Description("The name of the calling function to inspect callees for (e.g., 'Serve', 'runMCP')")),
	)
	addTool(calleesTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		fn, ok := args["function"].(string)
		if !ok {
			return mcp.NewToolResultError("function must be a string"), nil
		}
		results := search.Callees(g, fn, true)
		return formatResults(results), nil
	})

	// Tool: gograph_implementers
	implementersTool := mcp.NewTool("gograph_implementers",
		mcp.WithDescription("Find all structs in the codebase that implement the specified interface via Go duck-typing. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when tracing polymorphism, identifying dependency injection structures, or locating mock implementations. Do NOT use if you are looking for struct fields (use gograph_fields instead). COMPLETENESS: Requires 'interface' parameter. Returns a detailed list of implementing concrete structs and their package file locations. Example interface: 'error'."),
		mcp.WithString("interface", mcp.Required(), mcp.Description("The name of the interface (e.g., 'AuthService')")),
		mcp.WithBoolean("test_only", mcp.Description("If true, return only structs defined in test or mock files (replaces gograph_mocks)")),
	)
	addTool(implementersTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		iface, ok := args["interface"].(string)
		if !ok {
			return mcp.NewToolResultError("interface must be a string"), nil
		}
		if testOnly, _ := args["test_only"].(bool); testOnly {
			results := search.Mocks(g, iface)
			return formatResults(results), nil
		}
		results := search.Implementers(g, iface)
		return formatResults(results), nil
	})

	// Tool: gograph_fields
	fieldsTool := mcp.NewTool("gograph_fields",
		mcp.WithDescription("Extract all fields, types, and struct tags declared inside a specific Go struct. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when inspecting struct definitions, verifying struct layouts, mapping JSON/database tags, or generating serialization logic. Do NOT use if you are looking for struct methods (use gograph_node or gograph_source instead). COMPLETENESS: Requires 'struct' parameter. Returns field names, Go types, and exact struct tag string metadata, providing full visibility into struct fields without manual code viewing. Example struct: 'Graph'."),
		mcp.WithString("struct", mcp.Required(), mcp.Description("The exact name of the target struct to inspect fields for (e.g., 'Config', 'User')")),
	)
	addTool(fieldsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		structName, ok := args["struct"].(string)
		if !ok {
			return mcp.NewToolResultError("struct must be a string"), nil
		}
		results := search.Fields(g, structName)
		return formatResults(results), nil
	})

	// Tool: gograph_source
	sourceTool := mcp.NewTool("gograph_source",
		mcp.WithDescription("Extract the exact Go source code body for a specific function, method, struct, or interface. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when you need to read the implementation details of a specific symbol without loading large files. Do NOT use if you need call hierarchies (use gograph_callers/gograph_callees instead). COMPLETENESS: Requires 'symbol' parameter. Returns the exact block of source code defining the target symbol, complete with line numbers. Example symbol: 'BuildGraph'."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("The name of the symbol (e.g., 'ValidateToken' or 'AuthService')")),
	)
	addTool(sourceTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		sym, ok := args["symbol"].(string)
		if !ok {
			return mcp.NewToolResultError("symbol must be a string"), nil
		}
		// MCP currently defaults to root = "."
		code, err := search.Source(g, ".", sym)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(code), nil
	})

	// Tool: gograph_orphans
	orphansTool := mcp.NewTool("gograph_orphans",
		mcp.WithDescription("Find functions and methods that are unreachable from any entry point (main, HTTP routes, exported symbols). BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when clean-up is needed or to locate dead code that should be purged from the repository. Do NOT use for checking specific symbol usages (use gograph_usages instead). COMPLETENESS: Returns a list of orphan symbols, their packages, and file definitions."),
	)
	addTool(orphansTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		results := search.ReachableOrphans(g)
		return formatResults(results), nil
	})

	// Tool: gograph_impact
	impactTool := mcp.NewTool("gograph_impact",
		mcp.WithDescription("Traverse the call graph backwards to find all symbols that eventually call the target (blast radius). BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to determine the blast radius of a sensitive core component before refactoring. Do NOT use for direct callers only (use gograph_callers instead). COMPLETENESS: Supports single symbol, uncommitted changes, or since a git ref. Returns a transitive list of all upstream affected symbols. Example symbol: 'BuildGraph'."),
		mcp.WithString("symbol", mcp.Description("Symbol name for single-symbol blast radius (e.g., 'ValidateToken')")),
		mcp.WithBoolean("uncommitted", mcp.Description("If true, compute blast radius of all uncommitted modified symbols")),
		mcp.WithString("since", mcp.Description("Git ref (e.g. 'main', 'HEAD~5'): blast radius of all symbols changed since this ref")),
	)
	addTool(impactTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		// --since <ref> mode
		if ref, ok := args["since"].(string); ok && ref != "" {
			root, _ := filepath.Abs(".")
			changes, err := search.ChangesByGitRef(g, root, ref)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if len(changes.Symbols) == 0 {
				return mcp.NewToolResultText(fmt.Sprintf(`{"count":0,"message":"No Go symbol changes found since %q."}`, ref)), nil
			}
			names := make([]string, 0, len(changes.Symbols))
			for _, s := range changes.Symbols {
				names = append(names, s.Name)
			}
			reason := fmt.Sprintf("downstream impact of changes since %s (%d symbols)", ref, len(names))
			results := search.ImpactMultiple(g, names, reason, true)
			return formatResults(results), nil
		}

		// --uncommitted mode
		if u, _ := args["uncommitted"].(bool); u {
			syms, err := search.UncommittedSymbols(g)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if len(syms) == 0 {
				return mcp.NewToolResultText(`{"count":0,"message":"No uncommitted modified symbols found."}`), nil
			}
			reason := fmt.Sprintf("downstream impact of uncommitted changes (%d symbols)", len(syms))
			results := search.ImpactMultiple(g, syms, reason, true)
			return formatResults(results), nil
		}

		// single symbol mode
		sym, ok := args["symbol"].(string)
		if !ok || sym == "" {
			return mcp.NewToolResultError("must provide symbol, set uncommitted=true, or provide a since ref"), nil
		}
		results := search.Impact(g, sym, true)
		return formatResults(results), nil
	})

	// Tool: gograph_boundaries
	boundariesTool := mcp.NewTool("gograph_boundaries",
		mcp.WithDescription("Verify package architecture constraints against boundaries.json to detect forbidden imports and illegal layer dependencies. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to check if packages conform to domain design constraints or to debug illegal import cycles and layered violations. Do NOT use for general dependency mapping (use gograph_deps instead). COMPLETENESS: Optional 'config' path. Returns a structured boundary compliance report listing authorized vs unauthorized dependencies."),
		mcp.WithString("config", mcp.Description("Optional file path to boundary constraints configuration (defaults to .gograph/boundaries.json)")),
	)
	addTool(boundariesTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}

		configPath := ".gograph/boundaries.json"
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if cp, ok := args["config"].(string); ok && cp != "" {
				configPath = cp
			}
		}

		results, err := search.Boundaries(g, configPath)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		summary := "Boundary violations found."
		pass := false
		if len(results) == 0 {
			summary = "No boundary violations found."
			pass = true
		}

		resp := map[string]any{
			"summary":  summary,
			"findings": results,
			"risk": map[string]any{
				"pass":            pass,
				"violation_count": len(results),
			},
		}

		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	// Tool: gograph_endpoint
	endpointTool := mcp.NewTool("gograph_endpoint",
		mcp.WithDescription("Audit and analyze HTTP route endpoints, handlers, and anonymous closure bodies. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to review HTTP interface definitions, verify request flow bindings, or audit handler implementations. Do NOT use for internal Go functions (use gograph_source instead). COMPLETENESS: Returns HTTP methods, paths, handler signatures, and handler bodies. Example package: 'cmd/gograph'."),
		mcp.WithString("query", mcp.Required(), mcp.Description(`Route pattern ("POST /api/users"), path fragment ("/users"), or handler symbol name ("CreateUser")`)),
		mcp.WithNumber("depth", mcp.Description("BFS depth for call chain traversal (default: 5)")),
	)
	addTool(endpointTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, _ := request.Params.Arguments.(map[string]any)
		query, _ := args["query"].(string)
		if query == "" {
			return mcp.NewToolResultError("query is required"), nil
		}
		depth := 5
		if d, ok := args["depth"].(float64); ok && d > 0 {
			depth = int(d)
		}
		slices := search.Endpoint(g, query, depth, false)
		if len(slices) == 0 {
			b, _ := json.MarshalIndent(map[string]any{
				"query":   query,
				"found":   false,
				"message": "No matching HTTP routes found. Run gograph_routes to see available routes.",
			}, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		}
		b, _ := json.MarshalIndent(map[string]any{
			"query":  query,
			"found":  true,
			"count":  len(slices),
			"slices": slices,
		}, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	// Tool: gograph_api

	apiTool := mcp.NewTool("gograph_api",
		mcp.WithDescription("Extract the public API surface of a Go package, listing all exported functions, types, and interfaces. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when building integration layers, documenting public interfaces, or verifying exports. Do NOT use for private/unexported symbols (use gograph_node instead). COMPLETENESS: Requires 'package' parameter. Returns a clean summary of exported package API symbols. Example package: 'internal/search'."),
		mcp.WithString("since", mcp.Required(), mcp.Description("The baseline git reference (e.g., 'main' or 'HEAD~1') to compare against")),
	)
	addTool(apiTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		sinceRef, ok := args["since"].(string)
		if !ok {
			return mcp.NewToolResultError("since must be a string"), nil
		}

		// Validate sinceRef with a positive allowlist
		safeGitRef := regexp.MustCompile(`^[A-Za-z0-9._/\-~^]+$`)
		if sinceRef == "" || strings.HasPrefix(sinceRef, "-") || !safeGitRef.MatchString(sinceRef) {
			return mcp.NewToolResultError("invalid since value: contains unsafe characters or is empty"), nil
		}

		// Run a temporary git archive extraction for the baseline
		tmpDir, err := os.MkdirTemp("", "gograph-baseline-*")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("error creating temp dir: %v", err)), nil
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		// Archive the full ref, letting graph builder ignore non-Go files
		cmd := exec.Command("git", "archive", "--format=tar", sinceRef)
		var errBuf strings.Builder
		cmd.Stderr = &errBuf

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("error piping git archive: %v", err)), nil
		}

		if err := cmd.Start(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("git archive start failed: %v", err)), nil
		}

		tr := tar.NewReader(stdout)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				return mcp.NewToolResultError(fmt.Sprintf("tar read error: %v", err)), nil
			}

			target := filepath.Join(tmpDir, header.Name)
			// Check for zip slip
			if !strings.HasPrefix(target, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
				continue
			}

			switch header.Typeflag {
			case tar.TypeDir:
				_ = os.MkdirAll(target, os.FileMode(header.Mode))
			case tar.TypeReg:
				_ = os.MkdirAll(filepath.Dir(target), 0755)
				f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
				if err == nil {
					_, _ = io.Copy(f, tr)
					_ = f.Close()
				}
			}
		}

		if err := cmd.Wait(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("git archive failed (invalid ref?): %v, stderr: %s", err, errBuf.String())), nil
		}

		baselineGraph, err := buildGraph(tmpDir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("error building baseline graph: %v", err)), nil
		}

		res := search.APIDrift(baselineGraph, g, sinceRef)

		// Convert the APIDriftResult into formatted JSON string for the agent
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	// Tool: gograph_routes
	routesTool := mcp.NewTool("gograph_routes",
		mcp.WithDescription("Discover all registered HTTP routes and endpoints defined inside the codebase. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to map out the entire web API structure of a microservice or application. Do NOT use if you need downstream call details (use gograph_endpoint instead). COMPLETENESS: Returns a structured table of HTTP method/route paths mapped to Go handlers."),
	)
	addTool(routesTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		results := search.Routes(g)
		return formatResults(results), nil
	})

	// Tool: gograph_context
	contextTool := mcp.NewTool("gograph_context",
		mcp.WithDescription("Build a highly condensed diagnostic context file for a set of target Go packages or symbols. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to summarize multiple packages for LLM consumption, reducing token overhead. Do NOT use for deep line-by-line reading (use gograph_source instead). COMPLETENESS: Returns a compact text summary of package structures, exported APIs, and type definitions. Example package: 'internal/search'."),
		mcp.WithString("symbol", mcp.Description("The exact name or ID of the symbol to retrieve context for.")),
		mcp.WithBoolean("uncommitted", mcp.Description("If true, return context for all uncommitted modified symbols bundled in one response.")),
	)
	addTool(contextTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		root, _ := filepath.Abs(".")

		if u, _ := args["uncommitted"].(bool); u {
			syms, err := search.UncommittedSymbols(g)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if len(syms) == 0 {
				data, _ := json.MarshalIndent(map[string]any{
					"summary":  "No uncommitted modified symbols found.",
					"count":    0,
					"contexts": []any{},
				}, "", "  ")
				return mcp.NewToolResultText(string(data)), nil
			}
			type symbolContext struct {
				Symbol  string          `json:"symbol"`
				Role    string          `json:"role,omitempty"`
				Node    *search.Result  `json:"node,omitempty"`
				Source  string          `json:"source,omitempty"`
				Callers []search.Result `json:"callers,omitempty"`
				Callees []search.Result `json:"callees,omitempty"`
				Tests   []string        `json:"tests,omitempty"`
			}
			var contexts []symbolContext
			for _, sym := range syms {
				r := search.Context(g, root, sym)
				if r == nil {
					continue
				}
				sc := symbolContext{
					Symbol:  sym,
					Role:    r.Role,
					Source:  r.Source,
					Callers: r.Callers,
					Callees: r.Callees,
				}
				if len(r.Node) > 0 {
					sc.Node = &r.Node[0]
				}
				for _, t := range r.Tests {
					sc.Tests = append(sc.Tests, t.Name)
				}
				contexts = append(contexts, sc)
			}
			data, err := json.MarshalIndent(map[string]any{
				"summary":  fmt.Sprintf("Context for %d uncommitted symbol(s)", len(contexts)),
				"count":    len(contexts),
				"contexts": contexts,
			}, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		}

		symbol, ok := args["symbol"].(string)
		if !ok || symbol == "" {
			return mcp.NewToolResultError("must provide either symbol or set uncommitted to true"), nil
		}
		result := search.Context(g, root, symbol)
		if result == nil {
			return mcp.NewToolResultText("{}"), nil
		}
		var node *search.Result
		if len(result.Node) > 0 {
			node = &result.Node[0]
		}
		resp := MCPResponse{
			Summary: "Context for " + symbol,
			Node:    node,
			Source:  result.Source,
			Callers: result.Callers,
			Callees: result.Callees,
			Risk:    map[string]any{"role": result.Role},
		}
		resp.TestResults = result.Tests
		for _, t := range result.Tests {
			resp.Tests = append(resp.Tests, t.Name)
		}
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_plan
	planTool := mcp.NewTool("gograph_plan",
		mcp.WithDescription("Analyze structural changes and dependencies to output a detailed refactoring execution plan. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool before making complex multi-file architectural changes or package refactoring. Do NOT use for trivial single-file edits. COMPLETENESS: Returns a step-by-step refactoring workflow detailing symbol modifications, dependency steps, and testing verification checklists."),
		mcp.WithString("symbol", mcp.Description("The name of the symbol you intend to modify (e.g., 'ValidateToken')")),
		mcp.WithBoolean("uncommitted", mcp.Description("Set to true to generate a global plan for all currently uncommitted changes across the repository")),
		mcp.WithBoolean("with_context", mcp.Description("If set to true, bundles full context, source code, callers, callees, and architectural roles for each symbol to be inspected")),
	)
	addTool(planTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		var symbolNames []string
		var title string
		if u, ok := args["uncommitted"].(bool); ok && u {
			syms, err := search.UncommittedSymbols(g)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			symbolNames = syms
			title = "Uncommitted Changes"
		} else if sym, ok := args["symbol"].(string); ok && sym != "" {
			symbolNames = []string{sym}
			title = sym
		} else {
			return mcp.NewToolResultError("must provide either symbol or set uncommitted to true"), nil
		}

		planRes := search.Plan(g, symbolNames, title)
		withContext, _ := args["with_context"].(bool)

		type inspectContext struct {
			Symbol  string          `json:"symbol"`
			Role    string          `json:"role,omitempty"`
			Node    *search.Result  `json:"node,omitempty"`
			Source  string          `json:"source,omitempty"`
			Callers []search.Result `json:"callers,omitempty"`
			Callees []search.Result `json:"callees,omitempty"`
			Tests   []string        `json:"tests,omitempty"`
		}

		resp := map[string]any{
			"summary":       "Change plan for " + planRes.Title,
			"inspect_first": planRes.ReadFirst,
			"tests":         planRes.Tests,
			"routes":        planRes.Routes,
			"env":           planRes.Envs,
			"risk": map[string]any{
				"public_api":     planRes.PublicAPI,
				"touches_sql":    planRes.TouchesSQL,
				"touches_routes": len(planRes.Routes) > 0,
				"touches_env":    len(planRes.Envs) > 0,
			},
		}

		if withContext {
			root, _ := filepath.Abs(".")
			var contexts []inspectContext
			for _, sym := range planRes.ReadFirst {
				r := search.Context(g, root, sym.Name)
				if r == nil {
					continue
				}
				ic := inspectContext{
					Symbol:  sym.Name,
					Role:    r.Role,
					Source:  r.Source,
					Callers: r.Callers,
					Callees: r.Callees,
				}
				if len(r.Node) > 0 {
					ic.Node = &r.Node[0]
				}
				for _, t := range r.Tests {
					ic.Tests = append(ic.Tests, t.Name)
				}
				contexts = append(contexts, ic)
			}
			resp["inspect_contexts"] = contexts
		}

		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_review
	reviewTool := mcp.NewTool("gograph_review",
		mcp.WithDescription("Run an architectural consistency and design constraint review against defined code boundaries or specific symbols. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool during post-edit verification, CI pipelines, or refactoring phases to ensure new additions do not violate boundary limits. Do NOT use for general dependency tree listing (use gograph_deps instead). COMPLETENESS: Requires either 'symbol' or 'uncommitted' set to true. Returns a structured checklist of all rules evaluated, listing exact package paths and files violating architectural boundaries. Example symbol: 'Graph'."),
		mcp.WithString("symbol", mcp.Description("The name of the target symbol to run the design review for (e.g. 'AuthService')")),
		mcp.WithBoolean("uncommitted", mcp.Description("Set to true to review all uncommitted/modified changes in the repository")),
	)
	addTool(reviewTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		var symbolNames []string
		var title string
		if u, ok := args["uncommitted"].(bool); ok && u {
			syms, err := search.UncommittedSymbols(g)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			symbolNames = syms
			title = "Uncommitted Changes"
		} else if sym, ok := args["symbol"].(string); ok && sym != "" {
			symbolNames = []string{sym}
			title = sym
		} else {
			return mcp.NewToolResultError("must provide either symbol or set uncommitted to true"), nil
		}

		revRes := search.Review(g, symbolNames, title)

		resp := MCPResponse{
			Summary:        "Code Review for " + revRes.Title,
			ChangedSymbols: revRes.Changes,
			Tests:          revRes.Tests,
			Routes:         revRes.Routes,
			Env:            revRes.Envs,
			Errors:         revRes.Errors,
			Risk: map[string]any{
				"public_api":      revRes.PublicAPI,
				"touches_sql":     revRes.TouchesSQL,
				"touches_routes":  len(revRes.Routes) > 0,
				"touches_env":     len(revRes.Envs) > 0,
				"touches_errors":  len(revRes.Errors) > 0,
				"touches_globals": false,
			},
		}
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_errorflow
	errorflowTool := mcp.NewTool("gograph_errorflow",
		mcp.WithDescription("Trace the flow and propagation of returned errors from a specific origin function. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when auditing error-handling completeness, locating unhandled errors, or tracing failure modes. Do NOT use for standard call graphs (use gograph_callers/callees instead). COMPLETENESS: Requires 'function' parameter. Returns error bubbling paths up the call chain. Example function: 'BuildGraph'."),
		mcp.WithString("term", mcp.Description("The error string or sentinel error name (e.g., 'ErrInvalidToken' or 'invalid token')")),
		mcp.WithString("query", mcp.Description("The error string or sentinel error name (preferred over term)")),
		mcp.WithBoolean("no_tests", mcp.Description("If true, exclude test files from related-test collection (matches CLI --no-tests)")),
	)
	addTool(errorflowTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		var term string
		if q, ok := args["query"].(string); ok && q != "" {
			term = q
		} else if t, ok := args["term"].(string); ok && t != "" {
			term = t
		}

		if term == "" {
			return mcp.NewToolResultError("query or term must be a non-empty string"), nil
		}

		noTests, _ := args["no_tests"].(bool)
		report := search.ErrorFlow(g, term, !noTests)

		resp := MCPResponse{
			Query:       report.Term,
			Summary:     "ErrorFlow Report for " + report.Term,
			Definitions: report.DefinitionSites,
			Sites:       report.ReturnSites,
			Paths:       report.Paths,
			Risk:        map[string]any{},
			Limitations: []string{
				"Errorflow uses heuristic static call-graph and AST reference analysis. It does not perform SSA or full data-flow tracking.",
			},
		}
		for _, t := range report.RelatedTests {
			resp.Tests = append(resp.Tests, t.Name)
		}
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_imports
	importsTool := mcp.NewTool("gograph_imports",
		mcp.WithDescription("Find all files and packages that import a specific internal or third-party package. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when mapping package dependency relationships, isolating features, or tracing where third-party libraries are used. Do NOT use if you need a package's own imports (use gograph_deps instead). COMPLETENESS: Requires 'package' parameter. Returns a complete array of file paths and importing packages referencing the target import path. Example package: 'golang.org/x/tools'."),
		mcp.WithString("package", mcp.Required(), mcp.Description("The exact import path of the target package to trace imports for (e.g., 'github.com/redis/go-redis')")),
	)
	addTool(importsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		pkg, ok := args["package"].(string)
		if !ok {
			return mcp.NewToolResultError("package must be a string"), nil
		}
		results := search.ExternalImports(g, pkg)
		return formatResults(results), nil
	})

	// Tool: gograph_dependents
	dependentsTool := mcp.NewTool("gograph_dependents",
		mcp.WithDescription("Find all downstream packages that depend on or import the specified target package. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to identify downstream impact before changing a public package interface. Do NOT use for direct callers of a single function (use gograph_callers instead). COMPLETENESS: Requires 'package' parameter. Returns concrete packages importing the target package. Example package: 'internal/graph'."),
		mcp.WithString("package", mcp.Required(), mcp.Description("The package to find dependents for (e.g., 'internal/auth', 'auth', or a full import path)")),
	)
	addTool(dependentsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		pkg, ok := args["package"].(string)
		if !ok || pkg == "" {
			return mcp.NewToolResultError("package must be a non-empty string"), nil
		}
		results := search.Dependents(g, pkg)
		return formatResults(results), nil
	})

	// Tool: gograph_sql
	sqlTool := mcp.NewTool("gograph_sql",
		mcp.WithDescription("Extract and analyze all raw SQL query literals, database touches, and transaction blocks inside the codebase. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when auditing database interactions, debugging database performance, or reviewing raw queries. Optional filter by keyword. Do NOT use for Go structs mapped to database tables (use gograph_schema instead). COMPLETENESS: Returns a list of matching SQL statements, their file and line locations, and the parent Go functions containing the database touch. Example term: 'SELECT'."),
		mcp.WithString("term", mcp.Description("Optional SQL keyword or table name to filter database queries (e.g., 'SELECT', 'users')")),
	)
	addTool(sqlTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		term := ""
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if t, ok := args["term"].(string); ok {
				term = t
			}
		}
		results := search.SQL(g, term)
		return formatResults(results), nil
	})

	// Tool: gograph_errors
	errorsTool := mcp.NewTool("gograph_errors",
		mcp.WithDescription("Locate all explicit error creation points (errors.New, fmt.Errorf, sentinel definitions) inside the codebase. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when auditing error taxonomies, cataloging error codes, or standardizing error handling. Do NOT use to trace propagation paths (use gograph_errorflow instead). COMPLETENESS: Returns a structured checklist of all matched error creation spots, showing their file and line coordinates."),
		mcp.WithString("term", mcp.Description("Optional keyword to filter the returned error structures (e.g., 'ErrInvalid', 'unauthorized')")),
	)
	addTool(errorsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		term := ""
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if t, ok := args["term"].(string); ok {
				term = t
			}
		}
		results := search.Errors(g, term, true)
		return formatResults(results), nil
	})

	// Tool: gograph_embeds
	embedsTool := mcp.NewTool("gograph_embeds",
		mcp.WithDescription("Identify and list all parent structures that compositionally embed the specified target struct. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when analyzing struct nesting, identifying compositions, or tracing where shared types are embedded. Do NOT use if you are looking for interfaces implemented (use gograph_implementers instead). COMPLETENESS: Requires 'struct' parameter. Returns a detailed array of embedding parent structs and their file definitions. Example struct: 'Symbol'."),
		mcp.WithString("struct", mcp.Required(), mcp.Description("The exact name of the target struct to inspect embedding relationships for (e.g., 'Symbol', 'PackageNode')")),
	)
	addTool(embedsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		structName, ok := args["struct"].(string)
		if !ok {
			return mcp.NewToolResultError("struct must be a string"), nil
		}
		results := search.Embeds(g, structName)
		return formatResults(results), nil
	})

	// Tool: gograph_public
	publicTool := mcp.NewTool("gograph_public",
		mcp.WithDescription("Extract all public, exported symbols across the entire repository, categorized by package. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when building a high-level catalog of system services or creating integration boundaries. Do NOT use if you need internal private details (use gograph_node instead). COMPLETENESS: Returns a structured outline of all exported functions, structs, interfaces, and variables."),
		mcp.WithString("package", mcp.Required(), mcp.Description("The package name or path to inspect (e.g., 'internal/auth')")),
	)
	addTool(publicTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		pkgName, ok := args["package"].(string)
		if !ok {
			return mcp.NewToolResultError("package must be a string"), nil
		}
		results := search.Public(g, pkgName)
		return formatResults(results), nil
	})

	initNewTools(g, rebuild, addTool)

	// Start stdio server
	return s
}

// Serve runs the gograph MCP server over stdio.
func Serve(g *graph.Graph, rebuild func() (*graph.Graph, error), buildGraph func(string) (*graph.Graph, error)) error {
	s := NewServer(g, rebuild, buildGraph)
	return server.ServeStdio(s)
}

func formatResults(results []search.Result) *mcp.CallToolResult {
	if len(results) == 0 {
		return mcp.NewToolResultText("No results found.")
	}

	var sb strings.Builder
	for _, r := range results {
		sb.WriteString(r.String() + "\n")
	}

	return mcp.NewToolResultText(sb.String())
}

func initNewTools(g *graph.Graph, rebuild func() (*graph.Graph, error), addTool func(tool mcp.Tool, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error))) {
	// Tool: gograph_usages
	usagesTool := mcp.NewTool("gograph_usages",
		mcp.WithDescription("Find all references and semantic usages of a specific function, struct, or variable across the codebase. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when renaming symbols, checking references, or identifying dead code usages. Do NOT use for AST metadata only (use gograph_node instead). COMPLETENESS: Requires 'symbol' parameter. Returns files and exact line locations containing references to the target symbol. Example symbol: 'BuildGraph'."),
		mcp.WithString("type", mcp.Required(), mcp.Description("The type name to search for (e.g., 'AuthService', 'Repository')")),
	)
	addTool(usagesTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		typeName, ok := args["type"].(string)
		if !ok || typeName == "" {
			return mcp.NewToolResultError("type must be a non-empty string"), nil
		}
		results := search.Usages(g, typeName)
		return formatResults(results), nil
	})

	// Tool: gograph_literals
	literalsTool := mcp.NewTool("gograph_literals",
		mcp.WithDescription("Find all primitive literals (strings, integers, floats, booleans) matching a filter pattern. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to identify hardcoded magic values, config strings, or credentials. Do NOT use for raw database queries (use gograph_sql instead). COMPLETENESS: Optional 'type' filter (string/int). Returns a list of matched literals and their code positions. Example pattern: 'mcp'."),
		mcp.WithString("struct", mcp.Required(), mcp.Description("The name of the struct (e.g., 'User')")),
	)
	addTool(literalsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		structName, ok := args["struct"].(string)
		if !ok || structName == "" {
			return mcp.NewToolResultError("struct must be a non-empty string"), nil
		}
		results := search.Literals(g, structName)
		return formatResults(results), nil
	})

	// Tool: gograph_constructors
	constructorsTool := mcp.NewTool("gograph_constructors",
		mcp.WithDescription("Find factory and constructor functions that instantiate and return the specified Go struct. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when auditing object initialization patterns, looking for existing builder functions, or verifying correct struct instantiation. Do NOT use if you need struct fields (use gograph_fields instead). COMPLETENESS: Requires 'struct' parameter. Returns a detailed list of constructor function symbols, their file locations, and signatures, showing exactly how the target struct is built. Example struct: 'Graph'."),
		mcp.WithString("struct", mcp.Required(), mcp.Description("The exact name of the target Go struct to find constructors for (e.g., 'User', 'Config')")),
	)
	addTool(constructorsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		str, ok := args["struct"].(string)
		if !ok || str == "" {
			return mcp.NewToolResultError("missing 'struct' argument"), nil
		}
		results := search.Constructors(g, str)
		return formatResults(results), nil
	})

	// Tool: gograph_schema
	schemaTool := mcp.NewTool("gograph_schema",
		mcp.WithDescription("Locate Go structs that map to a specific database table or schema using struct tags like db or gorm. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when mapping ORM structures, tracing database model boundaries, or locating table schemas. Do NOT use for non-ORM Go structs (use gograph_fields instead). COMPLETENESS: Requires 'table' parameter. Returns concrete structs mapping to the specified table. Example table: 'users'."),
		mcp.WithString("table", mcp.Required(), mcp.Description("The table or schema name to search for in struct tags (e.g., 'users', 'roles')")),
	)
	addTool(schemaTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		tbl, ok := args["table"].(string)
		if !ok || tbl == "" {
			return mcp.NewToolResultError("missing 'table' argument"), nil
		}
		results := search.Schema(g, tbl)
		return formatResults(results), nil
	})

	// Tool: gograph_globals
	globalsTool := mcp.NewTool("gograph_globals",
		mcp.WithDescription("Audit all global variable declarations, shared package states, and constants defined inside the repository. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to discover mutable global state, check config values, or identify thread-safety hazards. Do NOT use for local scope variables. COMPLETENESS: Returns a structured checklist of all global variables, constants, package paths, and types."),
		mcp.WithString("package", mcp.Required(), mcp.Description("The package name or path to inspect (e.g., 'internal/config')")),
	)
	addTool(globalsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		pkg, ok := args["package"].(string)
		if !ok || pkg == "" {
			return mcp.NewToolResultError("missing 'package' argument"), nil
		}
		results := search.Globals(g, pkg)
		return formatResults(results), nil
	})

	// Tool: gograph_mocks
	mocksTool := mcp.NewTool("gograph_mocks",
		mcp.WithDescription("Identify and locate all test mocks, double definitions, and test-only helper structs. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to identify active mock wrappers or find test fixtures. Do NOT use for production interface implementers (use gograph_implementers instead). COMPLETENESS: Requires 'interface' parameter. Returns test/mock concrete structs implementing the specified interface. Example interface: 'error'."),
		mcp.WithString("interface", mcp.Required(), mcp.Description("The name of the interface (e.g., 'AuthService')")),
	)
	addTool(mocksTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		iface, ok := args["interface"].(string)
		if !ok || iface == "" {
			return mcp.NewToolResultError("missing 'interface' argument"), nil
		}
		results := search.Mocks(g, iface)
		return formatResults(results), nil
	})

	// Tool: gograph_explain
	explainTool := mcp.NewTool("gograph_explain",
		mcp.WithDescription("Generate high-fidelity explanations, downstream impacts, and documentation summaries for codebase symbols. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to construct technical documentation, onboard to complex implementations, or summarize logic for PRs. Do NOT use for line-by-line raw source code (use gograph_source instead). COMPLETENESS: Requires 'symbol' parameter. Returns a detailed semantic summary of the symbol's role, dependencies, and calls. Example symbol: 'BuildGraph'."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("The name or ID of the symbol to explain (e.g., 'CreateUser' or 'Graph')")),
	)
	addTool(explainTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		sym, ok := args["symbol"].(string)
		if !ok || sym == "" {
			return mcp.NewToolResultError("symbol must be a non-empty string"), nil
		}
		result := search.Explain(g, sym)
		if result == nil {
			return mcp.NewToolResultText(fmt.Sprintf(`{"symbol":"%s","found":false}`, sym)), nil
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_node
	nodeTool := mcp.NewTool("gograph_node",
		mcp.WithDescription("Extract full AST property details for any specific symbol, package, or file node in the repository graph. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when you need deep structural metadata including kinds, signatures, documentation, or exact file lines. Do NOT use if you need raw source code only (use gograph_source instead). COMPLETENESS: Requires 'name' parameter. Returns complete node properties and fields. Example name: 'Graph'."),
		mcp.WithString("name", mcp.Required(), mcp.Description("The exact symbol, package path, or Go file name to inspect (e.g., 'Graph', 'internal/search', 'server.go')")),
	)
	addTool(nodeTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		name, ok := args["name"].(string)
		if !ok || name == "" {
			return mcp.NewToolResultError("name must be a non-empty string"), nil
		}
		results := search.Node(g, name)
		return formatResults(results), nil
	})

	// Tool: gograph_envs
	envsTool := mcp.NewTool("gograph_envs",
		mcp.WithDescription("Extract and audit all environment variables (os.Getenv, os.LookupEnv) referenced across the codebase. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when compiling configuration profiles, verifying env parameter bindings, or auditing configuration management. Do NOT use for magic constant literals (use gograph_literals instead). COMPLETENESS: Returns a structured list of env keys, their default fallbacks, and the file and line locations of the call."),
		mcp.WithString("term", mcp.Description("Optional filter term (e.g., 'DATABASE' matches DATABASE_URL, DATABASE_HOST, etc.)")),
	)
	addTool(envsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		term := ""
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if t, ok := args["term"].(string); ok {
				term = t
			}
		}
		results := search.Envs(g, term)
		return formatResults(results), nil
	})

	// Tool: gograph_interfaces
	interfacesTool := mcp.NewTool("gograph_interfaces",
		mcp.WithDescription("Audit and list all Go interface declarations, complete with their defined method signatures. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when mapping API abstractions, checking interface contracts, or reviewing modular design patterns. Do NOT use to find implementers (use gograph_implementers instead). COMPLETENESS: Returns all interface names, method signatures, package paths, and file locations."),
		mcp.WithString("struct", mcp.Required(), mcp.Description("The name of the struct (e.g., 'AuthService')")),
	)
	addTool(interfacesTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		structName, ok := args["struct"].(string)
		if !ok || structName == "" {
			return mcp.NewToolResultError("struct must be a non-empty string"), nil
		}
		results := search.Interfaces(g, structName)
		return formatResults(results), nil
	})

	// Tool: gograph_tests
	testsTool := mcp.NewTool("gograph_tests",
		mcp.WithDescription("Find all unit tests, benchmark functions, and testing suites declared in Go test files. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to identify existing test coverage or locate specific verification flows before running tests. Do NOT use for production code symbols (use gograph_node instead). COMPLETENESS: Returns test functions, target packages, and their file definitions."),
		mcp.WithString("symbol", mcp.Description("The symbol name to find tests for (optional)")),
	)
	addTool(testsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		term := ""
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if s, ok := args["symbol"].(string); ok {
				term = s
			}
		}
		results := search.Tests(g, term)
		return formatResults(results), nil
	})

	// Tool: gograph_hotspot
	hotspotTool := mcp.NewTool("gograph_hotspot",
		mcp.WithDescription("Locate code hotspots by combining structural call complexity and incoming dependency metrics. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when conducting performance audits, locating architectural coupling hotspots, or planning structural simplification. Do NOT use for single-package metrics (use gograph_focus instead). COMPLETENESS: Returns a prioritized list of hotspot packages, complexity counts, and caller metrics."),
		mcp.WithNumber("top", mcp.Description("Number of results to return (default: 10, 0 = all)")),
		mcp.WithBoolean("include_tests", mcp.Description("Include call edges from *_test.go files. Default false — production fan-in only, otherwise test helpers (baseReq, newTestFoo, etc.) tend to dominate rankings in test-heavy codebases.")),
	)
	addTool(hotspotTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		top := 10
		includeTests := false
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if n, ok := args["top"].(float64); ok {
				top = int(n)
			}
			if b, ok := args["include_tests"].(bool); ok {
				includeTests = b
			}
		}
		results := search.Hotspot(g, top, includeTests)
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_deps
	depsTool := mcp.NewTool("gograph_deps",
		mcp.WithDescription("Render the package import dependency tree for a target Go package. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when mapping package interactions, auditing layer separation, or analyzing transitive import paths. Do NOT use for tracing raw usages of a symbol (use gograph_usages instead). COMPLETENESS: Requires 'package' parameter. Returns a structured package list showing direct or full transitive dependency chains. Example package: 'internal/search'."),
		mcp.WithString("package", mcp.Required(), mcp.Description("The target package path or name to inspect (e.g., 'internal/search', 'internal/cli')")),
		mcp.WithBoolean("transitive", mcp.Description("If true, return the full transitive import closure via Breadth-First Search (BFS)")),
	)
	addTool(depsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		pkg, ok := args["package"].(string)
		if !ok || pkg == "" {
			return mcp.NewToolResultError("package must be a non-empty string"), nil
		}
		transitive, _ := args["transitive"].(bool)
		result := search.Deps(g, pkg, transitive)
		if result == nil {
			return mcp.NewToolResultText(fmt.Sprintf(`{"package":%q,"found":false}`, pkg)), nil
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_changes
	changesTool := mcp.NewTool("gograph_changes",
		mcp.WithDescription("Analyze structural changes and modifications in Go files compared to a specific git commit reference. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool during code reviews, pull request verification, or post-edit sessions to track structural diffs. Do NOT use for line diffs (use standard git diff instead). COMPLETENESS: Requires 'since' parameter. Returns changed symbol names, kinds, and paths. Example since: 'HEAD~1'."),
		mcp.WithString("git_ref", mcp.Description("Optional git reference to compare against (e.g., 'main', 'HEAD~5', 'v1.4.50')")),
	)
	addTool(changesTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		root, _ := filepath.Abs(".")
		gitRef := ""
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if r, ok := args["git_ref"].(string); ok {
				gitRef = r
			}
		}
		if gitRef != "" {
			result, err := search.ChangesByGitRef(g, root, gitRef)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		}
		result := search.Changes(g, root)
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_path
	pathTool := mcp.NewTool("gograph_path",
		mcp.WithDescription("Trace and find the direct or transitive call paths connecting a starting source symbol to a target destination symbol. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when tracing synchronous control flow, debugging deadlocks, or mapping execution paths. Do NOT use if you only need direct callers (use gograph_callers instead). COMPLETENESS: Requires 'from' and 'to' parameters. Returns call chains connecting the two symbols. Example from: 'Serve', example to: 'BuildGraph'."),
		mcp.WithString("from", mcp.Required(), mcp.Description("The starting symbol name")),
		mcp.WithString("to", mcp.Required(), mcp.Description("The target symbol name")),
	)
	addTool(pathTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		from, ok := args["from"].(string)
		if !ok || from == "" {
			return mcp.NewToolResultError("from must be a non-empty string"), nil
		}
		to, ok := args["to"].(string)
		if !ok || to == "" {
			return mcp.NewToolResultError("to must be a non-empty string"), nil
		}
		chain := search.Path(g, from, to, true)
		if len(chain) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf(`{"from":%q,"to":%q,"found":false}`, from, to)), nil
		}
		data, err := json.MarshalIndent(map[string]any{
			"from":  from,
			"to":    to,
			"found": true,
			"steps": chain,
		}, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_stale
	staleTool := mcp.NewTool("gograph_stale",
		mcp.WithDescription("List all Go package dependencies that are currently stale and require rebuilding. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when verifying incremental build status, caching layers, or validating compilation health. Do NOT use if package dependencies are up to date. COMPLETENESS: Returns a list of stale package paths."),
	)
	addTool(staleTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		absRoot, _ := filepath.Abs(".")
		sr := search.Stale(g, absRoot)
		data, err := json.MarshalIndent(sr, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_complexity
	complexityTool := mcp.NewTool("gograph_complexity",
		mcp.WithDescription("Estimate and report cyclomatic complexity metrics for Go functions, sorted from highest to lowest. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when auditing code quality, identifying high-maintenance functions, or planning refactoring of complex logic. Do NOT use for import dependency metrics (use gograph_deps instead). COMPLETENESS: Optional 'symbol' filter. Returns a structured complexity report indicating complexity numbers and severity labels. Example symbol: 'Build'."),
		mcp.WithString("symbol", mcp.Description("Optional Go function or method symbol name substring to filter the complexity report (e.g., 'Build')")),
	)
	addTool(complexityTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		term := ""
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if s, ok := args["symbol"].(string); ok {
				term = s
			}
		}
		results := search.Complexity(g, term)
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_coupling
	couplingTool := mcp.NewTool("gograph_coupling",
		mcp.WithDescription("Assess structural coupling and dependency ratios (Afferent vs Efferent coupling) for Go packages. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to evaluate package isolation, modularity strength, and stability indexes. Do NOT use for single function analysis (use gograph_complexity instead). COMPLETENESS: Returns afferent coupling (Ca), efferent coupling (Ce), and instability ratio (I) per package."),
		mcp.WithString("package", mcp.Description("Optional package name substring to filter results")),
		mcp.WithBoolean("include_stdlib", mcp.Description("Include standard-library packages in the report. Default false — users asking 'how coupled is my code?' rarely care about stdlib coupling.")),
		mcp.WithBoolean("internal_only", mcp.Description("Restrict the report to the project's own packages (anything starting with the module path from go.mod). Strictly stronger than excluding stdlib — also excludes third-party deps.")),
	)
	addTool(couplingTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		term := ""
		opts := search.CouplingOptions{}
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if p, ok := args["package"].(string); ok {
				term = p
			}
			if b, ok := args["include_stdlib"].(bool); ok {
				opts.IncludeStdlib = b
			}
			if b, ok := args["internal_only"].(bool); ok && b {
				// CLI reads go.mod from cwd; MCP server runs server-side
				// so cwd is whichever directory the user invoked gograph
				// from. Re-read here for consistency.
				if mod := search.ReadModulePath("."); mod != "" {
					opts.ModuleOnly = mod
				}
			}
		}
		results := search.Coupling(g, term, opts)
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_arity
	arityTool := mcp.NewTool("gograph_arity",
		mcp.WithDescription("Identify functions or methods that exceed a specified number of parameters (arity violation). BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to audit design parameters or identify functions that should be refactored into a parameter struct. Do NOT use for struct fields (use gograph_fields instead). COMPLETENESS: Optional 'threshold' value (defaults to 5). Returns a list of functions exceeding the target parameter count."),
		mcp.WithNumber("min", mcp.Description("Minimum argument count to report (default: 5)")),
	)
	addTool(arityTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		minArgs := 5
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if n, ok := args["min"].(float64); ok && n > 0 {
				minArgs = int(n)
			}
		}
		results := search.Arity(g, minArgs)
		return formatResults(results), nil
	})

	// Tool: gograph_concurrency
	concurrencyTool := mcp.NewTool("gograph_concurrency",
		mcp.WithDescription("Scan the codebase to locate all concurrency declarations (go channels, select statements, sync primitives, go routines). BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to trace asynchronous routines, verify race safety, or audit synchronization primitives. Do NOT use for standard linear execution flows. COMPLETENESS: Returns file locations, line coordinates, and types of all concurrency primitives detected."),
		mcp.WithString("term", mcp.Description("Optional filter term (e.g., 'goroutine', 'mutex', 'channel')")),
	)
	addTool(concurrencyTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		term := ""
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if t, ok := args["term"].(string); ok {
				term = t
			}
		}
		results := search.Concurrency(g, term)
		return formatResults(results), nil
	})

	// Tool: gograph_fixtures
	fixturesTool := mcp.NewTool("gograph_fixtures",
		mcp.WithDescription("Locate and audit external data files, JSON payloads, or database seeds used inside test suites. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to trace test dependencies or catalog test datasets. Do NOT use for active test functions (use gograph_tests instead). COMPLETENESS: Returns a structured checklist of all referenced testing datasets and their file paths."),
		mcp.WithString("package", mcp.Required(), mcp.Description("The package path or name (e.g., 'internal/auth')")),
	)
	addTool(fixturesTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		pkg, ok := args["package"].(string)
		if !ok || pkg == "" {
			return mcp.NewToolResultError("package must be a non-empty string"), nil
		}
		results := search.Fixtures(g, pkg)
		return formatResults(results), nil
	})

	// Tool: gograph_godobj
	godobjTool := mcp.NewTool("gograph_godobj",
		mcp.WithDescription("Detect God Object code smells by analyzing structural sizes, method counts, and references. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when auditing architectural modularity or identifying monolithic structs that should be refactored. Do NOT use for general struct layout checks (use gograph_fields instead). COMPLETENESS: Returns concrete structs exceeding target field or method count limits."),
		mcp.WithNumber("methods", mcp.Description("Minimum method count (default: 5)")),
		mcp.WithNumber("fields", mcp.Description("Minimum field count (default: 8)")),
		mcp.WithNumber("calls", mcp.Description("Minimum outgoing call count (default: 15)")),
		mcp.WithNumber("top", mcp.Description("Maximum results to return (default: 10)")),
	)
	addTool(godobjTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		p := search.DefaultGodObjectParams()
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			if n, ok := args["methods"].(float64); ok && n > 0 {
				p.MinMethods = int(n)
			}
			if n, ok := args["fields"].(float64); ok && n > 0 {
				p.MinFields = int(n)
			}
			if n, ok := args["calls"].(float64); ok && n > 0 {
				p.MinCalls = int(n)
			}
			if n, ok := args["top"].(float64); ok && n > 0 {
				p.Top = int(n)
			}
		}
		candidates := search.GodObjects(g, p)
		data, err := json.MarshalIndent(candidates, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_skeleton
	skeletonTool := mcp.NewTool("gograph_skeleton",
		mcp.WithDescription("Generate high-level skeletal outline structural abstractions of codebase files, excluding implementation details. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when you need to understand the skeleton structure of files quickly without reading massive line counts. Do NOT use if you need full function implementations (use gograph_source instead). COMPLETENESS: Returns a concise abstract list of struct schemas, function definitions, and interfaces."),
	)
	addTool(skeletonTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		return mcp.NewToolResultText(search.Skeleton(g)), nil
	})

	// Tool: gograph_returnusage
	returnusageTool := mcp.NewTool("gograph_returnusage",
		mcp.WithDescription("Verify the patterns and coverage of return parameter usages and validation handling. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to check if returned parameters are properly inspected, validated, or captured after method invocations. Do NOT use for error flow bubbling (use gograph_errorflow instead). COMPLETENESS: Returns matched call locations and variable binding status."),
		mcp.WithString("function", mcp.Required(), mcp.Description("The function name to analyse (e.g., 'ValidateToken')")),
	)
	addTool(returnusageTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		fn, ok := args["function"].(string)
		if !ok || fn == "" {
			return mcp.NewToolResultError("function must be a non-empty string"), nil
		}
		results := search.ReturnUsages(g, fn)
		return formatResults(results), nil
	})

	// Tool: gograph_mutate
	mutateTool := mcp.NewTool("gograph_mutate",
		mcp.WithDescription("Trace and audit all state-mutation and assignment points where a specific struct field gets modified. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool when diagnosing state bugs, checking data mutability, or analyzing struct field lifetimes. Do NOT use for direct struct layouts (use gograph_fields instead). COMPLETENESS: Requires 'struct' and 'field' parameters. Returns file and line locations where the target field is assigned or updated. Example struct: 'Symbol', example field: 'Name'."),
		mcp.WithString("field", mcp.Required(), mcp.Description("The field name to search for mutations (e.g., 'Status')")),
	)
	addTool(mutateTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		field, ok := args["field"].(string)
		if !ok || field == "" {
			return mcp.NewToolResultError("field must be a non-empty string"), nil
		}
		results := search.Mutate(g, field)
		return formatResults(results), nil
	})

	// Tool: gograph_stats
	statsTool := mcp.NewTool("gograph_stats",
		mcp.WithDescription("Generate comprehensive repository-level code statistics including total package counts, symbol frequencies, and imports. BEHAVIOR & SAFETY: This is a 100% local, read-only static analysis tool. It has no side effects, requires no authorization or credentials, has no rate limits, and performs zero destructive modifications. USAGE GUIDELINES: Call this tool to build high-level code density reports, trace total codebase sizes, or verify initial repository parsing health. Do NOT use for single symbol profiling (use gograph_node instead). COMPLETENESS: Returns total counts of files, lines, packages, structs, functions, and interfaces."),
	)
	addTool(statsTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		st := search.Stats(g)
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}
