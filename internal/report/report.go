// Package report generates the human-readable GRAPH_REPORT.md file.
package report

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// Generate produces the markdown report content from the given graph.
func Generate(g *graph.Graph) string {
	var sb strings.Builder
	writeHeader(&sb, g)
	writeSummary(&sb, g)
	writeEntryPoints(&sb, g)
	writePackages(&sb, g)
	writeImportantFiles(&sb, g)
	writeImportantSymbols(&sb, g)
	writeEnvVars(&sb, g)
	writeImports(&sb, g)
	writeUsageInstructions(&sb)
	return sb.String()
}

func writeHeader(sb *strings.Builder, g *graph.Graph) {
	sb.WriteString("# GoGraph Report\n\n")
	fmt.Fprintf(sb, "**Root:** `%s`  \n", g.Root)
	fmt.Fprintf(sb, "**Generated:** %s  \n", g.GeneratedAt.Format("2006-01-02 15:04:05 MST"))
	sb.WriteString("\n---\n\n")
}

func writeSummary(sb *strings.Builder, g *graph.Graph) {
	sb.WriteString("## 1. Summary\n\n")
	funcs, methods, structs, ifaces := symbolCounts(g)
	fmt.Fprintf(sb, "| Metric | Count |\n|--------|-------|\n")
	fmt.Fprintf(sb, "| Packages | %d |\n", len(g.Packages))
	fmt.Fprintf(sb, "| Files | %d |\n", len(g.Files))
	fmt.Fprintf(sb, "| Symbols | %d |\n", len(g.Symbols))
	fmt.Fprintf(sb, "| Functions | %d |\n", funcs)
	fmt.Fprintf(sb, "| Methods | %d |\n", methods)
	fmt.Fprintf(sb, "| Structs | %d |\n", structs)
	fmt.Fprintf(sb, "| Interfaces | %d |\n", ifaces)
	fmt.Fprintf(sb, "| Call edges | %d |\n", len(g.Calls))
	fmt.Fprintf(sb, "| Env var reads | %d |\n", len(g.EnvReads))
	sb.WriteString("\n")
}

func writeEntryPoints(sb *strings.Builder, g *graph.Graph) {
	sb.WriteString("## 2. Likely Entry Points\n\n")
	found := false
	for _, f := range g.Files {
		base := filepath.Base(f.Path)
		dir := filepath.Dir(f.Path)
		dirBase := filepath.Base(dir)
		parentBase := filepath.Base(filepath.Dir(dir))
		isCmdMain := base == "main.go" && (dirBase == "main" || parentBase == "cmd" || strings.HasPrefix(f.Path, "cmd/"))
		isMainPkg := f.PackageName == "main"
		if isCmdMain || isMainPkg {
			fmt.Fprintf(sb, "- `%s` (package `%s`)\n", f.Path, f.PackageName)
			found = true
		}
	}
	if !found {
		sb.WriteString("_No `main` package files detected._\n")
	}
	sb.WriteString("\n")
}

func writePackages(sb *strings.Builder, g *graph.Graph) {
	sb.WriteString("## 3. Packages\n\n")
	sb.WriteString("| Package | Dir | Files | Symbols |\n|---------|-----|-------|---------|\n")
	symCount := make(map[string]int)
	for _, s := range g.Symbols {
		symCount[s.PackageName]++
	}
	for _, pkg := range g.Packages {
		fmt.Fprintf(sb, "| `%s` | `%s` | %d | %d |\n",
			pkg.Name, pkg.Dir, len(pkg.Files), symCount[pkg.Name])
	}
	sb.WriteString("\n")
}

func writeImportantFiles(sb *strings.Builder, g *graph.Graph) {
	sb.WriteString("## 4. Important Files (top 20 by symbol+call density)\n\n")
	type scored struct {
		path  string
		score int
	}
	symPerFile := make(map[string]int)
	for _, s := range g.Symbols {
		symPerFile[s.File]++
	}
	callPerFile := make(map[string]int)
	for _, c := range g.Calls {
		callPerFile[c.File]++
	}
	scoredFiles := make([]scored, 0, len(g.Files))
	for _, f := range g.Files {
		scoredFiles = append(scoredFiles, scored{path: f.Path, score: symPerFile[f.Path] + callPerFile[f.Path]})
	}
	sort.Slice(scoredFiles, func(i, j int) bool {
		if scoredFiles[i].score != scoredFiles[j].score {
			return scoredFiles[i].score > scoredFiles[j].score
		}
		return scoredFiles[i].path < scoredFiles[j].path
	})
	if len(scoredFiles) > 20 {
		scoredFiles = scoredFiles[:20]
	}
	sb.WriteString("| File | Symbols | Calls |\n|------|---------|-------|\n")
	for _, sf := range scoredFiles {
		fmt.Fprintf(sb, "| `%s` | %d | %d |\n", sf.path, symPerFile[sf.path], callPerFile[sf.path])
	}
	sb.WriteString("\n")
}

func writeImportantSymbols(sb *strings.Builder, g *graph.Graph) {
	sb.WriteString("## 5. Important Symbols (top 30 by outgoing calls)\n\n")
	type scored struct {
		sym   graph.SymbolNode
		calls int
	}
	callsPerSym := make(map[string]int)
	for _, c := range g.Calls {
		callsPerSym[c.CallerSymbolID]++
	}
	var funcs []scored
	for _, s := range g.Symbols {
		if s.Kind == graph.KindFunction || s.Kind == graph.KindMethod {
			funcs = append(funcs, scored{sym: s, calls: callsPerSym[s.ID]})
		}
	}
	sort.Slice(funcs, func(i, j int) bool {
		if funcs[i].calls != funcs[j].calls {
			return funcs[i].calls > funcs[j].calls
		}
		return funcs[i].sym.ID < funcs[j].sym.ID
	})
	if len(funcs) > 30 {
		funcs = funcs[:30]
	}
	sb.WriteString("| Symbol | Kind | File | Line | Calls out |\n|--------|------|------|------|-----------|\n")
	for _, sf := range funcs {
		name := sf.sym.Name
		if sf.sym.Receiver != "" {
			name = fmt.Sprintf("(%s).%s", sf.sym.Receiver, sf.sym.Name)
		}
		fmt.Fprintf(sb, "| `%s` | %s | `%s` | %d | %d |\n",
			name, sf.sym.Kind, sf.sym.File, sf.sym.Line, sf.calls)
	}
	sb.WriteString("\n")
}

func writeEnvVars(sb *strings.Builder, g *graph.Graph) {
	sb.WriteString("## 6. Environment Variables\n\n")
	if len(g.EnvReads) == 0 {
		sb.WriteString("_None detected._\n\n")
		return
	}
	sb.WriteString("| Key | Accessor | File | Line | Function |\n|-----|----------|------|------|----------|\n")
	for _, ev := range g.EnvReads {
		fmt.Fprintf(sb, "| `%s` | `%s` | `%s` | %d | `%s` |\n",
			ev.Key, ev.Accessor, ev.File, ev.Line, ev.Function)
	}
	sb.WriteString("\n")
}

func writeImports(sb *strings.Builder, g *graph.Graph) {
	sb.WriteString("## 7. Package Imports\n\n")
	type pkgImport struct{ pkg, path string }
	seen := make(map[pkgImport]bool)
	var pairs []pkgImport
	for _, imp := range g.Imports {
		pi := pkgImport{imp.FromPackage, imp.ImportPath}
		if !seen[pi] {
			seen[pi] = true
			pairs = append(pairs, pi)
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].pkg != pairs[j].pkg {
			return pairs[i].pkg < pairs[j].pkg
		}
		return pairs[i].path < pairs[j].path
	})
	byPkg := make(map[string][]string)
	for _, pi := range pairs {
		byPkg[pi.pkg] = append(byPkg[pi.pkg], pi.path)
	}
	pkgs := make([]string, 0, len(byPkg))
	for k := range byPkg {
		pkgs = append(pkgs, k)
	}
	sort.Strings(pkgs)
	sb.WriteString("| Package | Imports |\n|---------|--------|\n")
	for _, pkg := range pkgs {
		imps := byPkg[pkg]
		fmt.Fprintf(sb, "| `%s` | %s |\n", pkg, strings.Join(wrap(imps, "`"), ", "))
	}
	sb.WriteString("\n")
}

func writeUsageInstructions(sb *strings.Builder) {
	sb.WriteString("## 8. AI Assistant / Coding Agent Usage\n\n")
	sb.WriteString("Add the following to your AI assistant's system prompt, project instructions, or context file\n")
	sb.WriteString("(works with any tool: Cursor, GitHub Copilot, Claude, Gemini, Codeium, etc.):\n\n")
	sb.WriteString("> Before answering architecture, dependency, or 'where is X?' questions about this\n")
	sb.WriteString("> repository, read `.gograph/GRAPH_REPORT.md` first. Use it as the repo map before\n")
	sb.WriteString("> searching raw files. For symbol lookup, use `gograph query \"<term>\"`,\n")
	sb.WriteString("> `gograph callers \"<function>\"`, and `gograph callees \"<function>\"`. After\n")
	sb.WriteString("> structural code changes, run `gograph build .`.\n\n")
}

func symbolCounts(g *graph.Graph) (funcs, methods, structs, ifaces int) {
	for _, s := range g.Symbols {
		switch s.Kind {
		case graph.KindFunction:
			funcs++
		case graph.KindMethod:
			methods++
		case graph.KindStruct:
			structs++
		case graph.KindInterface:
			ifaces++
		}
	}
	return
}

func wrap(ss []string, delim string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = delim + s + delim
	}
	return out
}
