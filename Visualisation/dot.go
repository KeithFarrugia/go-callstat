package visualisation

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

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



func (g *DotGraph) WriteDOTToFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Header
	fmt.Fprintln(f, "digraph \"\" {")
	fmt.Fprintln(f, "  rankdir=LR;")

	// Clusters
	clusterIDs := make([]string, 0, len(g.Clusters))
	for id := range g.Clusters {
		clusterIDs = append(clusterIDs, id)
	}
	sort.Strings(clusterIDs)

	for _, cid := range clusterIDs {
		c := g.Clusters[cid] // <-- this was missing
		fmt.Fprintf( f, "  subgraph %q {\n", c.ID)
		fmt.Fprintln(f, "    style=rounded;")

		// Bold label for cluster
		fmt.Fprintf(f, "    labelfontname=%q;\n", "Helvetica-Bold")
		fmt.Fprintf(f, "    label=%q;\n", c.Label)

		// Cluster attributes
		for k, v := range c.Attrs {
			fmt.Fprintf(f, "    %s=%q;\n", k, v)
		}

		// Cluster nodes
		for _, n := range c.Nodes {
			fmt.Fprintf(f, "    %q", n.ID)

			if n.Attrs == nil {
				n.Attrs = make(map[string]string)
			}

			// Solid style with light fill
			if _, ok := n.Attrs["style"]; !ok {
				n.Attrs["style"] = "solid"
			}
			if _, ok := n.Attrs["color"]; !ok {
				n.Attrs["color"] = "#444444" // border
			}
			if _, ok := n.Attrs["fillcolor"]; !ok {
				n.Attrs["fillcolor"] = "#f5e6c8" // light fill
				n.Attrs["style"] = "filled,solid"
			}

			// Tooltip shows full path
			if _, ok := n.Attrs["tooltip"]; !ok && n.Label != "" {
				n.Attrs["tooltip"] = n.Label
			}

			writeAttrs(f, n.Attrs)
			fmt.Fprintln(f, ";")
		}

		fmt.Fprintln(f, "  }")
	}
	// Nodes (stable order)
	nodeIDs := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	for _, id := range nodeIDs {
		n := g.Nodes[id]
		fmt.Fprintf(f, "  %q", n.ID)
		writeAttrs(f, n.Attrs)
		fmt.Fprintln(f, ";")
	}

	// Edges
	for _, e := range g.Edges {
		fmt.Fprintf(f, "  %q -> %q", e.From, e.To)
		writeAttrs(f, e.Attrs)
		fmt.Fprintln(f, ";")
	}

	fmt.Fprintln(f, "}")
	return nil
}

/* ============================================================================
 * Write 
 * ----------------------------------------------------------------------------
 * Wrewrite this comment but basically say that it is safe to do because the
 * attribute field is never going to be that large.
 * ============================================================================
 */



/* ============================================================================
 * Write Attibutes
 * ----------------------------------------------------------------------------
 * Wrewrite this comment but basically say that it is safe to do because the
 * attribute field is never going to be that large.
 * ============================================================================
 */

func writeAttrs(f *os.File, attrs map[string]string) {
	if len(attrs) == 0 {
		return
	}

	var att_string strings.Builder
	att_string.WriteString(" [")

	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, k := range keys {
		if i > 0 {
			att_string.WriteString(", ")
		}
		v := attrs[k]
		fmt.Fprintf(&att_string, "%s=%q", k, v)
	}

	att_string.WriteString("]")
	_, _ = f.WriteString(att_string.String())
}
