package visualisation

import (
	cs_callgraph "callstat/CS-Callgraph"
	"fmt"
	"maps"
	"strconv"
	"strings"
)

/* ============================================================================
 * PackageGraph
 * ----------------------------------------------------------------------------
 * Represents a per-package graph structure containing:
 *   - Nodes belonging to the package
 *   - Edges between nodes
 *   - External function references
 * ============================================================================
 */
type PackageGraph struct {
    Name          string
    Nodes         map[int]*DotNode
    Edges         []*DotEdge
    ExternalFuncs map[string]*cs_callgraph.Node
}



/* ============================================================================
 * isAnonFunc
 * ----------------------------------------------------------------------------
 * Checks whether a function is anonymous by looking for compiler-generated
 * naming patterns (e.g. containing '$'). Returns false if function is nil.
 * ============================================================================
 */
func isAnonFunc(fn *cs_callgraph.Node) bool {
    return fn.Func != nil && strings.Contains(fn.Func.Name(), "$")
}

/* ============================================================================
 * BuildDotGraphPerPackage
 * ----------------------------------------------------------------------------
 * Traverses the callgraph and builds a DOT graph per package.
 * Each package gets its own DotGraph containing:
 *   - Local nodes
 *   - Edges within the package
 *   - Links to external package clusters
 * ============================================================================
 */
func BuildDotGraphPerPackage(g *cs_callgraph.Graph) map[string]*DotGraph {

    packageGraphs := map[string]*DotGraph{}

    for _, n := range g.Nodes {

        /* -------------------------------------------------------
         * 1. VALIDATION
         * ------------------------------------------------------- */
        if n.Func == nil || n.Func.Pkg == nil || n.Func.Pkg.Pkg == nil {
            continue
        }

        pkgPath := n.Func.Pkg.Pkg.Path()

        /* -------------------------------------------------------
         * 2. GRAPH INITIALIZATION
         * ------------------------------------------------------- */
        pkgGraph := ensurePackageGraph(packageGraphs, pkgPath)

        /* -------------------------------------------------------
         * 3. LOCAL NODE REGISTRATION
         * ------------------------------------------------------- */
        registerLocalNode(pkgGraph, n)

        /* -------------------------------------------------------
         * 4. EDGE HANDLING
         * ------------------------------------------------------- */
        handleEdges(pkgGraph, n, pkgPath)
    }

    return packageGraphs
}

/* ============================================================================
 * ensurePackageGraph
 * ----------------------------------------------------------------------------
 * Retrieves or creates a DotGraph for a given package path.
 * ============================================================================
 */
func ensurePackageGraph(
    graphs map[string]*DotGraph,
    pkgPath string,
) *DotGraph {

    if g, ok := graphs[pkgPath]; ok {
        return g
    }

    g := newDotGraph()
    graphs[pkgPath] = g
    return g
}

/* ============================================================================
 * registerLocalNode
 * ----------------------------------------------------------------------------
 * Adds the given callgraph node to the package graph as a DOT node.
 * Avoids duplicate insertion if already present.
 * ============================================================================
 */
func registerLocalNode(pkgGraph *DotGraph, n *cs_callgraph.Node) {
    nodeID := convertNodeID(n.ID, ns_normal)

    if _, exists := pkgGraph.Nodes[nodeID]; exists {
        return
    }

    pkgGraph.Nodes[nodeID] = buildNodeFromCS(n)
}

/* ============================================================================
 * handleEdges
 * ----------------------------------------------------------------------------
 * Iterates through all outgoing edges of a node and delegates handling.
 * ============================================================================
 */
func handleEdges(
    pkgGraph *DotGraph,
    n *cs_callgraph.Node,
    pkgPath string,
) {
    for _, e := range n.Out {
        handleEdge(pkgGraph, n, e, pkgPath)
    }
}

/* ============================================================================
 * handleEdge
 * ----------------------------------------------------------------------------
 * Handles a single edge and determines how it should be represented:
 *   1. Root / special edges (nil callee)
 *   2. Intra-package  edges
 *   3. Inter-package  edges (clustered)
 * ============================================================================
 */
func handleEdge(
    pkgGraph *DotGraph, n *cs_callgraph.Node,
    e *cs_callgraph.Edge, pkgPath string,
) {
     if e.Callee == nil || e.Callee.Func == nil || e.Callee.Func.Pkg == nil || e.Callee.Func.Pkg.Pkg == nil {
        fmt.Printf("Skipping edge from %s: incomplete callee info\n", shortFuncName(n))
        return
    }
    /* -------------------------------------------------------
     * 1. ROOT / SPECIAL EDGE HANDLING
     * ------------------------------------------------------- */
    if e.Callee == nil || e.Callee.Func == nil {

        // Guard: e.Callee might be nil (avoid panic)
        if e.Callee != nil {
            rootID := convertNodeID(e.Callee.ID, ns_normal)

            if _, exists := pkgGraph.Nodes[rootID]; !exists {
                pkgGraph.Nodes[rootID] = buildNodeFromCS(e.Callee)
            }
        }

        pkgGraph.Edges = append(pkgGraph.Edges, buildEdgeFromCS(e))
        return
    }

    calleePkg := e.Callee.Func.Pkg.Pkg.Path()

    /* -------------------------------------------------------
     * 2. INTRA-PACKAGE EDGE
     * ------------------------------------------------------- */
    if calleePkg == pkgPath {
        pkgGraph.Edges = append(pkgGraph.Edges, buildEdgeFromCS(e))
        return
    }

    /* -------------------------------------------------------
     * 3. INTER-PACKAGE EDGE (CLUSTER)
     * ------------------------------------------------------- */
    buildLinkClusterNode(pkgGraph, &calleePkg, e, n)
}


/* ============================================================================
 * shortPkgName
 * ----------------------------------------------------------------------------
 * Returns a short display name for a package.
 * ============================================================================
 */

func shortPkgName(pkgPath string) string {
    parts := strings.Split(pkgPath, "/")
    return parts[len(parts)-1]
}

/* ============================================================================
 * shortFuncName
 * ----------------------------------------------------------------------------
 * Returns a short display name for a function. Falls back to "<root>" if
 * the function is nil. Used mainly for graph node labels.
 * ============================================================================
 */
func shortFuncName(fn *cs_callgraph.Node) string {
    if fn.Func == nil {
        return "<root>"
    }
    return fn.Func.Name()
}

/* ============================================================================
 * fullFuncName
 * ----------------------------------------------------------------------------
 * Returns the full function name including package path and signature.
 * Used for tooltips to provide more detailed function information.
 * ============================================================================
 */
func fullFuncName(fn *cs_callgraph.Node) string {
    if fn.Func == nil {
        return "<root>"
    }
    return fn.Func.String()
}

/* ============================================================================
 * convertNodeID
 * ----------------------------------------------------------------------------
 * Converts a numeric node ID into a string identifier suitable for DOT.
 * External nodes are prefixed differently to avoid collisions and allow
 * styling/grouping.
 * ============================================================================
 */
func convertNodeID(id int, typ NodeStyle) string {
    switch typ {
    case ns_external    : return "ext_" + strconv.Itoa(id)
    default             : return "n"    + strconv.Itoa(id)
    }
}
/* ============================================================================
 * buildNodeFromCS
 * ----------------------------------------------------------------------------
 * Creates a DotNode from a callgraph node. Determines node type (e.g. anon)
 * and injects label + tooltip information.
 * ============================================================================
 */
func buildNodeFromCS(n *cs_callgraph.Node) *DotNode {
    nodeType := ns_normal
    if isAnonFunc(n) {
        nodeType = ns_anon
    }

    return buildNode(
        convertNodeID   (n.ID, nodeType),
        shortFuncName   (n),
        fullFuncName    (n),
        nodeType,
    )
}

/* ============================================================================
 * buildNode
 * ----------------------------------------------------------------------------
 * Constructs a DotNode and merges global styling attributes for the given
 * node type. Ensures label and tooltip are always present.
 * ============================================================================
 */
func buildNode(
    id string, label string,
    tooltip string, typ NodeStyle,
) *DotNode {

    attrs := map[string]string{
        "label"     : label,
        "tooltip"   : tooltip,
    }

    if styleMap, ok := global_styles.NodeStyles[string(typ)]; ok {
        maps.Copy(attrs, styleMap)
    }

    return &DotNode{
        ID      : id,
        Attrs   : attrs,
    }
}

/* ============================================================================
 * mapEdgeKindToStyle
 * ----------------------------------------------------------------------------
 * Maps callgraph edge kinds to visual edge styles used in DOT output.
 * ============================================================================
 */
func mapEdgeKindToStyle(typ cs_callgraph.EdgeKind) EdgeStyle {
    switch typ {
    case cs_callgraph.CallEdge  :		return es_call
    case cs_callgraph.DeferEdge :		return es_defer
    case cs_callgraph.GoEdge    :		return es_go
    case cs_callgraph.PanicEdge :		return es_panic
    default                     :		return es_default
    }
}

/* ============================================================================
 * buildEdgeFromCS
 * ----------------------------------------------------------------------------
 * Converts a callgraph edge into a DotEdge including styling and tooltip.
 * ============================================================================
 */
func buildEdgeFromCS(e *cs_callgraph.Edge) *DotEdge {
    return buildEdge(
        convertNodeID(e.Caller.ID, ns_normal),
        convertNodeID(e.Callee.ID, ns_normal),
        mapEdgeKindToStyle(e.Kind),
        e.Description(),
    )
}

/* ============================================================================
 * buildEdge
 * ----------------------------------------------------------------------------
 * Constructs a DotEdge and applies styling attributes based on edge type.
 * Tooltip is always included from the edge description.
 * ============================================================================
 */
func buildEdge(
    from string, to string,
    typ EdgeStyle, des string,
) *DotEdge {

    attrs := map[string]string{
        "tooltip": des,
    }

    if styleMap, ok := global_styles.EdgeStyles[string(typ)]; ok {
        maps.Copy(attrs, styleMap)
    }

    return &DotEdge{
        From:  from,
        To:    to,
        Attrs: attrs,
    }
}

/* ============================================================================
 * buildLinkClusterNode
 * ----------------------------------------------------------------------------
 * Handles edges where the callee belongs to a different package.
 *
 * Behaviour:
 *   1. Ensure the target package cluster exists
 *   2. Add the external node into that cluster if not already present
 *   3. Create an edge from the current node to the external node
 *
 * This allows external package functions to be visually grouped together.
 * ============================================================================
 */
func buildLinkClusterNode(
    pkgGraph *DotGraph, calleePkg *string,
    e *cs_callgraph.Edge, n *cs_callgraph.Node,
) {

    cluster := buildCluster(pkgGraph, calleePkg)
    extNodeID := convertNodeID(e.Callee.ID, ns_external)

    /* -------------------------------------------------------
     * Ensure external node exists in cluster
     * ------------------------------------------------------- */
    if _, exists := cluster.Nodes[extNodeID]; !exists {
        cluster.Nodes[extNodeID] = buildNode(
            extNodeID,
            shortFuncName(e.Callee),
            fullFuncName(e.Callee),
            ns_external,
        )
    }

    /* -------------------------------------------------------
     * Add edge from internal node -> external node
     * ------------------------------------------------------- */
    pkgGraph.Edges = append(pkgGraph.Edges, buildEdge(
        convertNodeID(n.ID, ns_normal),
        extNodeID,
        mapEdgeKindToStyle(e.Kind),
        e.Description(),
    ))
}

/* ============================================================================
 * buildCluster
 * ----------------------------------------------------------------------------
 * Ensures a cluster exists for a given external package.
 *
 * Clusters group nodes belonging to the same package visually in DOT output.
 * If the cluster does not exist, it is created with:
 *   - Label (short package name)
 *   - Tooltip (full package path)
 *   - Global cluster styling applied
 * ============================================================================
 */
func buildCluster(pkgGraph *DotGraph, calleePkg *string) *DotCluster {

    clusterID := fmt.Sprintf("cluster_%s", *calleePkg)

    if cluster, exists := pkgGraph.Clusters[clusterID]; exists {
        return cluster
    }

    /* -------------------------------------------------------
     * Base attributes
     * ------------------------------------------------------- */
    attrs := map[string]string{
        "tooltip": *calleePkg,
    }

    /* -------------------------------------------------------
     * Apply global cluster styles
     * ------------------------------------------------------- */
    maps.Copy(attrs, global_styles.Cluster)

    /* -------------------------------------------------------
     * Create cluster
     * ------------------------------------------------------- */
    cluster := &DotCluster{
        ID:    clusterID,
        Label: shortPkgName(*calleePkg),
        Attrs: attrs,
        Nodes: make(map[string]*DotNode),
    }

    pkgGraph.Clusters[clusterID] = cluster
    return cluster
}