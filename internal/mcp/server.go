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
		"1.4.44",
		server.WithToolCapabilities(true),
	)

	addTool := func(tool mcp.Tool, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
		s.AddTool(tool, handler)
		if ExposeToolsForTesting != nil {
			ExposeToolsForTesting[tool.Name] = handler
		}
	}

	// Tool: gograph_capabilities
	capabilitiesTool := mcp.NewTool("gograph_capabilities",
		mcp.WithDescription("Discover the available gograph MCP tools, their purposes, recommended workflows, and limitations. Call this tool first to understand what gograph can do."),
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
				{"name": "gograph_implementers", "purpose": "Find all structs that implement the specified interface."},
				{"name": "gograph_fields", "purpose": "Extract all fields from a specific struct, including their types and struct tags."},
				{"name": "gograph_source", "purpose": "Extract the exact source code for a specific function, method, struct, or interface."},
				{"name": "gograph_orphans", "purpose": "Find functions and methods that have 0 explicit incoming calls (potential dead code)."},
				{"name": "gograph_impact", "purpose": "Traverse the call graph backwards to find all symbols that eventually call the target symbol."},
				{"name": "gograph_boundaries", "purpose": "Verify package architecture constraints against boundaries.json."},
				{"name": "gograph_endpoint", "purpose": "Full vertical slice for one HTTP endpoint: route, handler, downstream call chain (BFS), SQL emitted, env reads."},
				{"name": "gograph_api", "purpose": "Compare the public-facing contract and integration surface drift against a baseline git reference."},
				{"name": "gograph_routes", "purpose": "Extract all HTTP REST API routes found in the codebase."},
				{"name": "gograph_context", "purpose": "Bundles node details, callers, callees, tests, and source code into one structured response."},
				{"name": "gograph_plan", "purpose": "Safe edit planning before code changes. Highlights likely affected tests, routes, env reads, SQL touches, and public API impact."},
				{"name": "gograph_review", "purpose": "Post-edit or symbol-focused review. Summarizes what changed and its risk profile."},
				{"name": "gograph_errorflow", "purpose": "Trace likely error paths up to entry points (HTTP routes or CLI commands)."},
				{"name": "gograph_imports", "purpose": "Find all files that import a specific external package."},
				{"name": "gograph_sql", "purpose": "Extract database SQL queries found in the codebase."},
				{"name": "gograph_errors", "purpose": "Extract custom error messages and panics."},
				{"name": "gograph_embeds", "purpose": "Find what structs embed the given target struct."},
				{"name": "gograph_public", "purpose": "Show only the exported (public) symbols of a specific package."},
				{"name": "gograph_constructors", "purpose": "Find factory functions returning the named struct."},
				{"name": "gograph_schema", "purpose": "Find structs mapped to a database table or schema via struct tags."},
				{"name": "gograph_globals", "purpose": "Find package-level variables and functions mutating them."},
				{"name": "gograph_mocks", "purpose": "Find structs implementing an interface, filtered to test or mock files."},
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
		mcp.WithDescription("Search the Go repository for symbols, packages, files, or imports using a keyword term."),
		mcp.WithString("term", mcp.Required(), mcp.Description("The keyword to search for (e.g., 'Auth')")),
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
		mcp.WithDescription("Extract highly targeted context for a single Go package, including all files, symbols, internal calls, and dependencies associated with it."),
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
		mcp.WithDescription("Find what functions or methods call the specified function. Useful for impact analysis."),
		mcp.WithString("function", mcp.Required(), mcp.Description("The name of the function being called (e.g., 'ValidateToken')")),
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
		mcp.WithDescription("Find what functions or methods are called from inside the specified function. Useful to understand dependencies."),
		mcp.WithString("function", mcp.Required(), mcp.Description("The name of the calling function (e.g., 'InitServer')")),
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
		mcp.WithDescription("Find all structs that implement the specified interface. Essential for understanding implicit Go interfaces and dependency injection."),
		mcp.WithString("interface", mcp.Required(), mcp.Description("The name of the interface (e.g., 'AuthService')")),
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
		results := search.Implementers(g, iface)
		return formatResults(results), nil
	})

	// Tool: gograph_fields
	fieldsTool := mcp.NewTool("gograph_fields",
		mcp.WithDescription("Extract all fields from a specific struct, including their types and struct tags. Useful for understanding data models."),
		mcp.WithString("struct", mcp.Required(), mcp.Description("The name of the struct (e.g., 'User')")),
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
		mcp.WithDescription("Extract the exact source code for a specific function, method, struct, or interface. Extremely efficient for reading implementation logic without reading the entire file."),
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
		mcp.WithDescription("Find functions and methods that have 0 explicit incoming calls (potential dead code)."),
	)
	addTool(orphansTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		results := search.Orphans(g)
		return formatResults(results), nil
	})

	// Tool: gograph_impact
	impactTool := mcp.NewTool("gograph_impact",
		mcp.WithDescription("Traverse the call graph backwards to find all symbols that eventually call the target symbol. Useful for blast radius analysis."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("The name of the symbol (e.g., 'ValidateToken')")),
	)
	addTool(impactTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		results := search.Impact(g, sym, true)
		return formatResults(results), nil
	})

	// Tool: gograph_boundaries
	boundariesTool := mcp.NewTool("gograph_boundaries",
		mcp.WithDescription("Verify package architecture constraints against boundaries.json to detect forbidden imports between layers."),
		mcp.WithString("config", mcp.Description("Optional path to configuration file (defaults to .gograph/boundaries.json)")),
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
		mcp.WithDescription("Full vertical slice for one HTTP endpoint. Resolves a route pattern or handler symbol to its handler, then traces the downstream call chain (BFS, default depth 5), collects SQL queries emitted, and env vars read. Heuristic AST call-graph — calls through interfaces may not appear."),
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
		slices := search.Endpoint(g, query, depth)
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
		mcp.WithDescription("Compare the public-facing contract and integration surface of the Go codebase against a baseline git reference. Identifies likely breaking changes to exported functions, structs, interfaces, and routes."),
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
				cmd.Process.Kill()
				cmd.Wait()
				return mcp.NewToolResultError(fmt.Sprintf("tar read error: %v", err)), nil
			}

			target := filepath.Join(tmpDir, header.Name)
			// Check for zip slip
			if !strings.HasPrefix(target, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
				continue
			}

			switch header.Typeflag {
			case tar.TypeDir:
				os.MkdirAll(target, os.FileMode(header.Mode))
			case tar.TypeReg:
				os.MkdirAll(filepath.Dir(target), 0755)
				f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
				if err == nil {
					io.Copy(f, tr)
					f.Close()
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
		mcp.WithDescription("Extract all HTTP REST API routes found in the codebase (e.g. GET /api)."),
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
		mcp.WithDescription("Compact symbol context. Bundles node details, callers, callees, tests, and source code into one structured response."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("The exact name or ID of the symbol to retrieve context for.")),
	)
	addTool(contextTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		symbol, ok := args["symbol"].(string)
		if !ok || symbol == "" {
			return mcp.NewToolResultError("symbol must be a non-empty string"), nil
		}
		root, _ := filepath.Abs(".")
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
			Risk:    map[string]any{},
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
		mcp.WithDescription("Safe edit planning before code changes. Highlights likely affected tests, routes, env reads, SQL touches, and public API impact."),
		mcp.WithString("symbol", mcp.Description("The symbol to plan changes for")),
		mcp.WithBoolean("uncommitted", mcp.Description("Set to true to plan all uncommitted changes")),
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

		resp := MCPResponse{
			Summary:      "Change plan for " + planRes.Title,
			InspectFirst: planRes.ReadFirst,
			Tests:        planRes.Tests,
			Routes:       planRes.Routes,
			Env:          planRes.Envs,
			Risk: map[string]any{
				"public_api":     planRes.PublicAPI,
				"touches_sql":    planRes.TouchesSQL,
				"touches_routes": len(planRes.Routes) > 0,
				"touches_env":    len(planRes.Envs) > 0,
			},
		}
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	// Tool: gograph_review
	reviewTool := mcp.NewTool("gograph_review",
		mcp.WithDescription("Post-edit or symbol-focused review. Summarizes what changed and its risk profile."),
		mcp.WithString("symbol", mcp.Description("The symbol to review")),
		mcp.WithBoolean("uncommitted", mcp.Description("Set to true to review all uncommitted changes")),
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
		mcp.WithDescription("Trace likely error paths up to entry points (HTTP routes or CLI commands). Use this to find where an error originates and how it is handled. (AST heuristic, NO SSA)"),
		mcp.WithString("term", mcp.Description("The error string or sentinel error name (e.g., 'ErrInvalidToken' or 'invalid token')")),
		mcp.WithString("query", mcp.Description("The error string or sentinel error name (preferred over term)")),
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

		report := search.ErrorFlow(g, term)

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
		mcp.WithDescription("Find all files that import a specific external package. Useful to trace where third-party libraries are used."),
		mcp.WithString("package", mcp.Required(), mcp.Description("The name of the package (e.g., 'github.com/redis/go-redis')")),
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

	// Tool: gograph_sql
	sqlTool := mcp.NewTool("gograph_sql",
		mcp.WithDescription("Extract database SQL queries found in the codebase. You can optionally filter by term."),
		mcp.WithString("term", mcp.Description("Optional string to filter the queries")),
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
		mcp.WithDescription("Extract custom error messages and panics. You can optionally filter by a string."),
		mcp.WithString("term", mcp.Description("Optional string to filter the errors")),
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
		mcp.WithDescription("Find what structs embed the given target struct."),
		mcp.WithString("struct", mcp.Required(), mcp.Description("The name of the target struct (e.g., 'Mutex')")),
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
		mcp.WithDescription("Show only the exported (public) symbols of a specific package."),
		mcp.WithString("package", mcp.Required(), mcp.Description("The package name (e.g., 'auth')")),
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
	// Tool: gograph_constructors
	constructorsTool := mcp.NewTool("gograph_constructors",
		mcp.WithDescription("Find factory functions returning the named struct."),
		mcp.WithString("struct", mcp.Required(), mcp.Description("The name of the struct (e.g., 'User')")),
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
		mcp.WithDescription("Find structs mapped to a database table or schema via struct tags."),
		mcp.WithString("table", mcp.Required(), mcp.Description("The table or schema name (e.g., 'users')")),
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
		mcp.WithDescription("Find package-level variables and functions mutating them."),
		mcp.WithString("package", mcp.Required(), mcp.Description("The package name (e.g., 'internal/config')")),
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
		mcp.WithDescription("Find structs implementing an interface, filtered to test or mock files."),
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
}
