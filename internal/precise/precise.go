package precise

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Enrich applies type-checked precision to the graph.
// It loads the project via go/packages, finds exact interface implementers,
// and uses Class Hierarchy Analysis (CHA) to add precise dynamic dispatch call edges.
func Enrich(absRoot string, g *graph.Graph) error {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Dir: absRoot,
	}

	// Load all packages
	initial, err := packages.Load(cfg, "./...")
	if err != nil {
		return fmt.Errorf("packages.Load failed: %w", err)
	}

	// Build SSA
	prog, _ := ssautil.AllPackages(initial, 0)
	prog.Build()

	// 1. Precise Interface Satisfaction
	var interfaces []*types.Interface
	var interfaceNames []string
	var concretes []types.Type
	var concreteNames []string

	// Collect all types across all loaded packages
	for _, pkg := range initial {
		if pkg.Types == nil || pkg.Types.Scope() == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if typeName, ok := obj.(*types.TypeName); ok {
				t := typeName.Type()
				if t == nil {
					continue
				}

				// Keep track of interface types
				if iface, isIface := t.Underlying().(*types.Interface); isIface {
					// We only care about interfaces with methods
					if iface.NumMethods() > 0 {
						interfaces = append(interfaces, iface)
						interfaceNames = append(interfaceNames, obj.Name())
					}
					continue
				}

				// Otherwise it's a concrete type
				concretes = append(concretes, t)
				concreteNames = append(concreteNames, obj.Name())
			}
		}
	}

	// Compute precise Implements edges
	for i, iface := range interfaces {
		for j, conc := range concretes {
			// Check value receiver
			if types.Implements(conc, iface) {
				g.Implements = append(g.Implements, graph.ImplementsEdge{
					Interface: interfaceNames[i],
					Concrete:  concreteNames[j],
				})
				continue
			}
			// Check pointer receiver
			ptr := types.NewPointer(conc)
			if types.Implements(ptr, iface) {
				g.Implements = append(g.Implements, graph.ImplementsEdge{
					Interface: interfaceNames[i],
					Concrete:  concreteNames[j],
				})
			}
		}
	}

	// 2. Precise Call Graph via CHA
	cg := cha.CallGraph(prog)
	if cg != nil {
		// Do not clear g.Calls (g.Calls = nil). We want to be strictly additive.
		// Instead, map the existing AST calls so we don't duplicate them.
		// Both the caller and callee are normalized with cleanName so the keys
		// match what CHA produces (CHA strips package qualifiers; AST does not).
		seenEdges := make(map[string]bool)
		for _, edge := range g.Calls {
			key := fmt.Sprintf("%s->%s@%s:%d", cleanName(edge.CallerName), cleanName(edge.CalleeRaw), edge.File, edge.Line)
			seenEdges[key] = true
		}

		for _, node := range cg.Nodes {
			if node.Func == nil || node.Func.Pkg == nil {
				continue
			}
			callerName := cleanName(node.Func.Name())

			for _, edge := range node.Out {
				if edge.Callee == nil || edge.Callee.Func == nil || edge.Callee.Func.Pkg == nil {
					continue
				}
				calleeName := cleanName(edge.Callee.Func.Name())

				if edge.Site == nil {
					continue
				}

				pos := prog.Fset.Position(edge.Site.Pos())
				if pos.Filename == "" || !strings.HasPrefix(pos.Filename, absRoot) {
					continue
				}
				// Normalize to a repo-relative path so the dedup key matches the
				// AST-sourced edge keys (which use relative paths). Without this,
				// every AST call edge gets a CHA duplicate because the keys never
				// match (absolute vs. relative path for the same call site).
				relFile := strings.TrimPrefix(pos.Filename, absRoot+"/")

				// Create a unique key to prevent duplicate call edges
				key := fmt.Sprintf("%s->%s@%s:%d", callerName, calleeName, relFile, pos.Line)
				if seenEdges[key] {
					continue
				}
				seenEdges[key] = true

				// Append to graph (Additive enrichment)
				g.Calls = append(g.Calls, graph.CallEdge{
					CallerName: callerName,
					CalleeRaw:  calleeName,
					File:       relFile,
					Line:       pos.Line,
				})
			}
		}
	}

	return nil
}

// cleanName strips package paths or pointer indicators from SSA names to match AST names.
func cleanName(name string) string {
	name = strings.TrimPrefix(name, "*")
	if idx := strings.LastIndex(name, "."); idx != -1 {
		return name[idx+1:]
	}
	return name
}
