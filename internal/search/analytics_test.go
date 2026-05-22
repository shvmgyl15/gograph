package search_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

// ---------------------------------------------------------------------------
// Context tests
// ---------------------------------------------------------------------------

func buildContextGraph() *graph.Graph {
	return &graph.Graph{
		Symbols: []graph.SymbolNode{
			{
				ID:   "Connect",
				Name: "Connect",
				Kind: graph.KindFunction,
				File: "db.go",
				Line: 10,
				Doc:  "Connect opens a DB connection.",
			},
			{
				ID:   "Disconnect",
				Name: "Disconnect",
				Kind: graph.KindFunction,
				File: "db.go",
				Line: 30,
			},
		},
		Calls: []graph.CallEdge{
			// main calls Connect: CalleeRaw must contain "Connect" for Callers()
			{CallerName: "main", CalleeRaw: "Connect", File: "main.go", Line: 5},
			// Connect calls sql.Open: CallerSymbolID must match Symbol.ID for Callees()
			{CallerName: "Connect", CallerSymbolID: "Connect", CalleeRaw: "sql.Open", File: "db.go", Line: 12},
		},
		TestEdges: []graph.TestEdge{
			{TestFunc: "TestConnect", Target: "Connect", File: "db_test.go", Line: 5},
		},
	}
}

func TestContext_Found(t *testing.T) {
	g := buildContextGraph()
	result := search.Context(g, ".", "Connect")
	if result == nil {
		t.Fatal("expected a ContextResult for 'Connect', got nil")
	}
	if len(result.Node) == 0 {
		t.Error("expected Node results to be non-empty")
	}
	// Callers: "main" calls Connect
	if len(result.Callers) == 0 {
		t.Error("expected at least one caller for Connect")
	}
	// Callees: Connect calls sql.Open
	if len(result.Callees) == 0 {
		t.Error("expected at least one callee for Connect")
	}
	// Tests: TestConnect exercises Connect
	if len(result.Tests) == 0 {
		t.Error("expected at least one test for Connect")
	}
}

func TestContext_NotFound(t *testing.T) {
	g := buildContextGraph()
	result := search.Context(g, ".", "NonExistentSymbol999")
	if result != nil {
		t.Errorf("expected nil for unknown symbol, got %+v", result)
	}
}

func TestContext_CaseInsensitive(t *testing.T) {
	g := buildContextGraph()
	result := search.Context(g, ".", "connect") // lowercase
	if result == nil {
		t.Fatal("Context should match case-insensitively")
	}
}

// ---------------------------------------------------------------------------
// Hotspot tests
// ---------------------------------------------------------------------------

func buildHotspotGraph() *graph.Graph {
	return &graph.Graph{
		Symbols: []graph.SymbolNode{
			{ID: "loadGraph", Name: "loadGraph", Kind: graph.KindFunction, File: "cli.go", Line: 10, PackageName: "cli"},
			{ID: "parseFile", Name: "parseFile", Kind: graph.KindFunction, File: "parser.go", Line: 5, PackageName: "parser"},
			{ID: "Run", Name: "Run", Kind: graph.KindFunction, File: "cli.go", Line: 1, PackageName: "cli"},
		},
		Calls: []graph.CallEdge{
			// loadGraph called 4 times
			{CallerName: "runQuery", CalleeRaw: "loadGraph", File: "cli.go", Line: 20},
			{CallerName: "runBuild", CalleeRaw: "loadGraph", File: "cli.go", Line: 30},
			{CallerName: "runCallers", CalleeRaw: "loadGraph", File: "cli.go", Line: 40},
			{CallerName: "runCallees", CalleeRaw: "loadGraph", File: "cli.go", Line: 50},
			// parseFile called 2 times
			{CallerName: "loadGraph", CalleeRaw: "parseFile", File: "cli.go", Line: 60},
			{CallerName: "Build", CalleeRaw: "parseFile", File: "builder.go", Line: 10},
			// Run called 1 time
			{CallerName: "main", CalleeRaw: "Run", File: "main.go", Line: 5},
		},
	}
}

func TestHotspot_RankedByIncomingCalls(t *testing.T) {
	g := buildHotspotGraph()
	results := search.Hotspot(g, 0)

	if len(results) == 0 {
		t.Fatal("expected hotspot results, got none")
	}
	// Results must be sorted descending.
	for i := 1; i < len(results); i++ {
		if results[i].IncomingCalls > results[i-1].IncomingCalls {
			t.Errorf("results not sorted: [%d] %d > [%d] %d",
				i, results[i].IncomingCalls, i-1, results[i-1].IncomingCalls)
		}
	}
}

func TestHotspot_TopLimit(t *testing.T) {
	g := buildHotspotGraph()
	results := search.Hotspot(g, 1)
	if len(results) != 1 {
		t.Errorf("expected 1 result with top=1, got %d", len(results))
	}
}

func TestHotspot_TopZeroReturnsAll(t *testing.T) {
	g := buildHotspotGraph()
	all := search.Hotspot(g, 0)
	capped := search.Hotspot(g, 100)
	if len(all) != len(capped) {
		t.Errorf("top=0 and top=100 should return same count: %d vs %d", len(all), len(capped))
	}
}

func TestHotspot_MostCalledIsFirst(t *testing.T) {
	g := buildHotspotGraph()
	results := search.Hotspot(g, 0)
	if len(results) == 0 {
		t.Fatal("no results")
	}
	// loadGraph has the most incoming calls — should be first.
	if results[0].Name != "loadGraph" {
		t.Errorf("expected loadGraph as top hotspot, got %q", results[0].Name)
	}
	// IncomingCalls should be > 0 and it should be the highest.
	if results[0].IncomingCalls <= 0 {
		t.Errorf("expected positive incoming calls for loadGraph, got %d", results[0].IncomingCalls)
	}
}

func TestHotspot_EmptyGraph(t *testing.T) {
	g := &graph.Graph{}
	results := search.Hotspot(g, 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty graph, got %d", len(results))
	}
}

func TestHotspot_OnlyFunctionsAndMethods(t *testing.T) {
	g := &graph.Graph{
		Symbols: []graph.SymbolNode{
			{Name: "MyStruct", Kind: graph.KindStruct, File: "a.go", Line: 1},
			{Name: "MyInterface", Kind: graph.KindInterface, File: "a.go", Line: 5},
		},
		Calls: []graph.CallEdge{
			{CallerName: "x", CalleeRaw: "MyStruct", File: "b.go", Line: 1},
		},
	}
	results := search.Hotspot(g, 0)
	// Structs and interfaces should not appear as hotspot candidates.
	if len(results) != 0 {
		t.Errorf("expected 0 results (structs/interfaces excluded), got %d: %+v", len(results), results)
	}
}

// ---------------------------------------------------------------------------
// Deps tests
// ---------------------------------------------------------------------------

func buildDepsGraph() *graph.Graph {
	return &graph.Graph{
		Imports: []graph.ImportEdge{
			{FromPackage: "api", ImportPath: "github.com/foo/bar/internal/auth"},
			{FromPackage: "api", ImportPath: "github.com/foo/bar/internal/db"},
			{FromPackage: "auth", ImportPath: "github.com/foo/bar/internal/db"},
			{FromPackage: "auth", ImportPath: "github.com/foo/bar/pkg/crypto"},
			{FromPackage: "db", ImportPath: "database/sql"},
		},
	}
}

func TestDeps_DirectImports(t *testing.T) {
	g := buildDepsGraph()
	result := search.Deps(g, "api", false)
	if result == nil {
		t.Fatal("expected DepsResult for 'api', got nil")
	}
	if result.Package != "api" {
		t.Errorf("expected Package='api', got %q", result.Package)
	}
	if len(result.Direct) != 2 {
		t.Errorf("expected 2 direct imports for 'api', got %d: %v", len(result.Direct), result.Direct)
	}
	if len(result.Transitive) != 0 {
		t.Error("transitive should be empty when not requested")
	}
}

func TestDeps_TransitiveImports(t *testing.T) {
	g := buildDepsGraph()
	result := search.Deps(g, "api", true)
	if result == nil {
		t.Fatal("expected DepsResult for 'api' with transitive, got nil")
	}
	if len(result.Transitive) == 0 {
		t.Error("expected transitive imports, got none")
	}
	// Transitive should include at minimum the two direct imports of 'api'.
	found := make(map[string]bool)
	for _, imp := range result.Transitive {
		found[imp] = true
	}
	if !found["github.com/foo/bar/internal/auth"] {
		t.Errorf("expected auth in transitive deps, got: %v", result.Transitive)
	}
	if !found["github.com/foo/bar/internal/db"] {
		t.Errorf("expected db in transitive deps, got: %v", result.Transitive)
	}
}

func TestDeps_NotFound(t *testing.T) {
	g := buildDepsGraph()
	result := search.Deps(g, "nonexistent", false)
	if result != nil {
		t.Errorf("expected nil for unknown package, got %+v", result)
	}
}

func TestDeps_DirectIsSorted(t *testing.T) {
	g := buildDepsGraph()
	result := search.Deps(g, "api", false)
	if result == nil || len(result.Direct) < 2 {
		t.Skip("need 2+ imports to test sorting")
	}
	for i := 1; i < len(result.Direct); i++ {
		if result.Direct[i] < result.Direct[i-1] {
			t.Errorf("Direct imports not sorted: %v", result.Direct)
		}
	}
}

func TestDeps_EmptyGraph(t *testing.T) {
	g := &graph.Graph{}
	result := search.Deps(g, "api", false)
	if result != nil {
		t.Errorf("expected nil for empty graph, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// Usages tests
// ---------------------------------------------------------------------------

func buildUsagesGraph() *graph.Graph {
	return &graph.Graph{
		Symbols: []graph.SymbolNode{
			// Function with AuthService as param
			{
				ID:        "github.com/foo/bar/internal/handler::CreateUser",
				Kind:      graph.KindFunction,
				Name:      "CreateUser",
				File:      "internal/handler/user.go",
				Line:      10,
				Signature: "func CreateUser(svc AuthService, id string) (*User, error)",
			},
			// Method returning AuthService
			{
				ID:        "github.com/foo/bar/internal/factory::(*Factory).NewAuth",
				Kind:      graph.KindMethod,
				Name:      "NewAuth",
				Receiver:  "Factory",
				File:      "internal/factory/factory.go",
				Line:      20,
				Signature: "func (*Factory).NewAuth() AuthService",
			},
			// Struct with AuthService field
			{
				ID:   "github.com/foo/bar/internal/server::Server",
				Kind: graph.KindStruct,
				Name: "Server",
				File: "internal/server/server.go",
				Line: 5,
				StructFields: []graph.StructField{
					{Name: "auth", Type: "AuthService"},
					{Name: "db", Type: "*sql.DB"},
				},
			},
			// Should NOT match AuthServiceImpl (longer identifier)
			{
				ID:        "github.com/foo/bar/internal/impl::WrapService",
				Kind:      graph.KindFunction,
				Name:      "WrapService",
				File:      "internal/impl/wrap.go",
				Line:      30,
				Signature: "func WrapService(impl AuthServiceImpl) AuthService",
			},
		},
	}
}

func TestUsages_ParamType(t *testing.T) {
	g := buildUsagesGraph()
	results := search.Usages(g, "AuthService")
	// CreateUser (param), NewAuth (return), Server.auth (field), WrapService (return only — param is AuthServiceImpl)
	if len(results) == 0 {
		t.Fatal("expected results for AuthService, got none")
	}
	kinds := make(map[string]int)
	for _, r := range results {
		kinds[r.Kind]++
	}
	if kinds["function"] == 0 && kinds["method"] == 0 {
		t.Error("expected at least one function/method usage")
	}
	if kinds["field"] == 0 {
		t.Error("expected at least one field usage")
	}
}

func TestUsages_NoFalsePositive(t *testing.T) {
	g := buildUsagesGraph()
	results := search.Usages(g, "AuthService")
	// WrapService takes AuthServiceImpl (not AuthService) as param — should only
	// appear as a return type, not a param type.
	for _, r := range results {
		if r.Name == "github.com/foo/bar/internal/impl::WrapService" {
			if r.Detail == "param type" {
				t.Errorf("AuthServiceImpl matched as AuthService param: %+v", r)
			}
		}
	}
}

func TestUsages_CaseInsensitive(t *testing.T) {
	g := buildUsagesGraph()
	upper := search.Usages(g, "AuthService")
	lower := search.Usages(g, "authservice")
	if len(upper) != len(lower) {
		t.Errorf("case sensitivity mismatch: %d vs %d results", len(upper), len(lower))
	}
}

func TestUsages_NotFound(t *testing.T) {
	g := buildUsagesGraph()
	results := search.Usages(g, "NonExistentType")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestUsages_EmptyGraph(t *testing.T) {
	g := &graph.Graph{}
	results := search.Usages(g, "AuthService")
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty graph, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// Literals tests
// ---------------------------------------------------------------------------

func buildLiteralsGraph() *graph.Graph {
	return &graph.Graph{
		Literals: []graph.LiteralEdge{
			{TypeName: "User", Function: "CreateUser", File: "handlers/user.go", Line: 42},
			{TypeName: "User", Function: "UpdateUser", File: "handlers/user.go", Line: 88},
			{TypeName: "Config", Function: "NewConfig", File: "internal/config/config.go", Line: 15},
		},
	}
}

func TestLiterals_Basic(t *testing.T) {
	g := buildLiteralsGraph()
	results := search.Literals(g, "User")
	if len(results) != 2 {
		t.Fatalf("expected 2 User literal sites, got %d", len(results))
	}
}

func TestLiterals_CaseInsensitive(t *testing.T) {
	g := buildLiteralsGraph()
	results := search.Literals(g, "user")
	if len(results) != 2 {
		t.Fatalf("expected 2 results for lowercase 'user', got %d", len(results))
	}
}

func TestLiterals_NotFound(t *testing.T) {
	g := buildLiteralsGraph()
	results := search.Literals(g, "Order")
	if len(results) != 0 {
		t.Errorf("expected 0 results for unknown struct, got %d", len(results))
	}
}

func TestLiterals_EmptyGraph(t *testing.T) {
	g := &graph.Graph{}
	results := search.Literals(g, "User")
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty graph, got %d", len(results))
	}
}

func TestLiterals_DetailField(t *testing.T) {
	g := buildLiteralsGraph()
	results := search.Literals(g, "Config")
	if len(results) != 1 {
		t.Fatalf("expected 1 Config result, got %d", len(results))
	}
	if results[0].Kind != "literal" {
		t.Errorf("expected kind 'literal', got %q", results[0].Kind)
	}
	if results[0].Detail == "" {
		t.Error("expected non-empty Detail")
	}
}

// ---------------------------------------------------------------------------
// Dependents tests
// ---------------------------------------------------------------------------

func TestDependents_Basic(t *testing.T) {
	g := buildDepsGraph()
	// "db" is imported by both "api" and "auth"
	results := search.Dependents(g, "db")
	if len(results) != 2 {
		t.Fatalf("expected 2 dependents for 'db', got %d: %v", len(results), results)
	}
	names := make(map[string]bool)
	for _, r := range results {
		names[r.Name] = true
	}
	if !names["api"] {
		t.Errorf("expected 'api' in dependents, got %v", results)
	}
	if !names["auth"] {
		t.Errorf("expected 'auth' in dependents, got %v", results)
	}
}

func TestDependents_SingleDependent(t *testing.T) {
	g := buildDepsGraph()
	// "auth" is only imported by "api"
	results := search.Dependents(g, "auth")
	if len(results) != 1 {
		t.Fatalf("expected 1 dependent for 'auth', got %d: %v", len(results), results)
	}
	if results[0].Name != "api" {
		t.Errorf("expected 'api', got %q", results[0].Name)
	}
	if results[0].Kind != "package" {
		t.Errorf("expected kind 'package', got %q", results[0].Kind)
	}
}

func TestDependents_FullPath(t *testing.T) {
	g := buildDepsGraph()
	// Full import path should also match
	results := search.Dependents(g, "database/sql")
	if len(results) != 1 {
		t.Fatalf("expected 1 dependent for 'database/sql', got %d: %v", len(results), results)
	}
	if results[0].Name != "db" {
		t.Errorf("expected 'db', got %q", results[0].Name)
	}
}

func TestDependents_NotFound(t *testing.T) {
	g := buildDepsGraph()
	results := search.Dependents(g, "nonexistent")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestDependents_EmptyGraph(t *testing.T) {
	g := &graph.Graph{}
	results := search.Dependents(g, "auth")
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty graph, got %d", len(results))
	}
}

func TestDependents_NoDuplicates(t *testing.T) {
	// Same package importing via multiple files should appear once.
	g := &graph.Graph{
		Imports: []graph.ImportEdge{
			{FromPackage: "api", FromFile: "api/a.go", ImportPath: "github.com/foo/bar/internal/auth"},
			{FromPackage: "api", FromFile: "api/b.go", ImportPath: "github.com/foo/bar/internal/auth"},
		},
	}
	results := search.Dependents(g, "auth")
	if len(results) != 1 {
		t.Errorf("expected 1 deduplicated result, got %d: %v", len(results), results)
	}
}

// ---------------------------------------------------------------------------
// Changes tests
// ---------------------------------------------------------------------------

// writeTempGoFile writes a Go source file into dir and optionally adjusts its mtime.
func writeTempGoFile(t *testing.T, dir, filename, content string, mtime time.Time) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", filename, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("failed to set mtime on %s: %v", filename, err)
	}
	return path
}

func TestChanges_NoChanges(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	graphTime := now.Add(time.Hour) // graph is NEWER than files

	writeTempGoFile(t, dir, "main.go", "package main\nfunc main() {}\n", now.Add(-time.Hour))

	g := &graph.Graph{
		GeneratedAt: graphTime,
		Symbols: []graph.SymbolNode{
			{Name: "main", Kind: graph.KindFunction, File: filepath.Join(dir, "main.go"), Line: 2},
		},
	}

	result := search.Changes(g, dir)
	if result == nil {
		t.Fatal("expected ChangesResult, got nil")
	}
	if len(result.ChangedFiles) != 0 {
		t.Errorf("expected no changed files, got: %v", result.ChangedFiles)
	}
	if len(result.Symbols) != 0 {
		t.Errorf("expected no changed symbols, got: %v", result.Symbols)
	}
}

func TestChanges_ModifiedSymbol(t *testing.T) {
	dir := t.TempDir()
	graphTime := time.Now().Add(-time.Hour) // graph is OLD

	writeTempGoFile(t, dir, "handler.go",
		"package main\nfunc HandleRequest() {}\n",
		time.Now()) // file is newer than graph

	g := &graph.Graph{
		GeneratedAt: graphTime,
		Symbols: []graph.SymbolNode{
			{Name: "HandleRequest", Kind: graph.KindFunction,
				File: filepath.Join(dir, "handler.go"), Line: 2},
		},
	}

	result := search.Changes(g, dir)
	if len(result.ChangedFiles) == 0 {
		t.Fatal("expected at least one changed file")
	}

	foundModified := false
	for _, sym := range result.Symbols {
		if sym.Name == "HandleRequest" && sym.Status == search.ChangeModified {
			foundModified = true
		}
	}
	if !foundModified {
		t.Errorf("expected HandleRequest to be reported as modified, got: %+v", result.Symbols)
	}
}

func TestChanges_NewSymbol(t *testing.T) {
	dir := t.TempDir()
	graphTime := time.Now().Add(-time.Hour)

	// File is newer and contains a function NOT in the graph.
	writeTempGoFile(t, dir, "new_feature.go",
		"package main\nfunc NewFeature() {}\n",
		time.Now())

	g := &graph.Graph{
		GeneratedAt: graphTime,
		Symbols:     []graph.SymbolNode{}, // graph doesn't know about NewFeature
	}

	result := search.Changes(g, dir)
	foundNew := false
	for _, sym := range result.Symbols {
		if sym.Name == "NewFeature" && sym.Status == search.ChangeNew {
			foundNew = true
		}
	}
	if !foundNew {
		t.Errorf("expected NewFeature to be reported as new, got: %+v", result.Symbols)
	}
}

func TestChanges_DeletedSymbol(t *testing.T) {
	dir := t.TempDir()
	graphTime := time.Now().Add(-time.Hour)

	// Graph references a file that does NOT exist on disk (use a non-existent path).
	nonExistentFile := filepath.Join(dir, "old_handler.go")

	g := &graph.Graph{
		GeneratedAt: graphTime,
		Symbols: []graph.SymbolNode{
			{Name: "OldHandler", Kind: graph.KindFunction,
				File: nonExistentFile, Line: 5},
		},
	}

	result := search.Changes(g, dir)
	foundDeleted := false
	for _, sym := range result.Symbols {
		if sym.Name == "OldHandler" && sym.Status == search.ChangeDeleted {
			foundDeleted = true
		}
	}
	if !foundDeleted {
		t.Errorf("expected OldHandler to be reported as deleted, got: %+v", result.Symbols)
	}
}

func TestChanges_GraphAgePreserved(t *testing.T) {
	dir := t.TempDir()
	graphTime := time.Now().Add(-2 * time.Hour)
	g := &graph.Graph{GeneratedAt: graphTime}

	result := search.Changes(g, dir)
	if !result.GraphAge.Equal(graphTime) {
		t.Errorf("expected GraphAge=%v, got %v", graphTime, result.GraphAge)
	}
}

func TestDeps_CaseInsensitive(t *testing.T) {
	g := buildDepsGraph()
	result := search.Deps(g, "API", false) // uppercase
	if result == nil {
		t.Error("Deps should match package name case-insensitively")
	}
}

func TestChanges_ChangedFilesAreSorted(t *testing.T) {
	dir := t.TempDir()
	graphTime := time.Now().Add(-time.Hour)
	future := time.Now()

	writeTempGoFile(t, dir, "z_file.go", "package main\nfunc Z() {}\n", future)
	writeTempGoFile(t, dir, "a_file.go", "package main\nfunc A() {}\n", future)

	g := &graph.Graph{GeneratedAt: graphTime}
	result := search.Changes(g, dir)

	for i := 1; i < len(result.ChangedFiles); i++ {
		if result.ChangedFiles[i] < result.ChangedFiles[i-1] {
			t.Errorf("ChangedFiles not sorted: %v", result.ChangedFiles)
		}
	}
}

func TestContext_CallerAndCalleeContent(t *testing.T) {
	g := buildContextGraph()
	result := search.Context(g, ".", "Connect")
	if result == nil {
		t.Fatal("expected result")
	}
	// Callers: main calls Connect
	found := false
	for _, r := range result.Callers {
		if strings.Contains(r.Detail, "Connect") || r.Name == "main" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected main in callers, got: %+v", result.Callers)
	}
	// Callees: Connect calls sql.Open
	foundCallee := false
	for _, r := range result.Callees {
		if strings.Contains(r.Name, "sql") || strings.Contains(r.Detail, "sql") {
			foundCallee = true
		}
	}
	if !foundCallee {
		t.Errorf("expected sql.Open in callees, got: %+v", result.Callees)
	}
}
