package search

import (
	"reflect"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

type APISymbolDrift struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // "changed"
	Details string `json:"details,omitempty"`
}

type APIRouteDrift struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Type    string `json:"type"` // "added", "removed", "changed"
	Details string `json:"details,omitempty"`
}

type APIDriftResult struct {
	Baseline string `json:"baseline"`

	BreakingGoAPI    bool   `json:"breaking_go_api"`
	BreakingHTTPAPI  string `json:"breaking_http_api"` // "yes", "likely", "no"
	StaleMocksLikely bool   `json:"stale_mocks_likely"`

	ExportedSymbols struct {
		Added   []string         `json:"added"`
		Removed []string         `json:"removed"`
		Changed []APISymbolDrift `json:"changed"`
	} `json:"exported_symbols"`

	Interfaces struct {
		Added   []string         `json:"added"`
		Removed []string         `json:"removed"`
		Changed []APISymbolDrift `json:"changed"`
	} `json:"interfaces"`

	Structs struct {
		Added   []string         `json:"added"`
		Removed []string         `json:"removed"`
		Changed []APISymbolDrift `json:"changed"`
	} `json:"structs"`

	Routes struct {
		Added   []string        `json:"added"`
		Removed []string        `json:"removed"`
		Changed []APIRouteDrift `json:"changed"`
	} `json:"routes"`

	AffectedTests []string `json:"affected_tests"`
	AffectedMocks []string `json:"affected_mocks"`
	Findings      []string `json:"findings"`
}

func APIDrift(baseline, current *graph.Graph, baselineRef string) *APIDriftResult {
	res := &APIDriftResult{
		Baseline:        baselineRef,
		BreakingHTTPAPI: "no",
	}

	oldSymbols := make(map[string]graph.SymbolNode)
	for _, s := range baseline.Symbols {
		oldSymbols[s.ID] = s
	}

	newSymbols := make(map[string]graph.SymbolNode)
	for _, s := range current.Symbols {
		newSymbols[s.ID] = s
	}

	// Compare Symbols
	for id, sOld := range oldSymbols {
		if !isExported(sOld.Name) && !isExported(sOld.Receiver) {
			continue
		}
		sNew, exists := newSymbols[id]
		if !exists {
			// Removed
			switch sOld.Kind {
			case graph.KindInterface:
				res.Interfaces.Removed = append(res.Interfaces.Removed, sOld.Name)
				res.BreakingGoAPI = true
			case graph.KindStruct:
				res.Structs.Removed = append(res.Structs.Removed, sOld.Name)
				res.BreakingGoAPI = true
			default:
				res.ExportedSymbols.Removed = append(res.ExportedSymbols.Removed, fmtSymbolName(sOld))
				res.BreakingGoAPI = true
			}
			continue
		}

		// Changed?
		switch sOld.Kind {
		case graph.KindInterface:
			// Compare InterfaceMethods
			changes := compareInterface(sOld, sNew)
			if len(changes) > 0 {
				res.Interfaces.Changed = append(res.Interfaces.Changed, APISymbolDrift{
					Name:    sOld.Name,
					Type:    "changed",
					Details: strings.Join(changes, ", "),
				})
				res.BreakingGoAPI = true
				res.StaleMocksLikely = true
			}
		case graph.KindStruct:
			// Compare StructFields
			changes, breaking := compareStruct(sOld, sNew)
			if len(changes) > 0 {
				res.Structs.Changed = append(res.Structs.Changed, APISymbolDrift{
					Name:    sOld.Name,
					Type:    "changed",
					Details: strings.Join(changes, ", "),
				})
				if breaking {
					res.BreakingHTTPAPI = "likely"
				}
			}
		case graph.KindFunction, graph.KindMethod:
			if sOld.Signature != sNew.Signature {
				res.ExportedSymbols.Changed = append(res.ExportedSymbols.Changed, APISymbolDrift{
					Name:    fmtSymbolName(sOld),
					Type:    "changed",
					Details: sOld.Signature + " -> " + sNew.Signature,
				})
				res.BreakingGoAPI = true
			}
		}
	}

	for id, sNew := range newSymbols {
		if !isExported(sNew.Name) && !isExported(sNew.Receiver) {
			continue
		}
		if _, exists := oldSymbols[id]; !exists {
			// Added
			switch sNew.Kind {
			case graph.KindInterface:
				res.Interfaces.Added = append(res.Interfaces.Added, sNew.Name)
			case graph.KindStruct:
				res.Structs.Added = append(res.Structs.Added, sNew.Name)
			default:
				res.ExportedSymbols.Added = append(res.ExportedSymbols.Added, fmtSymbolName(sNew))
			}
		}
	}

	// Compare Routes
	oldRoutes := make(map[string]graph.HTTPRoute)
	for _, r := range baseline.Routes {
		key := r.Method + " " + r.Path
		oldRoutes[key] = r
	}
	newRoutes := make(map[string]graph.HTTPRoute)
	for _, r := range current.Routes {
		key := r.Method + " " + r.Path
		newRoutes[key] = r
	}

	for key, rOld := range oldRoutes {
		rNew, exists := newRoutes[key]
		if !exists {
			res.Routes.Removed = append(res.Routes.Removed, key)
			res.BreakingHTTPAPI = "yes"
		} else if rOld.Handler != rNew.Handler {
			res.Routes.Changed = append(res.Routes.Changed, APIRouteDrift{
				Method:  rOld.Method,
				Path:    rOld.Path,
				Type:    "changed",
				Details: "Existing route handler changed; HTTP behavior may have changed. (" + rOld.Handler + " -> " + rNew.Handler + ")",
			})
			if res.BreakingHTTPAPI == "no" {
				res.BreakingHTTPAPI = "likely"
			}
		}
	}

	for key := range newRoutes {
		if _, exists := oldRoutes[key]; !exists {
			res.Routes.Added = append(res.Routes.Added, key)
		}
	}

	// Find affected tests/mocks
	affectedMocksMap := make(map[string]bool)
	affectedTestsMap := make(map[string]bool)

	// Collect affected tests
	var changedSymbols []string
	for _, c := range res.ExportedSymbols.Changed {
		changedSymbols = append(changedSymbols, c.Name)
	}
	for _, c := range res.Interfaces.Changed {
		changedSymbols = append(changedSymbols, c.Name)
	}
	for _, c := range res.Structs.Changed {
		changedSymbols = append(changedSymbols, c.Name)
	}
	for _, c := range res.Routes.Changed {
		// Try to find tests for the new handler
		parts := strings.Split(c.Details, " -> ")
		if len(parts) == 2 {
			changedSymbols = append(changedSymbols, strings.TrimSuffix(parts[1], ")"))
		}
	}
	for _, sym := range changedSymbols {
		for _, test := range Tests(current, sym) {
			affectedTestsMap[test.Name] = true
		}
	}

	for _, idf := range res.Interfaces.Changed {
		implementers := Implementers(current, idf.Name)
		for _, imp := range implementers {
			isMock := strings.Contains(strings.ToLower(imp.Name), "mock") || strings.Contains(strings.ToLower(imp.File), "mock")
			if isMock {
				affectedMocksMap[imp.Name+" (likely stale)"] = true
			} else {
				affectedMocksMap[imp.Name+" (affected)"] = true
			}
		}
	}

	for idf := range affectedMocksMap {
		res.AffectedMocks = append(res.AffectedMocks, idf)
	}
	for idf := range affectedTestsMap {
		res.AffectedTests = append(res.AffectedTests, idf)
	}

	return res
}

func isExported(name string) bool {
	if name == "" {
		return false
	}
	if name[0] >= 'A' && name[0] <= 'Z' {
		return true
	}
	// Also check if it's a pointer type e.g. "*MyStruct"
	if strings.HasPrefix(name, "*") && len(name) > 1 {
		return name[1] >= 'A' && name[1] <= 'Z'
	}
	return false
}

func fmtSymbolName(s graph.SymbolNode) string {
	if s.Receiver != "" {
		return "(" + s.Receiver + ")." + s.Name
	}
	return s.Name
}

func compareInterface(old, new graph.SymbolNode) []string {
	var changes []string
	for method, oldSig := range old.InterfaceMethods {
		newSig, exists := new.InterfaceMethods[method]
		if !exists {
			changes = append(changes, "removed method "+method)
		} else if oldSig != newSig {
			changes = append(changes, "changed method "+method+" ("+oldSig+" -> "+newSig+")")
		}
	}
	for method := range new.InterfaceMethods {
		if _, exists := old.InterfaceMethods[method]; !exists {
			changes = append(changes, "added method "+method)
		}
	}
	return changes
}

func extractContractName(f graph.StructField) (string, bool) {
	tagBody := strings.Trim(f.Tag, "`")
	if tagBody == "" {
		return f.Name + " (inferred)", false
	}
	jsonTag := reflect.StructTag(tagBody).Get("json")
	if jsonTag == "-" {
		return "", true
	}
	if jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		if parts[0] != "" {
			return parts[0], false
		}
	}
	return f.Name + " (inferred)", false
}

func compareStruct(old, new graph.SymbolNode) (changes []string, breaking bool) {
	oldFields := make(map[string]graph.StructField)
	for _, f := range old.StructFields {
		name, ignored := extractContractName(f)
		if !ignored {
			oldFields[name] = f
		}
	}
	newFields := make(map[string]graph.StructField)
	for _, f := range new.StructFields {
		name, ignored := extractContractName(f)
		if !ignored {
			newFields[name] = f
		}
	}

	for name, oldF := range oldFields {
		newF, exists := newFields[name]
		if !exists {
			changes = append(changes, "removed field "+name)
			breaking = true
		} else {
			if oldF.Type != newF.Type {
				changes = append(changes, "changed field "+name+" type: "+oldF.Type+" -> "+newF.Type)
				breaking = true
			}
			if oldF.Tag != newF.Tag {
				changes = append(changes, "changed field "+name+" tag: `"+oldF.Tag+"` -> `"+newF.Tag+"`")
			}
		}
	}
	for name := range newFields {
		if _, exists := oldFields[name]; !exists {
			changes = append(changes, "added field "+name+"; compatibility depends on validation/runtime behavior")
		}
	}
	return changes, breaking
}
