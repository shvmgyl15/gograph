// Package cli wires together the CLI commands.
package cli

import (
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/mcp"
	"github.com/ozgurcd/gograph/internal/parser"
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
const Version = "1.3.1"

// Run is the entrypoint called from main.
func Run(args []string) int {
	if len(args) == 0 {
		printHelp()
		return 0
	}
	switch args[0] {
	case "build":
		return runBuild(args[1:])
	case "query":
		return runQuery(args[1:])
	case "focus":
		return runFocus(args[1:])
	case "node":
		return runNode(args[1:])
	case "callers":
		return runCallers(args[1:])
	case "callees":
		return runCallees(args[1:])
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

COMMANDS (token-optimized):
build .              : parse AST, gen GRAPH_REPORT.md & .gograph/*
query <str>          : search symbols/files/pkgs
focus <pkg>          : isolate context for a package
callers <fn>         : who calls fn
callees <fn>         : what fn calls
impact <sym>         : blast radius (downstream callers)
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
imports <pkg>        : trace external/internal usage`)
	return 0
}

func runBuild(args []string) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
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

func runQuery(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph query <term...>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Query(g, args)
	if len(results) == 0 {
		fmt.Println("no results")
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
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
	if len(results) == 0 {
		fmt.Printf("no focus data found for package %q\n", args[0])
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
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
	if len(results) == 0 {
		fmt.Println("no results")
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
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
	if len(results) == 0 {
		fmt.Println("no callers found")
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
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
	if len(results) == 0 {
		fmt.Println("no callees found")
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
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
	if len(results) == 0 {
		fmt.Printf("No structs found implementing '%s'.\n", args[0])
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
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
	if len(results) == 0 {
		fmt.Println("No environment variable reads found.")
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
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
	if len(results) == 0 {
		fmt.Printf("No interfaces found satisfied by '%s'.\n", args[0])
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
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
	if len(results) == 0 {
		fmt.Println("No concurrency primitives found.")
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
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
	if len(results) == 0 {
		if term != "" {
			fmt.Printf("No test functions found exercising '%s'.\n", term)
		} else {
			fmt.Println("No test edges found.")
		}
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
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
  gograph <command> [arguments]

INDEXING
  build [path]               Walk and parse a Go repository. Generates graph.json
                             and 9 targeted Markdown reports in .gograph/.
                             Run after any major code change. Default path: .
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
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	absRoot, _ := filepath.Abs(".")
	sr := search.Stale(g, absRoot)
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
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.ReachableOrphans(g)
	if len(results) == 0 {
		fmt.Println("No unreachable symbols found.")
		return 0
	}
	fmt.Printf("%d unreachable symbol(s) found:\n", len(results))
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
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
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	candidates := search.GodObjects(g, p)
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
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Complexity(g, term)
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
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Coupling(g, term)
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
