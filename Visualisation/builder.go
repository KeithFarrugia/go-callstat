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
func BuildDotGraphPerPackage(
	g 			*cs_callgraph.Graph, 
    skipPkg 	map[string]struct{},  
) map[string]*DotGraph {

	packageGraphs := map[string]*DotGraph{}

	for _, n := range g.Nodes {

		/* -------------------------------------------------------
		 * 1. VALIDATION
		 * ------------------------------------------------------- */
		if n.Func == nil {
			continue
		}
		pkg := cs_callgraph.EffectivePkg(n.Func)
		if pkg == nil || pkg.Pkg == nil {
			continue
		}
		pkgPath := pkg.Pkg.Path()

        /* -------------------------------------------------------
         * Skip packages we don't want to visualise
         * ------------------------------------------------------- */
        if _, skip := skipPkg[pkgPath]; skip {
            continue
        }
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
	/* -------------------------------------------------------
     * Interface method nodes: seed into their own package graph
     * and register any incoming edges from concrete callers.
     * ------------------------------------------------------- */
    for _, n := range g.IfaceNodes {
		if n.IfaceMethod.Pkg() == nil {
			continue
		}
        pkgPath := n.IfaceMethod.Pkg().Path()
        if _, skip := skipPkg[pkgPath]; skip {
            continue
        }
        pkgGraph := ensurePackageGraph(packageGraphs, pkgPath)
        registerIfaceNode(pkgGraph, n)
    }
	return packageGraphs
}
/* ============================================================================
 * registerIfaceNode
 * ----------------------------------------------------------------------------
 * Adds an interface method node to its home package graph.
 * ============================================================================
 */
func registerIfaceNode(pkgGraph *DotGraph, n *cs_callgraph.Node) {
    nodeID := convertNodeID(n.ID, ns_interface)
    if _, exists := pkgGraph.Nodes[nodeID]; exists {
        return
    }
    pkgGraph.Nodes[nodeID] = buildNodeFromCS(n)
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
 * checkIncompleteCallee
 * ----------------------------------------------------------------------------
 * Validates the callee's metadata to ensure it has the necessary SSA and 
 * type information for graph generation. Returns an error description if 
 * incomplete, otherwise an empty string.
 * ============================================================================
 */
func checkIncompleteCallee(e *cs_callgraph.Edge) string {
    if e.Callee == nil {
        return "Callee Node is nil"
    }
    if e.Callee.IfaceMethod != nil {
        return ""
    }
    if e.Callee.Func == nil {
        return "Callee.Func is nil (unresolved dynamic call or intrinsic)"
    }
    if cs_callgraph.EffectivePkg(e.Callee.Func) == nil {
        return fmt.Sprintf(
            "Callee.Func (%s) has no resolvable package (synthetic with no origin)",
            e.Callee.Func.Name())
    }
    return ""
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
	pkgGraph 	*DotGraph			, n *cs_callgraph.Node,
	e 			*cs_callgraph.Edge	, pkgPath string,
) {
	/* -------------------------------------------------------
	 * 1. PANIC EDGE HANDLING
	 * ------------------------------------------------------- */
	if e.Kind == cs_callgraph.PanicEdge {
		handlePanicEdge(pkgGraph, n, e)
		return
	}

	/* -------------------------------------------------------
	 * 2. VALIDATION & LOGGING
	 * ------------------------------------------------------- */
	if errStr := checkIncompleteCallee(e); errStr != "" {
		return
	}
	
	/* -------------------------------------------------------
	 * 3. INTERFACE HANDLING
	 * ------------------------------------------------------- */
	if e.Callee.IfaceMethod != nil {
        handleIfaceEdge(pkgGraph, n, e, pkgPath)
        return
    }
	/* -------------------------------------------------------
	 * 4. ROOT / SPECIAL EDGE HANDLING
	 * ------------------------------------------------------- */
	if e.Callee == nil || e.Callee.Func == nil {
		handleRootEdge(pkgGraph, e)
		return
	}

	calleePkgSSA := cs_callgraph.EffectivePkg(e.Callee.Func)
    if calleePkgSSA == nil || calleePkgSSA.Pkg == nil {
        return // shouldn't reach here after checkIncompleteCallee, but be safe
    }
    calleePkg := calleePkgSSA.Pkg.Path()

	/* -------------------------------------------------------
	 * 5. INTRA-PACKAGE EDGE
	 * ------------------------------------------------------- */
	if calleePkg == pkgPath {
		handleIntraPackageEdge(pkgGraph, e)
		return
	}

	/* -------------------------------------------------------
	 * 6. INTER-PACKAGE EDGE (CLUSTER)
	 * ------------------------------------------------------- */
	handleInterPackageEdge(pkgGraph, n, e, calleePkg)
}
/* ============================================================================
 * handlePanicEdge
 * ----------------------------------------------------------------------------
 * Routes panic-kind edges to a dedicated visual "sink" node.
 * ============================================================================
 */
func handlePanicEdge(
	pkgGraph 	*DotGraph, 			n *cs_callgraph.Node, 
	e 			*cs_callgraph.Edge,
) {
	sinkID := "panic_sink"

	if _, exists := pkgGraph.Nodes[sinkID]; !exists {
		pkgGraph.Nodes[sinkID] = buildNode(
			sinkID, sinkID,
			sinkID, ns_panic,
		)
	}

	pkgGraph.Edges = append(pkgGraph.Edges, buildEdge(
		convertNodeID(n.ID, ns_normal),
		sinkID,
		mapEdgeKindToStyle(cs_callgraph.PanicEdge),
		e.Description(),
	))
}
/* ============================================================================
 * handleIfaceEdge
 * ----------------------------------------------------------------------------
 * Routes an interface dispatch edge. If the interface method lives in the
 * same package as the caller, it's intra-package. Otherwise it's treated
 * like an inter-package edge into the interface's home cluster.
 * ============================================================================
 */
func handleIfaceEdge(
	pkgGraph *DotGraph, n *cs_callgraph.Node, e *cs_callgraph.Edge, callerPkg string,
) {
	if e.Callee.IfaceMethod.Pkg() == nil {
        return
    }
    ifacePkg := e.Callee.IfaceMethod.Pkg().Path()
    ifaceNodeID := convertNodeID(e.Callee.ID, ns_interface)

    if ifacePkg == callerPkg {
        /* -------------------------------------------------------
         * Intra-package: node already registered by BuildDotGraphPerPackage,
         * just add the edge.
         * ------------------------------------------------------- */
        if _, exists := pkgGraph.Nodes[ifaceNodeID]; !exists {
            pkgGraph.Nodes[ifaceNodeID] = buildNodeFromCS(e.Callee)
        }
        pkgGraph.Edges = append(pkgGraph.Edges, buildEdge(
            convertNodeID(n.ID, ns_normal),
            ifaceNodeID,
            mapEdgeKindToStyle(e.Kind),
            e.Description(),
        ))
        return
    }

    /* -------------------------------------------------------
     * Inter-package: add interface node into the callee's cluster.
     * ------------------------------------------------------- */
    cluster := buildCluster(pkgGraph, &ifacePkg)
    if _, exists := cluster.Nodes[ifaceNodeID]; !exists {
        cluster.Nodes[ifaceNodeID] = buildNode(
            ifaceNodeID,
            e.Callee.IfaceMethod.Name(),
            e.Callee.IfaceMethod.FullName(),
            ns_interface,
        )
    }
    pkgGraph.Edges = append(pkgGraph.Edges, buildEdge(
        convertNodeID(n.ID, ns_normal),
        ifaceNodeID,
        mapEdgeKindToStyle(e.Kind),
        e.Description(),
    ))
}
/* ============================================================================
 * handleRootEdge
 * ----------------------------------------------------------------------------
 * Handles edges where the callee is the root or a special synthetic node.
 * ============================================================================
 */
func handleRootEdge(pkgGraph *DotGraph, e *cs_callgraph.Edge) {
	if e.Callee != nil {
		rootID := convertNodeID(e.Callee.ID, ns_normal)
		if _, exists := pkgGraph.Nodes[rootID]; !exists {
			pkgGraph.Nodes[rootID] = buildNodeFromCS(e.Callee)
		}
	}
	pkgGraph.Edges = append(pkgGraph.Edges, buildEdgeFromCS(e))
}

/* ============================================================================
 * handleIntraPackageEdge
 * ----------------------------------------------------------------------------
 * Handles edges where the caller and callee reside in the same package.
 * ============================================================================
 */
func handleIntraPackageEdge(pkgGraph *DotGraph, e *cs_callgraph.Edge) {
	pkgGraph.Edges = append(pkgGraph.Edges, buildEdgeFromCS(e))
}

/* ============================================================================
 * handleInterPackageEdge
 * ----------------------------------------------------------------------------
 * Handles edges that cross package boundaries by creating cluster links.
 * ============================================================================
 */
func handleInterPackageEdge(
	pkgGraph 	*DotGraph			, n *cs_callgraph.Node, 
	e 			*cs_callgraph.Edge	, calleePkg string,
) {
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
func shortFuncName(n *cs_callgraph.Node) string {
    if n.IfaceMethod != nil {
        return n.IfaceMethod.Name()
    }
    if n.Func == nil {
        return "<root>"
    }
    return n.Func.Name()
}

/* ============================================================================
 * fullFuncName
 * ----------------------------------------------------------------------------
 * Returns the full function name including package path and signature.
 * Used for tooltips to provide more detailed function information.
 * ============================================================================
 */
func fullFuncName(n *cs_callgraph.Node) string {
    if n.IfaceMethod != nil {
        return n.IfaceMethod.FullName()
    }
    if n.Func == nil {
        return "<root>"
    }
    return n.Func.String()
}

/* ============================================================================
 * convertNodeID
 * ----------------------------------------------------------------------------
 * Converts a numeric node ID into a string identifier suitable for DOT.
 * External nodes are prefixed differently to avoid collisions and allow
 * styling/grouping.
 * Interfaces nodes have their own prefix
 * ============================================================================
 */
func convertNodeID(id int, typ NodeStyle) string {
	switch typ {
	case ns_external:
		return "ext_" + strconv.Itoa(id)
	case ns_interface:
    	return "iface_" + strconv.Itoa(id)
	default:
		return "n" + strconv.Itoa(id)
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
    if n.IfaceMethod != nil {
        return buildNode(
            convertNodeID(n.ID, ns_interface),
            n.IfaceMethod.Name(),
            n.IfaceMethod.FullName(),
            ns_interface,
        )
    }
    nodeType := ns_normal
    if isAnonFunc(n) {
        nodeType = ns_anon
    }
    return buildNode(
        convertNodeID(n.ID, nodeType),
        shortFuncName(n),
        fullFuncName(n),
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
		"label":   label,
		"tooltip": tooltip,
	}

	if styleMap, ok := global_styles.NodeStyles[string(typ)]; ok {
		maps.Copy(attrs, styleMap)
	}

	return &DotNode{
		ID:    id,
		Attrs: attrs,
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
    case cs_callgraph.CallEdge      :   return es_call
    case cs_callgraph.DeferEdge     :   return es_defer
    case cs_callgraph.GoEdge        :   return es_go
    case cs_callgraph.PanicEdge     :   return es_panic
    case cs_callgraph.AssignEdge    :   return es_assign
    case cs_callgraph.SendEdge      :   return es_send
    case cs_callgraph.ReceiveEdge   :   return es_receive
    case cs_callgraph.InterfaceEdge :   return es_interface
    default                         :   return es_default
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

	cluster 	:= buildCluster(pkgGraph, calleePkg)
	extNodeID 	:= convertNodeID(e.Callee.ID, ns_external)
	
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
		"tooltip"	: *calleePkg,
		"URL"		: "pkg://" + *calleePkg,
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
