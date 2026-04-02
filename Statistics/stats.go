package statistics

import (
	cs_callgraph "callstat/CS-Callgraph"
	"fmt"
)

/* ============================================================================
 * CallGraphStats
 * ----------------------------------------------------------------------------
 * Contains statistics about functions in the callgraph
 * ============================================================================
 */
type CallGraphStats struct {
	TotalFunctions       int                      // total functions defined
	ReachableFunctions   int                      // functions reachable from main
	FunctionsPerPackage  map[string]int           // number of functions per package
	UnusedFunctions      map[string][]string      // unused functions per package (not reachable)
	ReachableFuncNames   map[string]struct{}      // internal set of reachable functions
}

/* ============================================================================
 * GatherCallGraphStats
 * ----------------------------------------------------------------------------
 * Traverses the callgraph starting from main and gathers stats:
 *   - total functions
 *   - reachable functions
 *   - functions per package
 *   - unused functions per package
 *
 * Any function connected (directly or indirectly) to main is counted as used.
 * ============================================================================
 */
func GatherCallGraphStats(g *cs_callgraph.Graph) *CallGraphStats {

	stats := &CallGraphStats{
		FunctionsPerPackage: make(map[string]int),
		UnusedFunctions:     make(map[string][]string),
		ReachableFuncNames:  make(map[string]struct{}),
	}

	/* -------------------------------------------------------
	 * 1. Count all functions per package
	 * ------------------------------------------------------- */
	for _, n := range g.Nodes {
		if n.Func == nil || n.Func.Pkg == nil || n.Func.Pkg.Pkg == nil {
			continue
		}

		pkgPath := n.Func.Pkg.Pkg.Path()
		stats.TotalFunctions++
		stats.FunctionsPerPackage[pkgPath]++
	}

	/* -------------------------------------------------------
	 * 2. Find reachable functions starting from main
	 * ------------------------------------------------------- */
	var mainNodes []*cs_callgraph.Node
	for _, n := range g.Nodes {
		if n.Func != nil && n.Func.Name() == "main" {
			mainNodes = append(mainNodes, n)
		}
	}

	visited := make(map[int]struct{})
	for _, mn := range mainNodes {
		traverseReachable(mn, visited, stats)
	}

	/* -------------------------------------------------------
	 * 3. Compute unused functions per package
	 * ------------------------------------------------------- */
	for _, n := range g.Nodes {
		if n.Func == nil || n.Func.Pkg == nil || n.Func.Pkg.Pkg == nil {
			continue
		}

		pkgPath := n.Func.Pkg.Pkg.Path()
		if _, ok := stats.ReachableFuncNames[n.Func.String()]; !ok {
			stats.UnusedFunctions[pkgPath] = append(stats.UnusedFunctions[pkgPath], n.Func.Name())
		}
	}

	return stats
}

/* ============================================================================
 * traverseReachable
 * ----------------------------------------------------------------------------
 * Recursively traverses outgoing edges to mark reachable functions from main.
 * Prevents cycles via visited map.
 * ============================================================================
 */
func traverseReachable(n *cs_callgraph.Node, visited map[int]struct{}, stats *CallGraphStats) {

	if n == nil || n.Func == nil {
		return
	}

	if _, ok := visited[n.ID]; ok {
		return
	}
	visited[n.ID] = struct{}{}

	stats.ReachableFuncNames[n.Func.String()] = struct{}{}
	stats.ReachableFunctions++

	for _, e := range n.Out {
		if e.Callee != nil {
			traverseReachable(e.Callee, visited, stats)
		}
	}
}

/* ============================================================================
 * PrintStats
 * ----------------------------------------------------------------------------
 * Utility function to print the statistics nicely
 * ============================================================================
 */
func (stats *CallGraphStats) PrintStats() {
	fmt.Printf("=== CallGraph Statistics ===\n")
	fmt.Printf("Total functions defined: %d\n", stats.TotalFunctions)
	fmt.Printf("Functions reachable from main: %d\n", stats.ReachableFunctions)
	fmt.Println("\nFunctions per package:")
	for pkg, count := range stats.FunctionsPerPackage {
		fmt.Printf("  %s: %d\n", pkg, count)
	}
	fmt.Println("\nUnused functions per package:")
	for pkg, funcs := range stats.UnusedFunctions {
		if len(funcs) > 0 {
			fmt.Printf("  %s: %v\n", pkg, funcs)
		}
	}
}