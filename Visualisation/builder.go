package visualisation

import (
	cs_callgraph "callstat/CS-Callgraph"
	"fmt"
)

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

func convertNodeID(id int, typ NodeStyle) string{
    switch typ{
        case ns_external    : return fmt.Sprintf("ext_%d"   , id);
        default             : return fmt.Sprintf("n%d"      , id);
    }
}



/* ============================================================================
 * ApplyNodeStyle
 * ----------------------------------------------------------------------------
 * Injects unique node data (label/tooltip) and merges all style attributes 
 * defined in the global config for the specified type.
 * ============================================================================
 */
func buildNodeFromCS(n *cs_callgraph.Node) *DotNode {
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

func buildNode(
    id      string  , label string, 
    tooltip string  , typ   NodeStyle,
) *DotNode {
    attrs := make(map[string]string)
    if global_styles == nil {
        panic("Visualisation Error: global_styles is nil. Did you call LoadInternalStyles()?")
    }
    attrs["label"  ]	= label
    attrs["tooltip"] 	= tooltip
    
    if styleMap, ok := global_styles.NodeStyles[string(typ)]; ok {
        for k, v := range styleMap {
            attrs[k] = v
        }
    }

    return &DotNode{
        ID		: id,
        Attrs	: attrs,
    }
}

/* ============================================================================
 * ApplyEdgeStyle
 * ----------------------------------------------------------------------------
 * Sets the edge tooltip based on call site and merges all attributes 
 * (color, style, arrowhead) from the global config for the edge type.
 * ============================================================================
 */

func mapEdgeKindToStyle (typ cs_callgraph.EdgeKind) EdgeStyle {
    switch typ {
        case cs_callgraph.CallEdge		: 	return es_call;
        case cs_callgraph.DeferEdge		:	return es_defer;
        case cs_callgraph.GoEdge		:	return es_go;
        case cs_callgraph.PanicEdge		:	return es_panic;
        default                         :   return es_default;
    }
} 


func buildEdgeFromCS(
    e *cs_callgraph.Edge,
) *DotEdge {
    return buildEdge(
        convertNodeID       (e.Caller.ID, ns_normal),
        convertNodeID       (e.Callee.ID, ns_normal),
        mapEdgeKindToStyle  (e.Kind),
        e.Description       (),
    )
}

func buildEdge(
    from    string      , to    string, 
    typ     EdgeStyle   , des   string,
) *DotEdge {
    attrs := make(map[string]string)
    attrs["tooltip"] = des

    if styleMap, ok := global_styles.EdgeStyles[string(typ)]; ok {
        for k, v := range styleMap {
            attrs[k] = v
        }
    }

    return &DotEdge{
        From    :   from,
        To      :   to,
        Attrs   :   attrs,
    }
}


/* ============================================================================
 * Build Cluster Node
 * ----------------------------------------------------------------------------
 * If we have a node n1 and it calls a node n2. In the case that n2 is not
 * in the same package as n1 then we add it to a cluster in n2. This is so,
 * when we visualise them all functions belonging to the same external package
 * are grouped together.
 * To do this we first make sure the cluster or group exists. then we add the
 * link between n1 and n2 and add n2 to the cluster.
 * ============================================================================
 */
func buildLinkClusterNode(
    pkgGraph    *DotGraph           , calleePkg *string, 
    e           *cs_callgraph.Edge  , n         *cs_callgraph.Node,
) {
    cluster     := buildCluster     (pkgGraph   , calleePkg);
    extNodeID   := convertNodeID    (e.Callee.ID, ns_external)

    if _, exists := cluster.Nodes[extNodeID]; !exists {
		cluster.Nodes[extNodeID] = buildNode(
			extNodeID,
			shortFuncName(e.Callee),
			fullFuncName(e.Callee),
			ns_external,
		)
	}

    // Add the Edge
    pkgGraph.Edges = append(pkgGraph.Edges, buildEdge(
        convertNodeID (n.ID, ns_normal) ,   
        extNodeID                       ,
        mapEdgeKindToStyle(e.Kind)      , 
        e.Description()                 ,
    ))
}





/* ============================================================================
 * Build Cluster
 * ----------------------------------------------------------------------------
 * Lets say we are traversing a callgraph node, and one of the functions it
 * calls belongs to a package different then the node.
 * In the visualisation we group these functions inside a box. The box is
 * labeled with this external package so that all these nodes to this package
 * are in one place.
 * This function makes sure to create this cluster. It generates the dot file
 * for it if no dot file config exists.
 * ============================================================================
 */

func buildCluster(pkgGraph *DotGraph, calleePkg *string) *DotCluster {
    clusterID := fmt.Sprintf("cluster_%s", *calleePkg)
    cluster, exists := pkgGraph.Clusters[clusterID]
    
    if !exists {
        
        /* -------------------------------------------------------
         * Set Tool Tip
         * ------------------------------------------------------- */
        attrs := map[string]string{
            "tooltip": *calleePkg,
        }

        /* -------------------------------------------------------
         * Loop through all the attributes in the config
         * ------------------------------------------------------- */
        for k, v := range global_styles.Cluster {
            attrs[k] = v
        }
        
        
        /* -------------------------------------------------------
         * Build Cluster Object
         * ------------------------------------------------------- */
        cluster = &DotCluster{
            ID:    clusterID,
            Label: shortPkgName(*calleePkg),
            Attrs: attrs,
            Nodes: make(map[string]*DotNode),
        }

        pkgGraph.Clusters[clusterID] = cluster
    }
    return cluster
}
