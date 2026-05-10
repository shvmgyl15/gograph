// Package cli wires together the CLI commands.
package cli

import (
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"os/exec"
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

	// Strip --json from args before dispatch; set the package-level flag.
	jsonMode = false
	filtered := args[:0]
	for _, a := range args {
		if a == "--json" {
			jsonMode = true
		} else {
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
	case "path":
		return runPath(args[1:])
	case "stale":
		return runStale()
	case "orphans":
		return runOrphans()
	case "godobj":
		return runGodObj(args[1:])
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

INSTRUCTIONS FOR AI AGENTS:
Use gograph first for repo navigation; use grep/raw reads only when the graph lacks detail or exact source is needed.
Use gograph to understand repository structure, dependencies, and call graphs.
To save tokens, the graph is split into targeted files in .gograph/. 
Read .gograph/GRAPH_REPORT.md first.

🚨 CRITICAL WARNING: DO NOT READ .gograph/graph.json DIRECTLY! It is a massive database file that will crash your context window. Use the commands below to extract targeted JSON slices instead.

COMMANDS (token-optimized):
(Note: All search/navigation commands support --json for stable machine parsing)
build . [--precise]  : parse AST, gen GRAPH_REPORT.md & .gograph/*

RULES FOR --precise:
- Default (build .): Use during active, messy development. It's lightning-fast, tolerates syntax/build errors, and uses AST heuristics (duck-typing) for interfaces.
- Precise (build . --precise): Use before a major refactor or when measuring blast radius (impact). Slower, but provides type-checked interface analysis and more precise call edges; requires compilable code.
query <str>          : search symbols/files/pkgs
focus <pkg>          : isolate context for a package
callers <fn>         : who calls fn
callees <fn>         : what fn calls
impact <sym>         : blast radius (downstream callers)
impact --uncommitted : blast radius of all your uncommitted code changes
source <sym>         : exact code of sym
node <sym>           : AST info of sym
fields <struct>      : fields/types of struct
embeds <struct>      : structs embedding this struct
interfaces <struct>  : duck-type interface check
implementers <iface> : structs implementing iface
public <pkg>         : exported API of pkg
routes               : HTTP REST routes
sql                  : raw SQL queries mapped
errors               : custom errors/panics
envs [str]           : os.Getenv/viper reads
concurrency [str]    : goroutines/channels/mutexes
tests <sym>          : tests exercising sym
path <from> <to>     : shortest call chain between two symbols (BFS)
stale                : check if graph is out of date vs source files
orphans              : reachability-based dead code analysis
godobj               : find god-object struct candidates (--methods N --fields N --calls N --top N)
complexity [sym]     : cyclomatic complexity estimate per function (highest first)
coupling [pkg]       : fan-in, fan-out, and instability per package
context <sym>        : bundle node+source+callers+callees+tests (saves 4-5 tool calls)
hotspot [--top N]    : rank functions by incoming calls (study these first)
deps <pkg> [--transitive] : import dependency tree of a package
changes              : symbols modified/new/deleted since last build
imports <pkg>        : trace external/internal usage`)
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
	results := search.Callers(g, strings.Join(args, " "))
	return printResults("callers", strings.Join(args, " "), results, "no callers found")
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
	results := search.Callees(g, strings.Join(args, " "))
	return printResults("callees", strings.Join(args, " "), results, "no callees found")
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

	if err := mcp.Serve(g, rebuild); err != nil {
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
	defer f.Close()
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

func printHelp() {
	fmt.Print(`gograph — local AST-based Go repository context indexer for AI agents

USAGE
  gograph <command> [arguments] [--json]

GLOBAL FLAGS
  --json                     Output strictly in a machine-parseable JSON envelope.
                             Recommended for all automated agent usage.

INDEXING
  build [path]               Walk and parse a Go repository. Generates graph.json
                             and 9 targeted Markdown reports in .gograph/.
                             Run after any major code change. Default path: .
                             Supports --precise to perform type-checked Class
                             Hierarchy Analysis (CHA) for more precise call edges.
  stale                      Check if graph.json is older than any source file.
                             Agents should run this before structural analysis.

SEARCH & NAVIGATION
  query <term...>            Search across symbols, packages, files, imports, and
                             call sites. Case-insensitive, OR logic across terms.
  focus <package>            Show all symbols, imports, and call edges for one
                             package. Token-efficient alternative to reading files.
  node <name>                Show full AST details for a symbol, package, or file.
  source <name>              Extract the raw source code of a named symbol.
  public <package>           List only the exported (public) API of a package.
  fields <struct>            List all fields and types of a struct.
  embeds <struct>            Find which structs embed the given struct.
  imports <pkg>              Find all files importing a given package path.

CALL GRAPH
  callers <name>             Functions/methods that call the named symbol.
  callees <name>             Functions/methods called by the named symbol.
  impact <name>              Full downstream blast radius (recursive callers).
  impact --uncommitted       Perform blast radius analysis on all currently modified,
                             uncommitted code lines using git diff.
  path <from> <to>           Shortest call chain between two symbols (BFS).
  orphans                    Functions unreachable from any entry point
                             (reachability analysis — stricter than 0-incoming).

INTERFACES & TYPES
  implementers <interface>   Structs that implement the named interface (duck-typing).
  interfaces <struct>        Interfaces satisfied by the named struct (duck-typing).

CODE QUALITY
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

EXTRACTION
  routes                     All HTTP REST API routes and their handler functions.
  sql                        Raw SQL queries mapped to the functions that run them.
  errors                     Custom error variables and panics mapped to their source.
  envs [term]                Every os.Getenv / viper.Get* read with file and line.
  concurrency [term]         Goroutine spawns, channel ops, mutex locks, WaitGroups.
  tests [symbol]             Test functions that exercise a named symbol.

AGENT INTEGRATION
  capabilities               Token-optimized cheat sheet for AI agents. Run this
                             first so the agent knows how to use gograph.
  mcp [path]                 Start a Model Context Protocol server over stdio.
                             Exposes graph queries as native tools for AI clients.

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
`)
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
	chain := search.Path(g, args[0], args[1])
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
		fmt.Fprintln(os.Stderr, "usage: gograph context <symbol>")
		return 1
	}
	term := strings.Join(args, " ")
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

	if len(result.Callers) > 0 {
		fmt.Printf("--- CALLERS (%d) ---\n", len(result.Callers))
		for _, r := range result.Callers {
			fmt.Println(r.String())
		}
		fmt.Println()
	}

	if len(result.Callees) > 0 {
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

	results := search.Impact(g, strings.Join(args, " "))
	return printResults("impact", strings.Join(args, " "), results, fmt.Sprintf("No callers found in blast radius of %q.", args[0]))
}

// runImpactUncommitted parses git diff to find modified symbols, then computes their blast radius.
func runImpactUncommitted(g *graph.Graph) int {
	// Parse git diff for unstaged and staged changes
	out, err := exec.Command("git", "diff", "HEAD", "-U0").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error running git diff:", err)
		return 1
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
			// Basic path matching; in practice, relative paths from git root might need adjusting
			// depending on where gograph is run, but assuming it's run at repo root:
			if strings.HasSuffix(s.File, file) {
				for _, line := range lines {
					if line >= s.Line && line <= s.EndLine {
						if !seenSymbols[s.Name] {
							seenSymbols[s.Name] = true
							modifiedSymbolNames = append(modifiedSymbolNames, s.Name)
						}
						break
					}
				}
			}
		}
	}

	if len(modifiedSymbolNames) == 0 {
		return printResults("impact", "--uncommitted", nil, "No uncommitted modified symbols found in the graph.")
	}

	reason := fmt.Sprintf("downstream impact of uncommitted changes (%d symbols)", len(modifiedSymbolNames))
	results := search.ImpactMultiple(g, modifiedSymbolNames, reason)
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
	if len(args) > 0 {
		term = args[0]
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Errors(g, term)
	return printResults("errors", term, results, "No custom errors or panics found.")
}
