// Package cli wires together the CLI commands.
package cli

import (
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/mcp"
	"github.com/ozgurcd/gograph/internal/parser"
	"github.com/ozgurcd/gograph/internal/precise"
	"github.com/ozgurcd/gograph/internal/report"
	"github.com/ozgurcd/gograph/internal/scanner"
	"github.com/ozgurcd/gograph/internal/search"
)

const outputDir = ".gograph"
const graphFile = ".gograph/graph.json"
const reportFile = ".gograph/GRAPH_REPORT.md"
const symFile = ".gograph/graph-symbols.md"
const depsFile = ".gograph/graph-deps.md"
const routesFile = ".gograph/graph-routes.md"
const sqlFile = ".gograph/graph-sql.md"
const errorsFile = ".gograph/graph-errors.md"
const configFile = ".gograph/graph-config.md"
const concFile = ".gograph/graph-concurrency.md"
const testsFile = ".gograph/graph-tests.md"

// Version is set at build time via -ldflags; defaults to "dev" for local builds.
var Version = "dev"

// Run is the entrypoint called from main.
func Run(args []string) int {
	if len(args) == 0 {
		printHelp()
		return 0
	}

	// Strip --json and --files-only from args before dispatch; set the package-level flag.
	jsonMode = false
	filesOnlyMode = false
	filtered := args[:0]
	for _, a := range args {
		switch a {
		case "--help", "-h":
			if len(args) > 1 && args[0] != "--help" && args[0] != "-h" {
				printCommandHelp(args[0])
			} else {
				printHelp()
			}
			return 0
		case "--json":
			jsonMode = true
		case "--files-only":
			filesOnlyMode = true
		default:
			filtered = append(filtered, a)
		}
	}
	args = filtered

	switch args[0] {
	case "build":
		return runBuild(args[1:])
	case "query":
		return runQuery(args[1:])
	case "focus":
		return runFocus(args[1:])
	case "node":
		return runNode(args[1:])
	case "source":
		return runSource(args[1:])
	case "public":
		return runPublic(args[1:])
	case "fields":
		return runFields(args[1:])
	case "embeds":
		return runEmbeds(args[1:])
	case "imports":
		return runImports(args[1:])
	case "callers":
		return runCallers(args[1:])
	case "callees":
		return runCallees(args[1:])
	case "impact":
		return runImpact(args[1:])
	case "implementers":
		return runImplementers(args[1:])
	case "envs":
		return runEnvs(args[1:])
	case "interfaces":
		return runInterfaces(args[1:])
	case "concurrency":
		return runConcurrency(args[1:])
	case "tests":
		return runTests(args[1:])
	case "routes":
		return runRoutes()
	case "sql":
		return runSQL(args[1:])
	case "errors":
		return runErrors(args[1:])
	case "errorflow":
		return runErrorFlow(args[1:])
	case "path":
		return runPath(args[1:])
	case "stale":
		return runStale()
	case "orphans":
		return runOrphans()
	case "godobj":
		return runGodObj(args[1:])
	case "skeleton":
		return runSkeleton()
	case "mutate":
		return runMutate(args[1:])
	case "trace":
		return runTrace(args[1:])
	case "arity":
		return runArity(args[1:])
	case "complexity":
		return runComplexity(args[1:])
	case "coupling":
		return runCoupling(args[1:])
	case "context":
		return runContext(args[1:])
	case "hotspot":
		return runHotspot(args[1:])
	case "deps":
		return runDeps(args[1:])
	case "changes":
		return runChanges()
	case "capabilities":
		return runCapabilities()
	case "mcp":
		return runMCP(args[1:])
	case "constructors":
		return runConstructors(args[1:])
	case "schema":
		return runSchema(args[1:])
	case "globals":
		return runGlobals(args[1:])
	case "mocks":
		return runMocks(args[1:])
	case "fixtures":
		return runFixtures(args[1:])
	case "boundaries":
		return runBoundaries(args[1:])
	case "plan":
		return runPlan(args[1:])
	case "review":
		return runReview(args[1:])
	case "api", "contract":
		return runAPI(args[1:])
	case "check":
		return runCheck(args[1:])
	case "add-claude-plugin":
		if err := installPlugin(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to install plugin: %v\n", err)
			return 1
		}
		return 0
	case "hook-guard":
		return runHookGuard()
	case "help", "--help", "-h":
		printHelp()
		return 0
	case "version", "--version", "-v":
		fmt.Printf("gograph version v%s\n", Version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printHelp()
		return 1
	}
}

func runCapabilities() int {
	fmt.Println(`gograph: AST-aware Repository Navigation Tool for AI Agents

COMMANDS (token-optimized):
(Note: All search/navigation commands support --json and --files-only)
build . [--precise]  : parse AST, gen GRAPH_REPORT.md & .gograph/*

RULES FOR --precise:
- Default (build .): Use during active, messy development. It's lightning-fast, tolerates syntax/build errors, and uses AST heuristics (duck-typing) for interfaces.
- Precise (build . --precise): Use before a major refactor or when measuring blast radius (impact). Slower, but provides type-checked interface analysis and more precise call edges; requires compilable code.

AGENT WORKFLOW RULES (CRITICAL):
1. BEFORE editing code: ALWAYS run 'gograph plan <symbol>' to understand the impact, mapped tests, and execution risks (SQL/Env/Routes) of your target.
2. AFTER editing code: ALWAYS run 'gograph build . --precise' followed by 'gograph review --uncommitted' to verify test coverage, complexity, and that no unintended execution risks were introduced.

QUERY COMMANDS:
boundaries [--config] : verify package architecture constraints using boundaries.json
boundaries --create   : auto-generate a baseline boundaries.json from the current repo
callees <fn> [--no-tests]: what fn calls (returns exact call-site source snippet)
callers <fn> [--no-tests]: who calls fn (returns exact call-site source snippet)
complexity [sym]     : cyclomatic complexity estimate per function (highest first)
concurrency [str]    : goroutines/channels/mutexes
coupling [pkg]       : fan-in, fan-out, and instability per package
embeds <struct>      : structs embedding this struct
envs [str]           : os.Getenv/viper reads
errors [--no-tests]  : custom errors/panics (use --no-tests to exclude test files)
fields <struct>      : fields/types of struct
focus <pkg>          : isolate context for a package
godobj               : find god-object struct candidates (--methods N --fields N --calls N --top N)
impact <sym>         : blast radius (downstream callers)
impact --uncommitted : blast radius of all your uncommitted code changes
implementers <iface> : structs implementing iface
imports <pkg>        : trace external/internal usage
interfaces <struct>  : duck-type interface check
node <sym>           : AST info of sym
orphans              : functions with 0 explicit incoming calls (potential dead code)
path <from> <to>     : shortest call chain between two symbols (BFS)
public <pkg>         : exported API of pkg
query <str>          : search symbols/files/pkgs
routes               : HTTP REST routes
source <sym>         : exact code of sym (USE THIS instead of grep to read function bodies, mock stubs, or full interface definitions)
sql                  : raw SQL queries mapped
stale                : check if graph is out of date vs source files
tests <sym>          : tests exercising sym

TOKEN SAVERS (COMPOSED COMMANDS):
errorflow <term>     : trace likely error paths up to entry points (AST heuristic, NO SSA)
arity [--min 5]      : find functions with many arguments (long parameter list smell)
changes              : symbols modified/new/deleted since last build
constructors <struct>: factory functions returning struct
context <sym> [--limit N]: bundle node+source+callers+callees+tests (saves 4-5 tool calls)
deps <pkg> [--transitive] : import dependency tree of a package
fixtures <pkg>       : test helper structs and functions in test files
globals <pkg>        : pkg-level vars, consts, and mutators
hotspot [--top N]    : rank functions by incoming calls (study these first)
mocks <iface>        : structs implementing iface in test files
mutate <field>       : find functions that mutate a specific struct field
plan <sym>           : generate an operational change plan (read-first, tests, risk profile)
plan --uncommitted   : generate a change plan for all currently uncommitted modified symbols
review <sym>         : generate a post-edit final review report for a modified symbol
review --uncommitted : generate a post-edit final review report for all uncommitted changes
api --since <ref>    : identify breaking API and contract changes since a git reference
schema <table>       : structs mapped to DB table via tags
skeleton             : output the whole repository's API signatures (function bodies stripped)
trace <err_str> [--no-tests]: trace an error backwards from entry points to origin
check [--since ref]  : run static policy checks (boundaries, api_drift, test requirements)
mcp [path]           : start a Model Context Protocol server over stdio
add-claude-plugin    : install gograph as a Claude Desktop/Code MCP plugin (also injects CLAUDE.md rules and PreToolUse hook)
hook-guard           : PreToolUse hook — intercepts grep on Go symbols and redirects to gograph (invoked by Claude Code automatically)`)
	return 0
}

func runBuild(args []string) int {
	root := "."
	preciseMode := false
	var filteredArgs []string
	for _, a := range args {
		if a == "--precise" {
			preciseMode = true
		} else {
			filteredArgs = append(filteredArgs, a)
		}
	}
	if len(filteredArgs) > 0 {
		root = filteredArgs[0]
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving path: %v\n", err)
		return 1
	}

	fmt.Printf("gograph build: scanning %s\n", absRoot)

	g, err := BuildGraph(absRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building graph: %v\n", err)
		return 1
	}

	if preciseMode {
		fmt.Println("  running type-checked precision analysis (this may take a moment)...")
		// Delay import check by using precise.Enrich explicitly
		if err := precise.Enrich(absRoot, g); err != nil {
			fmt.Fprintf(os.Stderr, "warning: precise enrichment failed: %v\n", err)
		}
	}

	outDir := filepath.Join(absRoot, outputDir)
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output dir: %v\n", err)
		return 1
	}

	if err := writeGitignore(absRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	jsonPath := filepath.Join(absRoot, graphFile)
	if err := writeJSON(jsonPath, g); err != nil {
		fmt.Fprintf(os.Stderr, "error writing graph.json: %v\n", err)
		return 1
	}

	// Write all split markdown reports
	reports := map[string]string{
		reportFile: report.GenerateIndex(g),
		symFile:    report.GenerateSymbols(g),
		depsFile:   report.GenerateDeps(g),
		routesFile: report.GenerateRoutes(g),
		sqlFile:    report.GenerateSQL(g),
		errorsFile: report.GenerateErrors(g),
		configFile: report.GenerateConfig(g),
		concFile:   report.GenerateConcurrency(g),
		testsFile:  report.GenerateTests(g),
	}

	for relPath, content := range reports {
		fullPath := filepath.Join(absRoot, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0o640); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", relPath, err)
			return 1
		}
	}

	fmt.Printf("  packages: %d  files: %d  symbols: %d  calls: %d\n",
		len(g.Packages), len(g.Files), len(g.Symbols), len(g.Calls))
	fmt.Printf("  wrote %s\n", jsonPath)
	fmt.Printf("  wrote %d markdown reports to %s/\n", len(reports), outputDir)
	return 0
}

func BuildGraph(absRoot string) (*graph.Graph, error) {
	files, walkErrs := scanner.Walk(absRoot)
	for _, e := range walkErrs {
		fmt.Fprintf(os.Stderr, "  warning: %v\n", e)
	}
	fmt.Fprintf(os.Stderr, "  found %d Go files to parse\n", len(files))

	g := &graph.Graph{
		Version:     graph.Version,
		GeneratedAt: time.Now().UTC(),
		Root:        absRoot,
	}

	if deps, err := parseDependencies(absRoot); err == nil {
		g.Dependencies = deps
	}

	fset := token.NewFileSet()
	pkgMap := make(map[string]*graph.PackageNode)

	for _, path := range files {
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			rel = path
		}
		result, err := parser.ParseFile(fset, path, rel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
			continue
		}

		g.Files = append(g.Files, result.File)
		g.Symbols = append(g.Symbols, result.Symbols...)
		g.Imports = append(g.Imports, result.Imports...)
		g.Calls = append(g.Calls, result.Calls...)
		g.EnvReads = append(g.EnvReads, result.Env...)
		g.Routes = append(g.Routes, result.Routes...)
		g.SQLs = append(g.SQLs, result.SQLs...)
		g.Errors = append(g.Errors, result.Errors...)
		g.Concurrency = append(g.Concurrency, result.Concurrency...)
		g.TestEdges = append(g.TestEdges, result.TestEdges...)
		g.Mutations = append(g.Mutations, result.Mutations...)

		dir := filepath.Dir(rel)
		if _, ok := pkgMap[dir]; !ok {
			pkgMap[dir] = &graph.PackageNode{
				ID:                   dir,
				Name:                 result.File.PackageName,
				ImportPathBestEffort: bestEffortImportPath(absRoot, dir),
				Dir:                  dir,
			}
		}
		pkgMap[dir].Files = append(pkgMap[dir].Files, rel)
	}

	pkgKeys := make([]string, 0, len(pkgMap))
	for k := range pkgMap {
		pkgKeys = append(pkgKeys, k)
	}
	sort.Strings(pkgKeys)
	for _, k := range pkgKeys {
		g.Packages = append(g.Packages, *pkgMap[k])
	}

	sortGraph(g)
	return g, nil
}

// printResults prints []Result in text or JSON mode.
// cmd is the command name; query is the search term (may be empty).
// emptyMsg is printed when results are empty in text mode.
// Returns the exit code (always 0 for empty results).
func printResults(cmd, query string, results []search.Result, emptyMsg string) int {
	if jsonMode {
		return PrintJSON(okEnvelope(cmd, query, results, len(results)))
	}
	if len(results) == 0 {
		fmt.Println(emptyMsg)
		return 0
	}
	if filesOnlyMode {
		seenFiles := make(map[string]bool)
		for _, r := range results {
			if r.File != "" && !seenFiles[r.File] {
				fmt.Println(r.File)
				seenFiles[r.File] = true
			}
		}
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
}

func runQuery(args []string) int {
	if len(args) == 0 {
		if jsonMode {
			return PrintJSON(errEnvelope("query", "usage: gograph query <term...>"))
		}
		fmt.Fprintln(os.Stderr, "usage: gograph query <term...>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("query", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Query(g, args)
	return printResults("query", strings.Join(args, " "), results, "no results")
}

func runFocus(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph focus <package>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Focus(g, args[0])
	return printResults("focus", args[0], results, fmt.Sprintf("no focus data found for package %q", args[0]))
}

func runNode(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph node <name>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Node(g, strings.Join(args, " "))
	return printResults("node", strings.Join(args, " "), results, "no results")
}

func runCallers(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph callers <function-or-method-name>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	includeTests := true
	termParts := args[:0]
	for _, a := range args {
		if a == "--no-tests" {
			includeTests = false
		} else {
			termParts = append(termParts, a)
		}
	}
	results := search.Callers(g, strings.Join(termParts, " "), includeTests)
	return printResults("callers", strings.Join(termParts, " "), results, "no callers found")
}

func runCallees(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph callees <function-or-method-name>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	includeTests := true
	termParts := args[:0]
	for _, a := range args {
		if a == "--no-tests" {
			includeTests = false
		} else {
			termParts = append(termParts, a)
		}
	}
	results := search.Callees(g, strings.Join(termParts, " "), includeTests)
	return printResults("callees", strings.Join(termParts, " "), results, "no callees found")
}

func runImplementers(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph implementers <interface>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph: %v\n", err)
		return 1
	}
	results := search.Implementers(g, args[0])
	return printResults("implementers", args[0], results, fmt.Sprintf("No structs found implementing '%s'.", args[0]))
}

func runEnvs(args []string) int {
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph: %v\n", err)
		return 1
	}
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	results := search.Envs(g, term)
	return printResults("envs", term, results, "No environment variable reads found.")
}

func runInterfaces(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph interfaces <struct>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph: %v\n", err)
		return 1
	}
	results := search.Interfaces(g, args[0])
	return printResults("interfaces", args[0], results, fmt.Sprintf("No interfaces found satisfied by '%s'.", args[0]))
}

func runConcurrency(args []string) int {
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph: %v\n", err)
		return 1
	}
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	results := search.Concurrency(g, term)
	return printResults("concurrency", term, results, "No concurrency primitives found.")
}

func runTests(args []string) int {
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph: %v\n", err)
		return 1
	}
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	results := search.Tests(g, term)
	emptyMsg := "No test edges found."
	if term != "" {
		emptyMsg = fmt.Sprintf("No test functions found exercising '%s'.", term)
	}
	return printResults("tests", term, results, emptyMsg)
}

func runMCP(args []string) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve path: %v\n", err)
		return 1
	}

	g, err := loadGraph(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph for MCP server: %v\n", err)
		return 1
	}

	rebuild := func() (*graph.Graph, error) {
		return BuildGraph(absRoot)
	}

	if err := mcp.Serve(g, rebuild, BuildGraph); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		return 1
	}
	return 0
}

func loadGraph(root string) (*graph.Graph, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}
	jsonPath := filepath.Join(absRoot, graphFile)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s — run `gograph build` first: %w", jsonPath, err)
	}
	var g graph.Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parsing graph.json: %w", err)
	}
	return &g, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o640)
}

func writeGitignore(root string) error {
	giPath := filepath.Join(root, ".gitignore")
	const entry = ".gograph/"
	existing, err := os.ReadFile(giPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}
	f, err := os.OpenFile(giPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	prefix := "\n"
	if len(existing) == 0 {
		prefix = ""
	}
	_, err = fmt.Fprintf(f, "%s%s\n", prefix, entry)
	return err
}

func parseDependencies(absRoot string) ([]graph.Dependency, error) {
	modPath := filepath.Join(absRoot, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return nil, err
	}

	var deps []graph.Dependency
	lines := strings.Split(string(data), "\n")
	inRequire := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if line == "require (" {
			inRequire = true
			continue
		}

		if inRequire && line == ")" {
			inRequire = false
			continue
		}

		if inRequire || strings.HasPrefix(line, "require ") {
			line = strings.TrimPrefix(line, "require ")
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				deps = append(deps, graph.Dependency{
					Module:  parts[0],
					Version: parts[1],
				})
			}
		}
	}

	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Module < deps[j].Module
	})

	return deps, nil
}

func bestEffortImportPath(absRoot, relDir string) string {
	modPath := filepath.Join(absRoot, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return relDir
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			mod := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if relDir == "." || relDir == "" {
				return mod
			}
			return mod + "/" + filepath.ToSlash(relDir)
		}
	}
	return relDir
}

func sortGraph(g *graph.Graph) {
	sort.Slice(g.Files, func(i, j int) bool { return g.Files[i].Path < g.Files[j].Path })
	sort.Slice(g.Symbols, func(i, j int) bool { return g.Symbols[i].ID < g.Symbols[j].ID })
	sort.Slice(g.Imports, func(i, j int) bool {
		if g.Imports[i].FromFile != g.Imports[j].FromFile {
			return g.Imports[i].FromFile < g.Imports[j].FromFile
		}
		return g.Imports[i].ImportPath < g.Imports[j].ImportPath
	})
	sort.Slice(g.Calls, func(i, j int) bool {
		if g.Calls[i].File != g.Calls[j].File {
			return g.Calls[i].File < g.Calls[j].File
		}
		return g.Calls[i].Line < g.Calls[j].Line
	})
	sort.Slice(g.EnvReads, func(i, j int) bool {
		if g.EnvReads[i].Key != g.EnvReads[j].Key {
			return g.EnvReads[i].Key < g.EnvReads[j].Key
		}
		return g.EnvReads[i].File < g.EnvReads[j].File
	})
}

const helpText = `gograph — local AST-based Go repository context indexer for AI agents

USAGE
  gograph <command> [arguments] [--json]

GLOBAL FLAGS
  --json                     Output strictly in a machine-parseable JSON envelope.
                             Recommended for all automated agent usage.
  --files-only               Output only a flat, deduplicated list of file paths.
                             Great for extracting checklists without blowing up tokens.

INDEXING
  build [path]               Walk and parse a Go repository. Generates graph.json
                             and 9 targeted Markdown reports in .gograph/.
                             Run after any major code change. Default path: .
                             Supports --precise to perform type-checked Class
                             Hierarchy Analysis (CHA) for more precise call edges.
  stale                      Check if graph.json is older than any source file.
                             Agents should run this before structural analysis.

AGENT WORKFLOW RULES (CRITICAL)
  1. BEFORE editing: ALWAYS run 'gograph plan <symbol>' to understand the impact,
     mapped tests, and execution risks (SQL/Env/Routes) of your target.
  2. AFTER editing: ALWAYS run 'gograph build . --precise' followed by 'gograph review --uncommitted'
     to verify test coverage, complexity, and that no unintended risks were introduced.

SEARCH & NAVIGATION
  query <term...>            Search across symbols, packages, files, imports, and
                             call sites. Case-insensitive, OR logic across terms.
  focus <package>            Show all symbols, imports, and call edges for one
                             package. Token-efficient alternative to reading files.
  node <name>                Show full AST details for a symbol, package, or file.
  source <name>              Extract raw source code (functions, interfaces, consts).
  public <package>           List only the exported (public) API of a package.
  fields <struct>            List all fields and types of a struct.
  embeds <struct>            Find which structs embed the given struct.
  imports <pkg>              Find all files importing a given package path.
  mutate <field>             Find functions that mutate a specific struct field.
  arity [--min 5]            Find functions with many arguments (long parameter list smell).
  skeleton                   Output the whole repository's API signatures with bodies stripped.

CALL GRAPH
  callers <function> [--no-tests]    find functions that call a target function (returns exact call-site source snippet)
  callees <function> [--no-tests]    find functions that a target function calls (returns exact call-site source snippet)
  impact <name>              Full downstream blast radius (recursive callers).
  impact --uncommitted       Perform blast radius analysis on all currently modified,
                             uncommitted code lines using git diff.
  path <from> <to>           Shortest call chain between two symbols (BFS).
  trace <err_str>            Find the origin of an error and trace backwards to entry points.
  orphans                    Functions with 0 explicit incoming calls in the call graph.
                             Useful for spotting potentially unused code.

INTERFACES & TYPES
  implementers <interface>   Structs that implement the named interface (duck-typing).
  interfaces <struct>        Interfaces satisfied by the named struct (duck-typing).
  constructors <struct>      Find factory functions returning the named struct.
  schema <table>             Find structs mapped to a database table/schema via tags.
  globals <pkg>              Find pkg-level vars, consts, and mutators.
  mocks <interface>          Find structs implementing an interface in test/mock files.
  fixtures <pkg>             Find test helper structs and functions in test files.

CODE QUALITY
  check [--config]           Run static policy checks using .gograph/checks.json.
  check --uncommitted        Run checks, including uncommitted code.
  check --since <ref>        Run checks, including API drift against a baseline.
  boundaries [--config]      Verify package architecture constraints using boundaries.json.
  boundaries --create        Auto-generate a baseline boundaries.json from the current repo.
  complexity [symbol]        Cyclomatic complexity per function, highest first.
                             Filter by symbol name substring. Labels: LOW / MEDIUM /
                             HIGH / VERY HIGH (McCabe thresholds: 5 / 10 / 20).
  coupling [package]         Fan-in, fan-out, and instability per package.
                             Instability = FanOut / (FanIn + FanOut). Range [0,1].
  context <symbol>           Bundle node+source+callers+callees+tests in one call.
                             Replaces 4–5 separate commands. Primary token saver.
  hotspot [--top N]          Rank functions by incoming call count (fan-in).
                             Shows the most-depended-on code to study first.
                             Default: --top 10
  deps <pkg> [--transitive]  Direct import dependencies of a package.
                             Add --transitive for the full closure (BFS).
  changes                    Symbols modified/added/deleted since last 'build'.
                             Surfaces new functions, deleted files, and modified
                             symbols without re-reading changed source files.
  godobj [flags]             God-object struct candidates scored by method count,
                             field count, and outgoing calls.
                             Flags: --methods N  --fields N  --calls N  --top N
                             Defaults: --methods 5  --fields 8  --calls 15  --top 10
  plan <symbol>              Generate an operational change plan (callers, tests, risk profile)
                             before editing a symbol.
  plan --uncommitted         Generate a change plan for all currently uncommitted modified symbols.
  review <symbol>            Generate a post-edit final review report for a modified symbol.
  review --uncommitted       Generate a post-edit final review report for all uncommitted changes.
  api --since <ref>          Identify breaking API and contract changes since a git reference (e.g. main).
                             Run 'gograph build . --precise' before this for best results.

EXTRACTION
  routes                     All HTTP REST API routes and their handler functions.
  sql                        Raw SQL queries mapped to the functions that run them.
  errorflow <term>           Trace likely error paths up to entry points (AST heuristic, NO SSA)
  errors                     Custom error variables and panics mapped to their source.
  envs [term]                Every os.Getenv / viper.Get* read with file and line.
  concurrency [term]         Goroutine spawns, channel ops, mutex locks, WaitGroups.
  tests [symbol]             Test functions that exercise a named symbol.

AGENT INTEGRATION
  capabilities               Token-optimized cheat sheet for AI agents. Run this
                             first so the agent knows how to use gograph.
  mcp [path]                 Start a Model Context Protocol server over stdio.
                             Exposes graph queries as native tools for AI clients.
  add-claude-plugin          Install gograph as a Claude MCP plugin. Also injects
                             CLAUDE.md steering rules and a smart PreToolUse hook
                             that redirects Go symbol greps to gograph tools.
  hook-guard                 PreToolUse hook invoked by Claude Code. Reads a JSON
                             tool call from stdin; blocks grep on Go symbols and
                             suggests the equivalent gograph command. Exit 0 = allow,
                             exit 2 = block. Not intended for direct human use.

OTHER
  version, -v                Print version.
  help, -h                   Show this help.

OUTPUTS (after 'build')
  .gograph/graph.json        Machine-readable graph (JSON).
  .gograph/GRAPH_REPORT.md   Master index report.
  .gograph/graph-symbols.md  .gograph/graph-routes.md
  .gograph/graph-sql.md      .gograph/graph-concurrency.md
  .gograph/graph-tests.md    .gograph/graph-deps.md
  .gograph/graph-errors.md   .gograph/graph-config.md
`

func printHelp() {
	fmt.Print(helpText)
}

func printCommandHelp(cmd string) {
	lines := strings.Split(helpText, "\n")
	found := false
	for _, line := range lines {
		if !found && strings.HasPrefix(line, "  "+cmd) && (len(line) == len("  "+cmd) || line[len("  "+cmd)] == ' ' || line[len("  "+cmd)] == '[') {
			found = true
			fmt.Println("USAGE")
			descStart := strings.Index(line[2:], "  ")
			if descStart != -1 {
				usagePart := line[2 : 2+descStart]
				descPart := strings.TrimSpace(line[2+descStart:])
				fmt.Printf("  gograph %s\n\nDESCRIPTION\n  %s\n", strings.TrimSpace(usagePart), descPart)
			} else {
				fmt.Printf("  gograph %s\n\nDESCRIPTION\n", strings.TrimSpace(line))
			}
		} else if found {
			if strings.HasPrefix(line, "                             ") || strings.HasPrefix(line, "    ") {
				fmt.Printf("  %s\n", strings.TrimSpace(line))
			} else {
				break
			}
		}
	}
	if !found {
		printHelp()
	}
}

// runPath finds the shortest call chain between two symbols via BFS.
func runPath(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gograph path <from-symbol> <to-symbol>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	chain := search.Path(g, args[0], args[1], true)
	if len(chain) == 0 {
		fmt.Printf("No call path found from %q to %q.\n", args[0], args[1])
		return 0
	}
	fmt.Printf("Call path: %s → %s\n", args[0], args[1])
	for i, step := range chain {
		fmt.Printf("  %d. %s\n", i+1, step.String())
	}
	return 0
}

// runStale checks whether graph.json is out of date relative to source files.
func runStale() int {
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("stale", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	absRoot, _ := filepath.Abs(".")
	sr := search.Stale(g, absRoot)
	if jsonMode {
		return PrintJSON(okEnvelope("stale", "", sr, len(sr.ChangedFiles)))
	}
	if !sr.IsStale {
		fmt.Printf("Graph is up to date (generated: %s).\n", sr.GraphAge)
		return 0
	}
	fmt.Printf("Graph is STALE (generated: %s). %d file(s) changed:\n", sr.GraphAge, len(sr.ChangedFiles))
	for _, f := range sr.ChangedFiles {
		fmt.Printf("  %s\n", f)
	}
	fmt.Println("Run `gograph build .` to refresh.")
	return 0
}

// runOrphans uses reachability analysis to find truly unreachable symbols.
func runOrphans() int {
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("orphans", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.ReachableOrphans(g)
	return printResults("orphans", "", results, "No unreachable symbols found.")
}

// runGodObj detects god-object struct candidates using configurable thresholds.
// Flags: --methods N, --fields N, --calls N, --top N
func runGodObj(args []string) int {
	p := search.DefaultGodObjectParams()

	// Parse --key value pairs manually (no external flag lib).
	for i := 0; i < len(args)-1; i++ {
		val := 0
		if _, err := fmt.Sscanf(args[i+1], "%d", &val); err != nil {
			fmt.Fprintf(os.Stderr, "invalid value for %s: %q\n", args[i], args[i+1])
			return 1
		}
		switch args[i] {
		case "--methods":
			p.MinMethods = val
			i++
		case "--fields":
			p.MinFields = val
			i++
		case "--calls":
			p.MinCalls = val
			i++
		case "--top":
			p.Top = val
			i++
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			return 1
		}
	}

	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("godobj", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	candidates := search.GodObjects(g, p)
	if jsonMode {
		return PrintJSON(okEnvelope("godobj", "", candidates, len(candidates)))
	}
	if len(candidates) == 0 {
		fmt.Printf("No god-object candidates found (methods>%d, fields>%d, calls>%d).\n",
			p.MinMethods, p.MinFields, p.MinCalls)
		return 0
	}

	fmt.Printf("God Object Candidates (methods>%d, fields>%d, calls>%d):\n\n",
		p.MinMethods, p.MinFields, p.MinCalls)
	for _, c := range candidates {
		fmt.Printf("[%-8s] %s — %d methods, %d fields, %d outgoing calls  (%s:%d)\n",
			c.Severity, c.Name, c.MethodCount, c.FieldCount, c.OutgoingCalls, c.File, c.Line)
	}
	return 0
}

func runArity(args []string) int {
	minArgs := 5
	for i, arg := range args {
		if arg == "--min" && i+1 < len(args) {
			parsed, err := strconv.Atoi(args[i+1])
			if err == nil {
				minArgs = parsed
			}
		}
	}

	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("arity", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Arity(g, minArgs)
	if jsonMode {
		return PrintJSON(okEnvelope("arity", "", results, len(results)))
	}

	if len(results) == 0 {
		fmt.Printf("No functions found with >= %d arguments.\n", minArgs)
		return 0
	}

	fmt.Printf("Functions with %d+ arguments:\n", minArgs)
	for _, r := range results {
		fmt.Printf("  %s (%s:%d) - %s\n", r.Name, r.File, r.Line, r.Detail)
	}
	return 0
}

// runComplexity estimates cyclomatic complexity for matching functions.
func runComplexity(args []string) int {
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("complexity", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Complexity(g, term)
	if jsonMode {
		return PrintJSON(okEnvelope("complexity", term, results, len(results)))
	}
	if len(results) == 0 {
		if term != "" {
			fmt.Printf("No functions found matching %q.\n", term)
		} else {
			fmt.Println("No functions found in graph.")
		}
		return 0
	}
	fmt.Printf("Cyclomatic Complexity (sorted highest first):\n\n")
	for _, r := range results {
		fmt.Printf("[%-9s] score=%-4d %s  (%s:%d)\n",
			r.Label, r.Score, r.Symbol, r.File, r.Line)
	}
	return 0
}

// runCoupling shows package fan-in, fan-out, and instability metrics.
func runCoupling(args []string) int {
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("coupling", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Coupling(g, term)
	if jsonMode {
		return PrintJSON(okEnvelope("coupling", term, results, len(results)))
	}
	if len(results) == 0 {
		if term != "" {
			fmt.Printf("No packages found matching %q.\n", term)
		} else {
			fmt.Println("No package import edges found in graph.")
		}
		return 0
	}
	fmt.Printf("Package Coupling (sorted by instability, highest first):\n\n")
	fmt.Printf("%-55s  %6s  %6s  %s\n", "Package", "FanOut", "FanIn", "Instability")
	fmt.Printf("%s\n", strings.Repeat("-", 82))
	for _, r := range results {
		instStr := fmt.Sprintf("%.2f", r.Instability)
		if r.Instability < 0 {
			instStr = "n/a"
		}
		fmt.Printf("%-55s  %6d  %6d  %s\n", r.Package, r.FanOut, r.FanIn, instStr)
	}
	return 0
}

// runContext bundles node+source+callers+callees+tests for a symbol in one call.
func runContext(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph context <symbol> [--limit N]")
		return 1
	}
	term := ""
	limit := 0
	filtered := args[:0]
	i := 0
	for i < len(args) {
		a := args[i]
		if (a == "--limit" || a == "-n") && i+1 < len(args) {
			if n, err := strconv.Atoi(args[i+1]); err == nil {
				limit = n
			}
			i += 2
			continue
		}
		filtered = append(filtered, a)
		i++
	}
	if len(filtered) > 0 {
		term = strings.Join(filtered, " ")
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("context", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	root, _ := filepath.Abs(".")
	result := search.Context(g, root, term)
	if result == nil {
		if jsonMode {
			return PrintJSON(okEnvelope("context", term, nil, 0))
		}
		fmt.Printf("No symbol found matching %q.\n", term)
		return 0
	}
	if jsonMode {
		count := len(result.Node) + len(result.Callers) + len(result.Callees) + len(result.Tests)
		if result.Source != "" {
			count++
		}
		return PrintJSON(okEnvelope("context", term, result, count))
	}

	fmt.Printf("=== CONTEXT: %s ===\n\n", term)

	if len(result.Node) > 0 {
		fmt.Println("--- NODE ---")
		for _, r := range result.Node {
			fmt.Println(r.String())
		}
		fmt.Println()
	}

	if result.Source != "" {
		fmt.Println("--- SOURCE ---")
		fmt.Println(result.Source)
	} else if result.SourceErr != nil {
		fmt.Printf("(source unavailable: %v)\n\n", result.SourceErr)
	}

	if limit > 0 && len(result.Callers) > limit {
		fmt.Printf("--- CALLERS (showing %d of %d) ---\n", limit, len(result.Callers))
		for _, r := range result.Callers[:limit] {
			fmt.Println(r.String())
		}
		fmt.Printf("... and %d more callers. Use --limit %d to see all.\n\n", len(result.Callers)-limit, len(result.Callers))
	} else if len(result.Callers) > 0 {
		fmt.Printf("--- CALLERS (%d) ---\n", len(result.Callers))
		for _, r := range result.Callers {
			fmt.Println(r.String())
		}
		fmt.Println()
	}

	if limit > 0 && len(result.Callees) > limit {
		fmt.Printf("--- CALLEES (showing %d of %d) ---\n", limit, len(result.Callees))
		for _, r := range result.Callees[:limit] {
			fmt.Println(r.String())
		}
		fmt.Printf("... and %d more callees. Use --limit %d to see all.\n\n", len(result.Callees)-limit, len(result.Callees))
	} else if len(result.Callees) > 0 {
		fmt.Printf("--- CALLEES (%d) ---\n", len(result.Callees))
		for _, r := range result.Callees {
			fmt.Println(r.String())
		}
		fmt.Println()
	}

	if len(result.Tests) > 0 {
		fmt.Printf("--- TESTS (%d) ---\n", len(result.Tests))
		for _, r := range result.Tests {
			fmt.Println(r.String())
		}
		fmt.Println()
	}
	return 0
}

// runHotspot ranks functions by incoming call count.
func runHotspot(args []string) int {
	top := 10
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--top" {
			if _, err := fmt.Sscanf(args[i+1], "%d", &top); err != nil {
				fmt.Fprintf(os.Stderr, "invalid --top value: %q\n", args[i+1])
				return 1
			}
			i++
		}
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("hotspot", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Hotspot(g, top)
	if jsonMode {
		return PrintJSON(okEnvelope("hotspot", "", results, len(results)))
	}
	if len(results) == 0 {
		fmt.Println("No hotspot data found (no call edges in graph).")
		return 0
	}
	label := fmt.Sprintf("top %d", top)
	if top == 0 {
		label = "all"
	}
	fmt.Printf("Hotspot Functions (%s, sorted by incoming calls):\n\n", label)
	for i, r := range results {
		fmt.Printf("%3d.  %-6d calls  %s  (%s:%d)\n", i+1, r.IncomingCalls, r.Name, r.File, r.Line)
	}
	return 0
}

// runDeps shows direct (and optionally transitive) imports for a package.
func runDeps(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph deps <package> [--transitive]")
		return 1
	}
	pkg := args[0]
	transitive := false
	for _, a := range args[1:] {
		if a == "--transitive" {
			transitive = true
		}
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("deps", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result := search.Deps(g, pkg, transitive)
	if result == nil {
		if jsonMode {
			return PrintJSON(okEnvelope("deps", pkg, nil, 0))
		}
		fmt.Printf("No package found matching %q.\n", pkg)
		return 0
	}
	if jsonMode {
		return PrintJSON(okEnvelope("deps", pkg, result, len(result.Direct)+len(result.Transitive)))
	}
	fmt.Printf("Package: %s\n\nDirect imports (%d):\n", result.Package, len(result.Direct))
	for _, imp := range result.Direct {
		fmt.Printf("  %s\n", imp)
	}
	if transitive {
		fmt.Printf("\nTransitive imports (%d):\n", len(result.Transitive))
		for _, imp := range result.Transitive {
			fmt.Printf("  %s\n", imp)
		}
	}
	return 0
}

// runChanges reports symbols modified/added/deleted since the last build.
func runChanges() int {
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("changes", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	root, _ := filepath.Abs(".")
	result := search.Changes(g, root)
	if jsonMode {
		return PrintJSON(okEnvelope("changes", "", result, len(result.ChangedFiles)+len(result.Symbols)))
	}

	if len(result.ChangedFiles) == 0 && len(result.Symbols) == 0 {
		fmt.Printf("No changes detected (graph generated: %s).\n",
			result.GraphAge.Format("2006-01-02 15:04:05 UTC"))
		return 0
	}

	fmt.Printf("Changes since graph build (%s):\n\n",
		result.GraphAge.Format("2006-01-02 15:04:05 UTC"))

	if len(result.ChangedFiles) > 0 {
		fmt.Printf("Modified files (%d):\n", len(result.ChangedFiles))
		for _, f := range result.ChangedFiles {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
	}

	counts := map[search.ChangeStatus]int{}
	for _, s := range result.Symbols {
		counts[s.Status]++
	}
	fmt.Printf("Affected symbols: %d modified, %d new, %d deleted\n\n",
		counts[search.ChangeModified], counts[search.ChangeNew], counts[search.ChangeDeleted])

	for _, sym := range result.Symbols {
		switch sym.Status {
		case search.ChangeNew:
			fmt.Printf("[NEW     ] %s  (%s:%d)\n", sym.Name, sym.File, sym.Line)
		case search.ChangeDeleted:
			fmt.Printf("[DELETED ] %s  (%s)\n", sym.Name, sym.File)
		case search.ChangeModified:
			fmt.Printf("[MODIFIED] %s  (%s:%d)\n", sym.Name, sym.File, sym.Line)
		}
	}
	return 0
}

// runSource extracts the raw source code of a named symbol.
func runSource(args []string) int {
	if len(args) == 0 {
		if jsonMode {
			return PrintJSON(errEnvelope("source", "usage: gograph source <name>"))
		}
		fmt.Fprintln(os.Stderr, "usage: gograph source <name>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("source", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	term := strings.Join(args, " ")
	root, _ := filepath.Abs(".")
	src, err := search.Source(g, root, term)
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("source", err.Error()))
		}
		fmt.Fprintf(os.Stderr, "source: %v\n", err)
		return 1
	}
	if jsonMode {
		return PrintJSON(okEnvelope("source", term, src, 1))
	}
	fmt.Println(src)
	return 0
}

// runPublic lists only the exported (public) API of a package.
func runPublic(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph public <package>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Public(g, args[0])
	return printResults("public", args[0], results, fmt.Sprintf("No exported symbols found for package %q.", args[0]))
}

// runFields lists all fields and types of a struct.
func runFields(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph fields <struct>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Fields(g, args[0])
	return printResults("fields", args[0], results, fmt.Sprintf("No fields found for struct %q.", args[0]))
}

// runEmbeds finds which structs embed the given struct.
func runEmbeds(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph embeds <struct>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Embeds(g, args[0])
	return printResults("embeds", args[0], results, fmt.Sprintf("No structs found embedding %q.", args[0]))
}

// runImports finds all files importing a given package path.
func runImports(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph imports <pkg>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.ExternalImports(g, args[0])
	return printResults("imports", args[0], results, fmt.Sprintf("No files found importing %q.", args[0]))
}

// runImpact traverses the call graph backwards to find all symbols that eventually call the target.
func runImpact(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph impact <symbol> OR gograph impact --uncommitted")
		return 1
	}

	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if args[0] == "--uncommitted" {
		return runImpactUncommitted(g)
	}

	results := search.Impact(g, strings.Join(args, " "), true)
	return printResults("impact", strings.Join(args, " "), results, fmt.Sprintf("No callers found in blast radius of %q.", args[0]))
}

// runImpactUncommitted parses git diff to find modified symbols, then computes their blast radius.
func runImpactUncommitted(g *graph.Graph) int {
	modifiedSymbolNames, err := search.UncommittedSymbols(g)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if len(modifiedSymbolNames) == 0 {
		return printResults("impact", "--uncommitted", nil, "No uncommitted modified symbols found in the graph.")
	}

	reason := fmt.Sprintf("downstream impact of uncommitted changes (%d symbols)", len(modifiedSymbolNames))
	results := search.ImpactMultiple(g, modifiedSymbolNames, reason, true)
	return printResults("impact", "--uncommitted", results, "No callers found in blast radius of uncommitted changes.")
}

// runRoutes lists all HTTP REST API routes and their handler functions.
func runRoutes() int {
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Routes(g)
	return printResults("routes", "", results, "No HTTP routes found.")
}

// runSQL lists raw SQL queries mapped to the functions that run them.
func runSQL(args []string) int {
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.SQL(g, term)
	return printResults("sql", term, results, "No SQL queries found.")
}

// runErrors lists custom error variables and panics mapped to their source.
func runErrors(args []string) int {
	term := ""
	includeTests := true
	filtered := args[:0]
	for _, a := range args {
		if a == "--no-tests" {
			includeTests = false
		} else {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) > 0 {
		term = filtered[0]
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Errors(g, term, includeTests)
	return printResults("errors", term, results, "No custom errors or panics found.")
}

// runSkeleton prints a stripped skeleton of the repository structure.
func runSkeleton() int {
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(search.Skeleton(g))
	return 0
}

// runMutate finds functions that mutate the given struct field.
func runMutate(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph mutate <Field>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Mutate(g, args[0])
	return printResults("mutate", args[0], results, "No mutations found for that field.")
}

// runTrace traces an error string backwards from entry points.
func runTrace(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph trace <error string> [--no-tests]")
		return 1
	}
	includeTests := true
	termParts := args[:0]
	for _, a := range args {
		if a == "--no-tests" {
			includeTests = false
		} else {
			termParts = append(termParts, a)
		}
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	term := strings.Join(termParts, " ")
	traces := search.Trace(g, term, includeTests)
	if len(traces) == 0 {
		fmt.Printf("No trace found for error matching %q\n", term)
		return 0
	}

	for i, t := range traces {
		fmt.Printf("=== Trace %d: %q generated in %s ===\n", i+1, t.Error.Message, t.Error.Function)
		for j, step := range t.Path {
			fmt.Printf("  %d. %s (%s:%d)\n", j+1, step.Name, step.File, step.Line)
		}
		fmt.Println()
	}
	return 0
}

func runErrorFlow(args []string) int {
	if len(args) == 0 {
		if jsonMode {
			return PrintJSON(errEnvelope("errorflow", "Usage: gograph errorflow <error-string|ErrSymbol>"))
		}
		fmt.Println("Usage: gograph errorflow <error-string|ErrSymbol>")
		return 1
	}

	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("errorflow", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	term := strings.Join(args, " ")
	report := search.ErrorFlow(g, term)

	if jsonMode {
		return PrintJSON(okEnvelope("errorflow", term, report, len(report.Paths)))
	}

	fmt.Printf("ErrorFlow Report for %q\n", term)
	fmt.Println("==================================================")
	fmt.Println("⚠️  DISCLAIMER: Likely error path based on static call graph and AST references.")
	fmt.Println("   Highly useful for navigation, not proof. No SSA/data-flow tracking performed.")
	fmt.Println("==================================================")
	fmt.Println()

	if len(report.DefinitionSites) > 0 {
		fmt.Println("1. Definition Sites:")
		for _, r := range report.DefinitionSites {
			fmt.Printf("   - %s (%s:%d) -> %s\n", r.Name, r.File, r.Line, r.Detail)
		}
		fmt.Println()
	}

	if len(report.ReturnSites) > 0 {
		fmt.Println("2. Return / Wrap / Check Sites:")
		for _, r := range report.ReturnSites {
			fmt.Printf("   - %s (%s:%d) -> %s\n", r.Name, r.File, r.Line, r.Detail)
		}
		fmt.Println()
	}

	if len(report.Paths) > 0 {
		fmt.Println("3. Likely Route / Entrypoint Paths:")
		for i, p := range report.Paths {
			confidence := "MEDIUM"
			if len(report.DefinitionSites) > 0 {
				confidence = "HIGH"
			}
			fmt.Printf("   Path %d [Confidence: %s] (Originates in %s):\n", i+1, confidence, p.Error.Function)
			for j, step := range p.Path {
				fmt.Printf("      %d. %s (%s:%d) - %s\n", j+1, step.Name, step.File, step.Line, step.Detail)
			}
			fmt.Println()
		}
	} else {
		fmt.Println("3. Likely Route / Entrypoint Paths:\n   - No complete path to an HTTP route or main entrypoint found.")
		fmt.Println()
	}

	if len(report.RelatedTests) > 0 {
		fmt.Println("4. Related Tests:")
		for _, r := range report.RelatedTests {
			fmt.Printf("   - %s (%s:%d) -> %s\n", r.Name, r.File, r.Line, r.Detail)
		}
		fmt.Println()
	}

	return 0
}

func runConstructors(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph constructors <struct>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		return 1
	}
	results := search.Constructors(g, args[0])
	return printResults("constructors", args[0], results, fmt.Sprintf("No constructors found for struct '%s'.", args[0]))
}

func runSchema(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph schema <table>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		return 1
	}
	results := search.Schema(g, args[0])
	return printResults("schema", args[0], results, fmt.Sprintf("No struct found mapped to table '%s'.", args[0]))
}

func runGlobals(args []string) int {
	term := ""
	if len(args) > 0 {
		term = args[0]
	} else {
		fmt.Fprintln(os.Stderr, "Usage: gograph globals <package>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		return 1
	}
	results := search.Globals(g, term)
	return printResults("globals", term, results, "No globals or mutators found.")
}

func runMocks(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph mocks <interface>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		return 1
	}
	results := search.Mocks(g, args[0])
	return printResults("mocks", args[0], results, fmt.Sprintf("No mocks found for interface '%s'.", args[0]))
}

func runFixtures(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph fixtures <package>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		return 1
	}
	results := search.Fixtures(g, args[0])
	return printResults("fixtures", args[0], results, fmt.Sprintf("No fixtures found for package '%s'.", args[0]))
}

func runBoundaries(args []string) int {
	configPath := ".gograph/boundaries.json"
	createMode := false
	for i, a := range args {
		if a == "--config" && i+1 < len(args) {
			configPath = args[i+1]
			break
		}
		if a == "--create" {
			createMode = true
		}
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		return 1
	}

	if createMode {
		if err := search.CreateBoundaries(g, configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create boundaries: %v\n", err)
			return 1
		}
		fmt.Printf("Successfully created baseline boundaries at %s\n", configPath)
		return 0
	}

	results, err := search.Boundaries(g, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Boundaries error: %v\n", err)
		return 1
	}
	code := printResults("boundaries", configPath, results, "No boundary violations found. Architecture is clean!")
	if len(results) > 0 && !jsonMode {
		// Exit with non-zero if violations exist (useful for CI/CD)
		return 1
	}
	return code
}

// runPlan generates an operational change plan for one or more symbols or for uncommitted changes.
func runPlan(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph plan <symbol> OR gograph plan --uncommitted")
		return 1
	}

	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	var symbolNames []string
	var title string

	if args[0] == "--uncommitted" {
		symbolNames, err = search.UncommittedSymbols(g)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(symbolNames) == 0 {
			fmt.Println("No uncommitted modified symbols found in the graph.")
			return 0
		}
		title = "Uncommitted Changes"
	} else {
		symbolNames = []string{strings.Join(args, " ")}
		title = symbolNames[0]
	}

	plan := search.Plan(g, symbolNames, title)

	if jsonMode {
		return PrintJSON(okEnvelope("plan", title, plan, 1))
	}

	fmt.Print(plan.String())
	return 0
}

// runReview generates a post-edit checklist for modified symbols.
func runReview(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph review <symbol> OR gograph review --uncommitted")
		return 1
	}

	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	var symbolNames []string
	var title string

	if args[0] == "--uncommitted" {
		symbolNames, err = search.UncommittedSymbols(g)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(symbolNames) == 0 {
			fmt.Println("No uncommitted modified symbols found in the graph.")
			return 0
		}
		title = "Uncommitted Changes"
	} else {
		symbolNames = []string{args[0]}
		title = args[0]
	}

	report := search.Review(g, symbolNames, title)

	if jsonMode {
		return PrintJSON(okEnvelope("review", title, report, 1))
	}

	fmt.Print(report.String())
	return 0
}
