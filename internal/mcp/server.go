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
func Serve(g *graph.Graph) error {
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
	s.AddTool(queryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(focusTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(callersTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		fn, ok := args["function"].(string)
		if !ok {
			return mcp.NewToolResultError("function must be a string"), nil
		}
		results := search.Callers(g, fn)
		return formatResults(results), nil
	})

	// Tool: gograph_callees
	calleesTool := mcp.NewTool("gograph_callees",
		mcp.WithDescription("Find what functions or methods are called from inside the specified function. Useful to understand dependencies."),
		mcp.WithString("function", mcp.Required(), mcp.Description("The name of the calling function (e.g., 'InitServer')")),
	)
	s.AddTool(calleesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		fn, ok := args["function"].(string)
		if !ok {
			return mcp.NewToolResultError("function must be a string"), nil
		}
		results := search.Callees(g, fn)
		return formatResults(results), nil
	})

	// Tool: gograph_implementers
	implementersTool := mcp.NewTool("gograph_implementers",
		mcp.WithDescription("Find all structs that implement the specified interface. Essential for understanding implicit Go interfaces and dependency injection."),
		mcp.WithString("interface", mcp.Required(), mcp.Description("The name of the interface (e.g., 'AuthService')")),
	)
	s.AddTool(implementersTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	s.AddTool(sourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		results := search.Orphans(g)
		return formatResults(results), nil
	})

	// Tool: gograph_impact
	impactTool := mcp.NewTool("gograph_impact",
		mcp.WithDescription("Traverse the call graph backwards to find all symbols that eventually call the target symbol. Useful for blast radius analysis."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("The name of the symbol (e.g., 'ValidateToken')")),
	)
	s.AddTool(impactTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		sym, ok := args["symbol"].(string)
		if !ok {
			return mcp.NewToolResultError("symbol must be a string"), nil
		}
		results := search.Impact(g, sym)
		return formatResults(results), nil
	})

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
