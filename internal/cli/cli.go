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
const Version = "1.1.3"

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

	mdContent := report.Generate(g)
	mdPath := filepath.Join(absRoot, reportFile)
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o640); err != nil {
		fmt.Fprintf(os.Stderr, "error writing GRAPH_REPORT.md: %v\n", err)
		return 1
	}

	fmt.Printf("  packages: %d  files: %d  symbols: %d  calls: %d\n",
		len(g.Packages), len(g.Files), len(g.Symbols), len(g.Calls))
	fmt.Printf("  wrote %s\n", jsonPath)
	fmt.Printf("  wrote %s\n", mdPath)
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
	fmt.Print(`gograph — Go repository graph tool

Commands:
  build [path]         Walk and parse a Go repository. Default: .
  query <term...>      Search symbols, packages, files, imports, calls.
  focus <package>      Focus context purely on a single package and its edges.
  node <name>          Show details for a symbol/package/file.
  callers <name>       Show functions that call the given function/method.
  callees <name>       Show calls made inside the given function/method.
  implementers <name>  Show structs that implement the given interface.
  mcp [path]           Start an MCP server over stdio for AI integration.
  version, -v          Print version.
  help, -h             Show this help.

Outputs:
  .gograph/graph.json      Machine-readable graph (JSON).
  .gograph/GRAPH_REPORT.md Human-readable report (Markdown).
`)
}
