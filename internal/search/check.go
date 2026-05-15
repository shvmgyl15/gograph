package search

import (
	"fmt"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

type CheckLevel string

const (
	CheckOff   CheckLevel = "off"
	CheckWarn  CheckLevel = "warn"
	CheckError CheckLevel = "error"
)

type CheckStatus string

const (
	CheckPassed  CheckStatus = "passed"
	CheckWarning CheckStatus = "warning"
	CheckFailed  CheckStatus = "failed"
)

type CheckFinding struct {
	Check    string         `json:"check"`
	Level    string         `json:"level"`
	Message  string         `json:"message"`
	File     string         `json:"file,omitempty"`
	Line     int            `json:"line,omitempty"`
	Symbol   string         `json:"symbol,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type CheckSkipped struct {
	Check  string `json:"check"`
	Reason string `json:"reason"`
}

type CheckSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Skipped  int `json:"skipped"`
}

type CheckReport struct {
	Status      string         `json:"status"`
	Summary     CheckSummary   `json:"summary"`
	Findings    []CheckFinding `json:"findings"`
	Skipped     []CheckSkipped `json:"skipped"`
	Limitations []string       `json:"limitations"`
}

type CheckConfig struct {
	Checks           map[string]any `json:"checks"`
	BoundariesConfig string         `json:"boundaries_config"`
	Baseline         string         `json:"baseline"`
}

type CheckParams struct {
	CurrentGraph  *graph.Graph
	BaselineGraph *graph.Graph // Can be nil
	Config        *CheckConfig
	SinceRef      string
	Uncommitted   bool
	RootDir       string
}

func parseLevel(val any) (CheckLevel, int) {
	switch v := val.(type) {
	case string:
		return CheckLevel(v), 0
	case map[string]any:
		lvl, _ := v["level"].(string)
		valFloat, _ := v["value"].(float64)
		return CheckLevel(lvl), int(valFloat)
	}
	return CheckOff, 0
}

func RunChecks(p *CheckParams) (*CheckReport, error) {
	report := &CheckReport{
		Findings: []CheckFinding{},
		Skipped:  []CheckSkipped{},
		Limitations: []string{
			"gograph check is static analysis only.",
			"It does not execute target repository code.",
			"It does not prove runtime behavior.",
		},
	}

	validChecks := map[string]bool{
		"boundaries":                       true,
		"api_drift":                        true,
		"require_tests_for_changed_routes": true,
		"require_tests_for_changed_exported_symbols": true,
		"max_arity":      true,
		"max_complexity": true,
		"new_globals":    true,
	}

	for checkName := range p.Config.Checks {
		if !validChecks[checkName] {
			return nil, fmt.Errorf("unknown check name in config: %s", checkName)
		}
	}

	addFinding := func(check, msg, file, symbol string, line int, lvl CheckLevel, meta map[string]any) {
		report.Findings = append(report.Findings, CheckFinding{
			Check:    check,
			Level:    string(lvl),
			Message:  msg,
			File:     file,
			Line:     line,
			Symbol:   symbol,
			Metadata: meta,
		})
		if lvl == CheckError {
			report.Summary.Errors++
		} else if lvl == CheckWarn {
			report.Summary.Warnings++
		}
	}

	addSkipped := func(check, reason string) {
		report.Skipped = append(report.Skipped, CheckSkipped{Check: check, Reason: reason})
		report.Summary.Skipped++
	}

	hasBaselineOrUncommitted := p.BaselineGraph != nil || p.Uncommitted

	// Helper to extract affected symbols
	var changedSymbols []string
	if hasBaselineOrUncommitted {
		if p.Uncommitted {
			changes := Changes(p.CurrentGraph, p.RootDir)
			for _, s := range changes.Symbols {
				changedSymbols = append(changedSymbols, s.Name)
			}
		} else if p.BaselineGraph != nil {
			// This is a naive way to get changed symbols from baseline vs current.
			// Ideally we use a better primitive, but we can just use search.Changes logic or just check symbols that differ.
			// Let's just collect symbols from CurrentGraph that aren't exactly in BaselineGraph.
			// To keep it simple, we use APIDrift result for api_drift, and for others we just do a rough diff.
			baselineSyms := make(map[string]graph.SymbolNode)
			for _, s := range p.BaselineGraph.Symbols {
				baselineSyms[s.Name] = s
			}
			for _, s := range p.CurrentGraph.Symbols {
				if bs, ok := baselineSyms[s.Name]; !ok || bs.Signature != s.Signature {
					changedSymbols = append(changedSymbols, s.Name)
				}
			}
		}
	}

	// 1. boundaries
	if lvl, _ := parseLevel(p.Config.Checks["boundaries"]); lvl != CheckOff {
		if p.Config.BoundariesConfig == "" {
			addSkipped("boundaries", "no boundaries config exists")
		} else {
			results, err := Boundaries(p.CurrentGraph, p.Config.BoundariesConfig)
			if err != nil {
				// config file doesn't exist or is invalid
				addSkipped("boundaries", fmt.Sprintf("failed to load boundaries config: %v", err))
			} else {
				for _, r := range results {
					meta := map[string]any{"rule": r.Kind} // r.Kind stores the rule type, e.g., "invalid_import"
					addFinding("boundaries", r.Name, r.File, "", r.Line, lvl, meta)
				}
			}
		}
	}

	// 2. api_drift
	if lvl, _ := parseLevel(p.Config.Checks["api_drift"]); lvl != CheckOff {
		if p.BaselineGraph == nil {
			addSkipped("api_drift", "no --since baseline provided")
		} else {
			driftRes := APIDrift(p.BaselineGraph, p.CurrentGraph, p.SinceRef)
			if driftRes.BreakingHTTPAPI == "yes" {
				addFinding("api_drift", "Breaking HTTP API drift detected", "", "", 0, lvl, nil)
			}
			if driftRes.BreakingGoAPI {
				addFinding("api_drift", "Breaking internal contract drift detected", "", "", 0, lvl, nil)
			}
		}
	}

	// 3. require_tests_for_changed_routes
	if lvl, _ := parseLevel(p.Config.Checks["require_tests_for_changed_routes"]); lvl != CheckOff {
		if !hasBaselineOrUncommitted {
			addSkipped("require_tests_for_changed_routes", "no --since or --uncommitted provided")
		} else {
			routesMap := make(map[string]Result)
			for _, r := range Routes(p.CurrentGraph) {
				routesMap[r.Name] = r
			}
			for _, sym := range changedSymbols {
				if route, ok := routesMap[sym]; ok {
					tests := Tests(p.CurrentGraph, sym)
					if len(tests) == 0 {
						addFinding("require_tests_for_changed_routes", fmt.Sprintf("Changed route handler %s has no mapped tests", sym), route.File, sym, route.Line, lvl, nil)
					}
				}
			}
		}
	}

	// 4. require_tests_for_changed_exported_symbols
	if lvl, _ := parseLevel(p.Config.Checks["require_tests_for_changed_exported_symbols"]); lvl != CheckOff {
		if !hasBaselineOrUncommitted {
			addSkipped("require_tests_for_changed_exported_symbols", "no --since or --uncommitted provided")
		} else {
			for _, sym := range changedSymbols {
				// check if exported
				if len(sym) > 0 && sym[0] >= 'A' && sym[0] <= 'Z' {
					// ensure it's in the graph
					nodes := Node(p.CurrentGraph, sym)
					if len(nodes) > 0 {
						n := nodes[0]
						if n.Kind != "test" && n.Kind != "file" && n.Kind != "interface" {
							tests := Tests(p.CurrentGraph, sym)
							if len(tests) == 0 {
								addFinding("require_tests_for_changed_exported_symbols", fmt.Sprintf("Changed exported symbol %s has no mapped tests", sym), n.File, sym, n.Line, lvl, nil)
							}
						}
					}
				}
			}
		}
	}

	// 5. max_arity
	if lvl, val := parseLevel(p.Config.Checks["max_arity"]); lvl != CheckOff {
		if val <= 0 {
			val = 6 // default
		}
		for _, s := range p.CurrentGraph.Symbols {
			if s.Kind == graph.KindFunction || s.Kind == graph.KindMethod {
				arity := countArgs(s.Signature)
				if arity > val {
					addFinding("max_arity", fmt.Sprintf("Function %s has arity %d", s.Name, arity), s.File, s.Name, s.Line, lvl, map[string]any{"arity": arity, "threshold": val})
				}
			}
		}
	}

	// 6. max_complexity
	if lvl, val := parseLevel(p.Config.Checks["max_complexity"]); lvl != CheckOff {
		if val <= 0 {
			val = 20 // default
		}
		results := Complexity(p.CurrentGraph, "")
		for _, r := range results {
			if r.Score > val {
				addFinding("max_complexity", fmt.Sprintf("Symbol %s has complexity %d", r.Symbol, r.Score), r.File, r.Symbol, r.Line, lvl, map[string]any{"complexity": r.Score, "threshold": val})
			}
		}
	}

	// 7. new_globals
	if lvl, _ := parseLevel(p.Config.Checks["new_globals"]); lvl != CheckOff {
		if !hasBaselineOrUncommitted {
			addSkipped("new_globals", "no --since or --uncommitted provided")
		} else {
			// Find globals in current
			currGlobals := Globals(p.CurrentGraph, "")

			if p.Uncommitted {
				changes := Changes(p.CurrentGraph, p.RootDir)
				for _, g := range currGlobals {
					for _, c := range changes.Symbols {
						if c.Status == ChangeNew && c.Name == g.Name {
							addFinding("new_globals", fmt.Sprintf("New package-level global %s", g.Name), g.File, g.Name, g.Line, lvl, nil)
						}
					}
				}
			} else if p.BaselineGraph != nil {
				baseGlobals := Globals(p.BaselineGraph, "")
				baseMap := make(map[string]bool)
				for _, b := range baseGlobals {
					baseMap[b.Name] = true
				}
				for _, g := range currGlobals {
					if !baseMap[g.Name] {
						// Only warn if the symbol itself changed or is new
						for _, sym := range changedSymbols {
							if sym == g.Name {
								addFinding("new_globals", fmt.Sprintf("New package-level global %s", g.Name), g.File, g.Name, g.Line, lvl, nil)
								break
							}
						}
					}
				}
			}
		}
	}

	if report.Summary.Errors > 0 {
		report.Status = string(CheckFailed)
	} else if report.Summary.Warnings > 0 {
		report.Status = string(CheckWarning)
	} else {
		report.Status = string(CheckPassed)
	}

	return report, nil
}

func countArgs(signature string) int {
	if signature == "" {
		return 0
	}
	start := strings.Index(signature, "(")
	end := strings.LastIndex(signature, ")")
	if start == -1 || end == -1 || start >= end {
		return 0
	}
	argsStr := signature[start+1 : end]
	if strings.TrimSpace(argsStr) == "" {
		return 0
	}
	// Note: this is a simple heuristic and might be slightly off for complex generics or funcs as args,
	// but it matches arity.go's heuristic. Actually, we should reuse arity logic if possible.
	return len(strings.Split(argsStr, ","))
}
