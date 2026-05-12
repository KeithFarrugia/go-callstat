package stats

import (
	cs_callgraph "callstat/CS-Callgraph"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
) *CallGraphReport {
    report := newCallGraphReport(maxDepth)
    inDepth := makeDepthGate(depthMap, maxDepth)

    countFunctions(g, report, depthMap, inDepth)
    countEdges(g, report, depthMap, inDepth)

    mainNode := resolveMainNode(g, depthMap, projectRoot, inDepth)
    if mainNode != nil {
        visited := make(map[int]struct{})
        // This now does both: marks reachable AND analyzes instructions
        traverseReachable(mainNode, visited, report, inDepth)
    }

    collectUnused(g, report, depthMap, inDepth)
	report.Indirect = GatherResearchStats(g, depthMap, maxDepth, projectRoot);
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
 * Finds the best candidate for the main entry point.
 *
 * Priority:
 *   1. Internal depth-0 package literally named "main", function "main"
 *   2. Any internal depth-0 function named "main"
 *   3. Any in-depth function named "main"
 * ============================================================================
 */
func resolveMainNode(
    g               *cs_callgraph.Graph , depthMap  map[string]int, 
    projectRoot     string              , inDepth   func(string) bool,
) *cs_callgraph.Node {
	var fallback *cs_callgraph.Node
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
		
        isInternalMain := ok && d == 0 && 
            strings.HasPrefix(pkgPath, projectRoot)

		if isInternalMain && pkg.Pkg.Name() == "main" {
			return n
		}
		if isInternalMain && fallback == nil {
			fallback = n
		}
		if fallback == nil && inDepth(pkgPath) {
			fallback = n
		}
	}
	return fallback
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

    // 1. Existing Logic: Mark Reachability
    r.ReachableFuncNames[n.Func.String()] = struct{}{}
    r.ReachableFunctions++

    for _, e := range n.Out {
        if e.Callee != nil {
            traverseReachable(e.Callee, visited, r, inDepth)
        }
    }
}

/* ============================================================================
 * PrintStats
 * ----------------------------------------------------------------------------
 * Prints the statistics to stdout.
 * ============================================================================
 */
func (r *CallGraphReport) PrintStats() {
	fmt.Printf("=== CallGraph Statistics (Max Depth: %d) ===\n", r.MaxDepthSpecified)
	fmt.Printf("Total functions: %d\n", r.TotalFunctions)
	fmt.Printf("Reachable functions: %d\n", r.ReachableFunctions)

	fmt.Println("\nPer-Package Details:")
	for path, p := range r.Packages {
		fmt.Printf("  %s (Depth: %d)\n", path, p.Depth)
		fmt.Printf("    Functions: %d\n", p.FunctionCount)
		fmt.Printf("    Edges: %d\n", p.Edges.Total)
		for kind, count := range p.Edges.Counts {
			fmt.Printf("      - %s: %d\n", kind, count)
		}
		if len(p.UnusedFunctions) > 0 {
			fmt.Printf("    Unused: %v\n", p.UnusedFunctions)
		}
	}

	fmt.Printf("\nGrand total edges: %d\n", r.GrandTotalEdges.Total)
	for kind, n := range r.GrandTotalEdges.Counts {
		fmt.Printf("  %s: %d\n", kind, n)
	}
}