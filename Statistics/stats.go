package stats

import (
	cs_callgraph "callstat/CS-Callgraph"
	"encoding/json"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/ssa"
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
	IsStdlib        bool            `json:"isStdlib"`
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
	pkg.IsStdlib = cs_callgraph.IsStdlib(path)
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
    main        *ssa.Function,
	skipPkg     map[string]struct{},
) *CallGraphReport {
    report := newCallGraphReport(maxDepth)
    inDepth := makeDepthGate(depthMap, maxDepth, skipPkg)

    countFunctions(g, report, depthMap, inDepth)
    countEdges(g, report, depthMap, inDepth)

    mainNode := g.Nodes[main]

    if mainNode != nil {
        visited := make(map[int]struct{})
        traverseReachable(mainNode, visited, report, inDepth)
    }

    collectUnused(g, report, depthMap, inDepth)
	report.Indirect = GatherResearchStats(
		g, depthMap, maxDepth, projectRoot, mainNode, skipPkg,
	);
    return report
}
/* ============================================================================
 * makeDepthGate
 * ----------------------------------------------------------------------------
 * Returns a closure that reports whether a package path is within depth.
 * ============================================================================
 */
func makeDepthGate(
	depthMap map[string]int, 
	maxDepth int,
	skipPkg  map[string]struct{},
)func(string) bool {
	return func(pkgPath string) bool {
		if _, skip := skipPkg[pkgPath]; skip {
            return false
        }

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
