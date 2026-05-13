package mcp

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

// Serve runs the gograph MCP server over stdio.
func Serve(g *graph.Graph, rebuild func() (*graph.Graph, error)) error {
	s := server.NewMCPServer(
		"gograph",
		"1.1.0",
		server.WithToolCapabilities(true),
	)

	// Tool: gograph_query
	queryTool := mcp.NewTool("gograph_query",
		mcp.WithDescription("Search the Go repository for symbols, packages, files, or imports using a keyword term."),
		mcp.WithString("term", mcp.Required(), mcp.Description("The keyword to search for (e.g., 'Auth')")),
	)
	s.AddTool(queryTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(focusTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(callersTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(calleesTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(implementersTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(fieldsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(sourceTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(orphansTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(impactTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(boundariesTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		
		if len(results) == 0 {
			return mcp.NewToolResultText("No boundary violations found. Architecture is clean!"), nil
		}
		return formatResults(results), nil
	})

	// Tool: gograph_plan
	planTool := mcp.NewTool("gograph_plan",
		mcp.WithDescription("Generate an operational change plan for a symbol before editing it. This aggregates callers, tests, and risk metrics (routes, sql, env) into a single actionable report."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("The name of the symbol you intend to edit (e.g., 'ValidateToken')")),
	)
	s.AddTool(planTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		
		planText := search.Plan(g, []string{sym}, sym)
		return mcp.NewToolResultText(planText), nil
	})

	// Tool: gograph_review
	reviewTool := mcp.NewTool("gograph_review",
		mcp.WithDescription("Generate a post-edit final review report for a modified symbol. Use this after making changes to verify test coverage, complexity, and downstream execution risks (SQL, Envs, etc)."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("The name of the symbol you just edited (e.g., 'ValidateToken')")),
	)
	s.AddTool(reviewTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		
		reviewText := search.Review(g, []string{sym}, sym)
		return mcp.NewToolResultText(reviewText), nil
	})

	// Tool: gograph_routes
	routesTool := mcp.NewTool("gograph_routes",
		mcp.WithDescription("Extract all HTTP REST API routes found in the codebase (e.g. GET /api)."),
	)
	s.AddTool(routesTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if newG, err := rebuild(); err == nil {
			g = newG
		}
		results := search.Routes(g)
		return formatResults(results), nil
	})

	// Tool: gograph_errorflow
	errorflowTool := mcp.NewTool("gograph_errorflow",
		mcp.WithDescription("Trace likely error paths up to entry points (HTTP routes or CLI commands). Use this to find where an error originates and how it is handled. (AST heuristic, NO SSA)"),
		mcp.WithString("term", mcp.Required(), mcp.Description("The error string or sentinel error name (e.g., 'ErrInvalidToken' or 'invalid token')")),
	)
	s.AddTool(errorflowTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		
		report := search.ErrorFlow(g, term)
		return mcp.NewToolResultText(report.String()), nil
	})

	// Tool: gograph_imports
	importsTool := mcp.NewTool("gograph_imports",
		mcp.WithDescription("Find all files that import a specific external package. Useful to trace where third-party libraries are used."),
		mcp.WithString("package", mcp.Required(), mcp.Description("The name of the package (e.g., 'github.com/redis/go-redis')")),
	)
	s.AddTool(importsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(sqlTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(errorsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(embedsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(publicTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	initNewTools(s, g, rebuild)

	// Start stdio server
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

func initNewTools(s *server.MCPServer, g *graph.Graph, rebuild func() (*graph.Graph, error)) {
	// Tool: gograph_constructors
	constructorsTool := mcp.NewTool("gograph_constructors",
		mcp.WithDescription("Find factory functions returning the named struct."),
		mcp.WithString("struct", mcp.Required(), mcp.Description("The name of the struct (e.g., 'User')")),
	)
	s.AddTool(constructorsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(schemaTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(globalsTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(mocksTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
