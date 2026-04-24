package visualisation

import (
	cs_callgraph "callstat/CS-Callgraph"
	"fmt"
	"strings"
)

/* ============================================================================
 * EdgeKindCounts
 * ----------------------------------------------------------------------------
 * Stores the count of each edge type for a package.
 * ============================================================================
 */
type EdgeKindCounts struct {
	Counts map[cs_callgraph.EdgeKind]int
	Total  int
}

func newEdgeKindCounts() *EdgeKindCounts {
	return &EdgeKindCounts{
		Counts	: make(map[cs_callgraph.EdgeKind]int),
		Total	: 0,
	}
}

func (e *EdgeKindCounts) add(kind cs_callgraph.EdgeKind) {
	e.Counts[kind]++
	e.Total++
}

/* ============================================================================
 * CallGraphStats
 * ----------------------------------------------------------------------------
 * Contains statistics about functions in the callgraph, scoped to packages
 * within the configured depth.
 * ============================================================================
 */
type CallGraphStats struct {
	TotalFunctions      int
	ReachableFunctions  int
	FunctionsPerPackage map[string]int
	UnusedFunctions     map[string][]string
	ReachableFuncNames  map[string]struct{}
	EdgesPerPackage     map[string]*EdgeKindCounts
	GrandTotalEdges     *EdgeKindCounts
}

func newCallGraphStats() *CallGraphStats {
	return &CallGraphStats{
		TotalFunctions			: 0,
		ReachableFunctions		: 0,
		FunctionsPerPackage		: make(map[string]int),
		UnusedFunctions			: make(map[string][]string),
		ReachableFuncNames		: make(map[string]struct{}),
		EdgesPerPackage			: make(map[string]*EdgeKindCounts),
		GrandTotalEdges			: newEdgeKindCounts(),
	}
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
) *CallGraphStats {
	stats   := newCallGraphStats()
	inDepth := makeDepthGate(depthMap, maxDepth)

	countFunctions(g, stats, inDepth)
	countEdges(g, stats, inDepth)

	mainNode := resolveMainNode(g, depthMap, maxDepth, projectRoot, inDepth)
	if mainNode != nil {
		visited := make(map[int]struct{})
		traverseReachable(mainNode, visited, stats, inDepth)
	}

	collectUnused(g, stats, inDepth)

	return stats
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
	g       *cs_callgraph.Graph,
	stats   *CallGraphStats,
	inDepth func(string) bool,
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
		stats.TotalFunctions++
		stats.FunctionsPerPackage[pkgPath]++
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
	g       *cs_callgraph.Graph,
	stats   *CallGraphStats,
	inDepth func(string) bool,
) {
	for _, n := range g.Nodes {
		if n.Func == nil {
			continue
		}
		callerPkg := cs_callgraph.EffectivePkg(n.Func)
		if callerPkg == nil || callerPkg.Pkg == nil {
			continue
		}
		callerPkgPath := callerPkg.Pkg.Path()
		if !inDepth(callerPkgPath) {
			continue
		}

		for _, e := range n.Out {
			if !edgeCalleeInDepth(e, inDepth) {
				continue
			}
			if _, ok := stats.EdgesPerPackage[callerPkgPath]; !ok {
				stats.EdgesPerPackage[callerPkgPath] = newEdgeKindCounts()
			}
			stats.EdgesPerPackage[callerPkgPath].add(e.Kind)
			stats.GrandTotalEdges.add(e.Kind)
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
	g           *cs_callgraph.Graph,
	depthMap    map[string]int,
	maxDepth    int,
	projectRoot string,
	inDepth     func(string) bool,
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
		isInternalMain := ok && d == 0 && strings.HasPrefix(pkgPath, projectRoot)

		if isInternalMain && pkg.Pkg.Name() == "main" {
			return n // best match, stop immediately
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
	g       *cs_callgraph.Graph,
	stats   *CallGraphStats,
	inDepth func(string) bool,
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
		if _, reachable := stats.ReachableFuncNames[n.Func.String()]; !reachable {
			stats.UnusedFunctions[pkgPath] = append(
				stats.UnusedFunctions[pkgPath], n.Func.Name(),
			)
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
	n       *cs_callgraph.Node,
	visited map[int]struct{},
	stats   *CallGraphStats,
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

	stats.ReachableFuncNames[n.Func.String()] = struct{}{}
	stats.ReachableFunctions++

	for _, e := range n.Out {
		if e.Callee != nil {
			traverseReachable(e.Callee, visited, stats, inDepth)
		}
	}
}

/* ============================================================================
 * PrintStats
 * ----------------------------------------------------------------------------
 * Prints the statistics to stdout.
 * ============================================================================
 */
func (stats *CallGraphStats) PrintStats() {
	fmt.Printf("=== CallGraph Statistics ===\n")
	fmt.Printf("Total functions (in depth): %d\n", stats.TotalFunctions)
	fmt.Printf("Functions reachable from main: %d\n", stats.ReachableFunctions)

	fmt.Println("\nFunctions per package:")
	for pkg, count := range stats.FunctionsPerPackage {
		fmt.Printf("  %s: %d\n", pkg, count)
	}

	fmt.Println("\nEdges per package:")
	for pkg, counts := range stats.EdgesPerPackage {
		fmt.Printf("  %s (total: %d)\n", pkg, counts.Total)
		for kind, n := range counts.Counts {
			fmt.Printf("    %s: %d\n", kind, n)
		}
	}

	fmt.Printf("\nGrand total edges: %d\n", stats.GrandTotalEdges.Total)
	for kind, n := range stats.GrandTotalEdges.Counts {
		fmt.Printf("  %s: %d\n", kind, n)
	}

	fmt.Println("\nUnused functions per package:")
	for pkg, funcs := range stats.UnusedFunctions {
		if len(funcs) > 0 {
			fmt.Printf("  %s: %v\n", pkg, funcs)
		}
	}
}