package stats

import (
	cs_callgraph "callstat/CS-Callgraph"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

/* ============================================================================
 * EdgeKindCounts
 * ----------------------------------------------------------------------------
 * Stores the count of each edge type for a package.
 * ============================================================================
 */
type EdgeKindCounts struct {
	Counts map[string]int `json:"counts"`
	Total  int            `json:"total"`
}

func newEdgeKindCounts() *EdgeKindCounts {
	return &EdgeKindCounts{
		Counts: make(map[string]int),
		Total:  0,
	}
}

func (e *EdgeKindCounts) add(kind cs_callgraph.EdgeKind) {
	e.Counts[kind.String()]++
	e.Total++
}


/* ============================================================================
 * CallGraphStats
 * ----------------------------------------------------------------------------
 * Contains statistics about functions in the callgraph, scoped to packages
 * within the configured depth.
 * ============================================================================
 */
type PackageStats struct {
	Path            string          `json:"path"`
	Depth           int             `json:"depth"`
	FunctionCount   int             `json:"functionCount"`
	UnusedFunctions []string        `json:"unusedFunctions"`
	Edges           *EdgeKindCounts `json:"edges"`
}

func newPackageStats(path string, depth int) *PackageStats {
	return &PackageStats{
		Path            : path,
		Depth           : depth,
		UnusedFunctions : []string{},
		Edges           : newEdgeKindCounts(),
	}
}

/* ============================================================================
 * CallGraphReport
 * ----------------------------------------------------------------------------
 * The root reporting structure containing totals and grouped package data.
 * ============================================================================
 */
type CallGraphReport struct {
	TotalFunctions     int                      `json:"totalFunctions"`
	ReachableFunctions int                      `json:"reachableFunctions"`
	MaxDepthSpecified  int                      `json:"maxDepthSpecified"`
	GrandTotalEdges    *EdgeKindCounts          `json:"grandTotal"`
	Packages           map[string]*PackageStats `json:"packages"`

	Indirect           *IndirectAnalysisReport	`json:"indirect"`

	ReachableFuncNames map[string]struct{}      `json:"-"`
}

func newCallGraphReport(maxDepth int) *CallGraphReport {
    return &CallGraphReport{
        MaxDepthSpecified:  maxDepth,
        GrandTotalEdges:    newEdgeKindCounts(),
        Packages:           make(map[string]*PackageStats),
        ReachableFuncNames: make(map[string]struct{}),
        Indirect:           newIndirectReport(),
    }
}

func (r *CallGraphReport) getPkg(
    path        string, 
    depthMap    map[string]int,
) *PackageStats {
	if pkg, ok := r.Packages[path]; ok {
		return pkg
	}
	d := -1
	if depth, ok := depthMap[path]; ok {
		d = depth
	}
	pkg := newPackageStats(path, d)
	r.Packages[path] = pkg
	return pkg
}
/* ============================================================================
 * IO Methods
 * ============================================================================
 */

func (r *CallGraphReport) ToJSON() ([]byte, error) {
	if r == nil {
		return []byte("null"), nil
	}
	return json.MarshalIndent(r, "", "  ")
}

/* ============================================================================
 * WriteJSONToFile
 * ----------------------------------------------------------------------------
 * Marshals the stats and writes them to the specified file path.
 * Uses 2-space indentation for readability.
 * ============================================================================
 */
func (r *CallGraphReport) WriteJSONToFile(filename string) error {
	dir := filepath.Dir(filename)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}


/* ============================================================================
 * GatherCallGraphStats
 * ----------------------------------------------------------------------------
 * Traverses the callgraph and gathers stats scoped to packages within depth.
 * ============================================================================
 */
func GatherCallGraphStats(
    g           *cs_callgraph.Graph,
    depthMap    map[string]int,
    maxDepth    int,
    projectRoot string,
    main   string,
) *CallGraphReport {
    report := newCallGraphReport(maxDepth)
    inDepth := makeDepthGate(depthMap, maxDepth)

    countFunctions(g, report, depthMap, inDepth)
    countEdges(g, report, depthMap, inDepth)

    mainNode := resolveMainNode(g, depthMap, projectRoot, main)
    if mainNode != nil {
        visited := make(map[int]struct{})
        traverseReachable(mainNode, visited, report, inDepth)
    }

    collectUnused(g, report, depthMap, inDepth)
	report.Indirect = GatherResearchStats(
		g, depthMap, maxDepth, projectRoot, mainNode,
	);
    return report
}
/* ============================================================================
 * makeDepthGate
 * ----------------------------------------------------------------------------
 * Returns a closure that reports whether a package path is within depth.
 * ============================================================================
 */
func makeDepthGate(depthMap map[string]int, maxDepth int) func(string) bool {
	return func(pkgPath string) bool {
		if maxDepth == -1 {
			return true
		}
		d, ok := depthMap[pkgPath]
		return ok && d <= maxDepth
	}
}

/* ============================================================================
 * countFunctions
 * ----------------------------------------------------------------------------
 * Counts all in-depth functions and groups them by package.
 * ============================================================================
 */
func countFunctions(
    g           *cs_callgraph.Graph , r         *CallGraphReport, 
    depthMap    map[string]int      , inDepth   func(string) bool,
) {
	for _, n := range g.Nodes {
		if n.Func == nil {
			continue
		}
		pkg := cs_callgraph.EffectivePkg(n.Func)
		if pkg == nil || pkg.Pkg == nil {
			continue
		}
		pkgPath := pkg.Pkg.Path()
		if !inDepth(pkgPath) {
			continue
		}
		r.TotalFunctions++
		r.getPkg(pkgPath, depthMap).FunctionCount++
	}
}

/* ============================================================================
 * countEdges
 * ----------------------------------------------------------------------------
 * Counts edges where both caller and callee are in-depth, grouped by
 * caller package. Also accumulates grand totals.
 * ============================================================================
 */
func countEdges(
    g           *cs_callgraph.Graph , r         *CallGraphReport, 
    depthMap    map[string]int      , inDepth   func(string) bool,
) {
	for _, n := range g.Nodes {
		if n.Func == nil {
			continue
		}
		callerPkg := cs_callgraph.EffectivePkg(n.Func)
		if callerPkg == nil || callerPkg.Pkg == nil {
			continue
		}
		callerPath := callerPkg.Pkg.Path()
		if !inDepth(callerPath) {
			continue
		}

		for _, e := range n.Out {
			if !edgeCalleeInDepth(e, inDepth) {
				continue
			}
			p := r.getPkg(callerPath, depthMap)
			p.Edges.add(e.Kind)
			r.GrandTotalEdges.add(e.Kind)
		}
	}
}

/* ============================================================================
 * edgeCalleeInDepth
 * ----------------------------------------------------------------------------
 * Returns true if the callee of an edge is a valid in-depth function.
 * ============================================================================
 */
func edgeCalleeInDepth(e *cs_callgraph.Edge, inDepth func(string) bool) bool {
	if e.Callee == nil || e.Callee.Func == nil {
		return false
	}
	calleePkg := cs_callgraph.EffectivePkg(e.Callee.Func)
	if calleePkg == nil || calleePkg.Pkg == nil {
		return false
	}
	return inDepth(calleePkg.Pkg.Path())
}


/* ============================================================================
 * resolveMainNode
 * ----------------------------------------------------------------------------
 * Calls either main resolver depending if a main path is specified or not
 * ============================================================================
 */
func resolveMainNode(
    g           *cs_callgraph.Graph, 
    depthMap    map[string]int,
    projectRoot string, 
    main        string,
) *cs_callgraph.Node {
    if main != "" {
        return resolveExplicitMain(g, depthMap, main)
    }
    return findPossibleMain(g, depthMap, projectRoot)
}

/* ============================================================================
 * resolveExplicitMain
 * ----------------------------------------------------------------------------
 * Resolves a main entry point by performing an exact match against a fully
 * qualified function string (e.g., "github.com/restic/restic/cmd/restic.main").
 *
 * Validates that the target function is present in the provided callgraph
 * and belongs to a package tracked within the project dependency map.
 * ============================================================================
 */
func resolveExplicitMain(
    g        *cs_callgraph.Graph, 
    depthMap map[string]int,
    main     string,
) *cs_callgraph.Node {
    for _, n := range g.Nodes {
        if n.Func == nil {
            continue
        }

        funcStr := n.Func.String()

        if funcStr == main {
            pkg := cs_callgraph.EffectivePkg(n.Func)
            if pkg == nil || pkg.Pkg == nil {
                continue
            }

            pkgPath := pkg.Pkg.Path()
            if _, exists := depthMap[pkgPath]; !exists {
                fmt.Printf(
                    "[resolveExplicitMain] Match found but " + 
                    "package %s is outside depthMap\n", pkgPath,
                )
                continue
            }

            fmt.Printf(
                "[resolveExplicitMain] Exact match found: %s\n",
                funcStr,
            )
            return n
        }
    }

    fmt.Printf(
        "[resolveExplicitMain] error: specified" + 
        " main entry point %q not found\n", main,
    )
    os.Exit(-1)
    return nil
}

/* ============================================================================
 * findPossibleMain
 * ----------------------------------------------------------------------------
 * Scans the callgraph to discover and rank potential main entry points.
 *
 * Enforces a strict selection hierarchy across project root packages:
 *   Priority 1: Function named "main" inside an internal depth-0 "main" package.
 *   Priority 2: Function named "main" inside any other internal depth-0 package.
 *
 * Discovered candidates are listed comprehensively. Both priority pools are
 * sorted alphabetically to guarantee consistent, deterministic root resolution
 * when multiple matching entry points are encountered.
 * ============================================================================
 */
func findPossibleMain(
    g *cs_callgraph.Graph, depthMap map[string]int,
    projectRoot string,
) *cs_callgraph.Node {
    var priority1 []*cs_callgraph.Node
    var priority2 []*cs_callgraph.Node

    for _, n := range g.Nodes {
        if n.Func == nil || n.Func.Name() != "main" {
            continue
        }

        pkg := cs_callgraph.EffectivePkg(n.Func)
        if pkg == nil || pkg.Pkg == nil {
            continue
        }

        pkgPath := pkg.Pkg.Path()
        d, ok := depthMap[pkgPath]

        if ok && d == 0 && strings.HasPrefix(pkgPath, projectRoot) {
            if pkg.Pkg.Name() == "main" {
                priority1 = append(priority1, n)
            } else {
                priority2 = append(priority2, n)
            }
        }
    }

    sort.Slice(priority1, func(i, j int) bool {
        return priority1[i].Func.String() < priority1[j].Func.String()
    })
    sort.Slice(priority2, func(i, j int) bool {
        return priority2[i].Func.String() < priority2[j].Func.String()
    })

    totalMains := len(priority1) + len(priority2)
    if totalMains > 0 {
        fmt.Printf("[resolveMainNode] Found %d total potential main function(s):\n", totalMains)
        for _, n := range priority1 {
            fmt.Printf("  -> [Priority 1] (pkg: main): %s\n", n.Func.String())
        }
        for _, n := range priority2 {
            fmt.Printf("  -> [Priority 2] (pkg: other): %s\n", n.Func.String())
        }
    }

    if len(priority1) > 1 || (len(priority1) == 0 && len(priority2) > 1) {
        fmt.Println("[WARNING]: Multiple possible main functions found.")
    }

    if len(priority1) > 0 {
        fmt.Printf("[resolveMainNode] selected priority main: %s\n", priority1[0].Func.String())
        return priority1[0]
    } else if len(priority2) > 0 {
        fmt.Printf("[resolveMainNode] selected priority main: %s\n", priority2[0].Func.String())
        return priority2[0]
    }

    fmt.Printf("[resolveMainNode] error: no main found\n")
    os.Exit(-1)
    return nil
}

/* ============================================================================
 * collectUnused
 * ----------------------------------------------------------------------------
 * Identifies functions that were not reached during traversal from main,
 * grouped by package. Only considers in-depth packages.
 * ============================================================================
 */
func collectUnused(
    g           *cs_callgraph.Graph , r         *CallGraphReport, 
    depthMap    map[string]int      , inDepth   func(string) bool,
) {
	for _, n := range g.Nodes {
		if n.Func == nil {
			continue
		}
		pkg := cs_callgraph.EffectivePkg(n.Func)
		if pkg == nil || pkg.Pkg == nil {
			continue
		}
		pkgPath := pkg.Pkg.Path()
		if !inDepth(pkgPath) {
			continue
		}
		if _, reachable := r.ReachableFuncNames[n.Func.String()]; !reachable {
			p := r.getPkg(pkgPath, depthMap)
			p.UnusedFunctions = append(p.UnusedFunctions, n.Func.Name())
		}
	}
}

/* ============================================================================
 * traverseReachable
 * ----------------------------------------------------------------------------
 * DFS from a node, only following edges whose callees are in-depth.
 * ============================================================================
 */
func traverseReachable(
    n *cs_callgraph.Node, 
    visited map[int]struct{}, 
    r *CallGraphReport, 
    inDepth func(string) bool,
) {
    if n == nil || n.Func == nil {
        return
    }
    if _, ok := visited[n.ID]; ok {
        return
    }
    visited[n.ID] = struct{}{}

    pkg := cs_callgraph.EffectivePkg(n.Func)
    if pkg == nil || pkg.Pkg == nil || !inDepth(pkg.Pkg.Path()) {
        return
    }

    r.ReachableFuncNames[n.Func.String()] = struct{}{}
    r.ReachableFunctions++

    for _, e := range n.Out {
        if e.Callee != nil {
            traverseReachable(e.Callee, visited, r, inDepth)
        }
    }
}
