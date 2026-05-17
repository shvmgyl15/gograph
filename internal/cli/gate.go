package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/ozgurcd/gograph/internal/search"
)

type GateConfig struct {
	MaxComplexity       *int     `yaml:"max_complexity"`
	MaxInstability      *float64 `yaml:"max_instability"`
	MaxGodObjectMethods *int     `yaml:"max_god_object_methods"`
	AllowNewOrphans     *bool    `yaml:"allow_new_orphans"`
	MaxNewCouplingEdges *int     `yaml:"max_new_coupling_edges"`
}

func runGate() int {
	configPath := ".gograph.yml"
	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading .gograph.yml: %v\n", err)
		return 1
	}

	var cfg GateConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing .gograph.yml: %v\n", err)
		return 1
	}

	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading graph (run gograph build . first): %v\n", err)
		return 1
	}

	absRoot, _ := filepath.Abs(".")
	sr := search.Stale(g, absRoot)
	if sr.IsStale {
		fmt.Fprintf(os.Stderr, "warning: graph is stale compared to source files. run 'gograph build .' for accurate results.\n")
	}

	violations := 0

	// 1. max_complexity
	if cfg.MaxComplexity != nil {
		worstScore := 0
		worstName := ""
		for _, s := range g.Symbols {
			if s.Kind != "function" && s.Kind != "method" {
				continue
			}
			res := search.Complexity(g, s.Name)
			for _, r := range res {
				if r.Score > worstScore {
					worstScore = r.Score
					worstName = r.Symbol
				}
			}
		}
		if worstScore > *cfg.MaxComplexity {
			fmt.Printf("cross      complexity     max=%d    worst=%d  %s  VIOLATION\n", *cfg.MaxComplexity, worstScore, worstName)
			violations++
		} else {
			fmt.Printf("checkmark  complexity     max=%d    worst=%d  %s\n", *cfg.MaxComplexity, worstScore, worstName)
		}
	}

	// 2. max_instability
	if cfg.MaxInstability != nil {
		worstScore := 0.0
		worstName := ""
		coupling := search.Coupling(g, "")
		for _, c := range coupling {
			if c.Instability > worstScore {
				worstScore = c.Instability
				worstName = c.Package
			}
		}
		if worstScore > *cfg.MaxInstability {
			fmt.Printf("cross      instability    max=%.2f  worst=%.2f  %s   VIOLATION\n", *cfg.MaxInstability, worstScore, worstName)
			violations++
		} else {
			fmt.Printf("checkmark  instability    max=%.2f  worst=%.2f  %s\n", *cfg.MaxInstability, worstScore, worstName)
		}
	}

	// 3. max_god_object_methods
	if cfg.MaxGodObjectMethods != nil {
		worstMethods := 0
		worstName := ""
		// Count methods per struct manually, matching the prompt's definition
		methodsByReceiver := make(map[string]int)
		for _, s := range g.Symbols {
			if s.Kind == "method" && s.Receiver != "" {
				recv := s.Receiver
				if len(recv) > 0 && recv[0] == '*' {
					recv = recv[1:]
				}
				methodsByReceiver[recv]++
			}
		}
		for name, count := range methodsByReceiver {
			if count > worstMethods {
				worstMethods = count
				worstName = name
			}
		}

		if worstMethods > *cfg.MaxGodObjectMethods {
			fmt.Printf("cross      god objects    %s has %d methods             VIOLATION\n", worstName, worstMethods)
			violations++
		} else {
			fmt.Printf("checkmark  god objects    0 structs exceed threshold\n")
		}
	}

	// 4. allow_new_orphans
	if cfg.AllowNewOrphans != nil && !*cfg.AllowNewOrphans {
		if g.Baseline == nil {
			fmt.Println("checkmark  orphans        0 new (skipped: no baseline exists yet)")
		} else {
			currOrphans := len(search.Orphans(g))
			if currOrphans > g.Baseline.OrphanCount {
				fmt.Printf("cross      orphans        %d new unreachable symbols (baseline %d)  VIOLATION\n", currOrphans-g.Baseline.OrphanCount, g.Baseline.OrphanCount)
				violations++
			} else {
				fmt.Printf("checkmark  orphans        0 new unreachable symbols\n")
			}
		}
	}

	// 5. max_new_coupling_edges
	if cfg.MaxNewCouplingEdges != nil {
		if g.Baseline == nil {
			fmt.Println("checkmark  coupling       0 new edges (skipped: no baseline exists yet)")
		} else {
			currEdges := len(g.Imports)
			newEdges := currEdges - g.Baseline.CouplingEdges
			if newEdges > *cfg.MaxNewCouplingEdges {
				fmt.Printf("cross      coupling       %d new edges (max %d)          VIOLATION\n", newEdges, *cfg.MaxNewCouplingEdges)
				violations++
			} else {
				if newEdges < 0 {
					newEdges = 0
				}
				fmt.Printf("checkmark  coupling       %d new edges\n", newEdges)
			}
		}
	}

	if violations == 0 {
		fmt.Println("GATE PASSED")
		return 0
	}
	fmt.Printf("GATE FAILED — %d violations\n", violations)
	return 1
}
