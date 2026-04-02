package visualisation

/* ============================================================================
 * Dot data strucutres
 * ----------------------------------------------------------------------------
 * Structs used to define how each of these datastructures should be formed.
 * This includes nodes Edges clusters and the like
 * ============================================================================
 */
type DotGraph struct {
	Nodes    map[string]*   DotNode
	Edges    []*            DotEdge
	Clusters map[string]*   DotCluster
}

type DotNode struct {
	ID    string
	Label string
	Attrs map[string]string
}

type DotEdge struct {
	From  string
	To    string
	Attrs map[string]string
}

type DotCluster struct {
	ID    string
	Label string
	Attrs map[string]string
	Nodes map[string]*DotNode
}


/* ============================================================================
 * 
 * ----------------------------------------------------------------------------
 * 
 * ============================================================================
 */
func newDotGraph() *DotGraph {
	return &DotGraph{
		Nodes:    make(map[string]*DotNode),
		Edges:    make([]*DotEdge, 0),
		Clusters: make(map[string]*DotCluster),
	}
}
