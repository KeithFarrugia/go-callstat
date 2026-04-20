package visualisation

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

/* ============================================================================
 * DotGraph Writing
 * ============================================================================
 */

/* ============================================================================
 * WriteDOTToFile
 * ----------------------------------------------------------------------------
 * Writes the entire DOT graph to a file, including clusters, nodes, and edges.
 * ============================================================================
 */
func (g *DotGraph) WriteDOTToFile(filename string) error {
	/* -------------------------------------------------------
	 * Open file
	 * ------------------------------------------------------- */
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	/* -------------------------------------------------------
	 * Graph header
	 * ------------------------------------------------------- */
	writeGraphHeader(f)

	/* -------------------------------------------------------
	 * Clusters
	 * ------------------------------------------------------- */
	clusterIDs := make([]string, 0, len(g.Clusters))
	for id := range g.Clusters {
		clusterIDs = append(clusterIDs, id)
	}
	sort.Strings(clusterIDs)
	for _, cid := range clusterIDs {
		writeCluster(g.Clusters[cid], f)
	}

	/* -------------------------------------------------------
	 * Nodes (outside clusters)
	 * ------------------------------------------------------- */
	nodeIDs := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)
	for _, id := range nodeIDs {
		writeNode(g.Nodes[id], f)
	}

	/* -------------------------------------------------------
	 * Edges
	 * ------------------------------------------------------- */
	for _, e := range g.Edges {
		writeEdge(e, f)
	}

	/* -------------------------------------------------------
	 * Graph footer
	 * ------------------------------------------------------- */
	fmt.Fprintln(f, "}")
	return nil
}

/* ============================================================================
 * writeGraphHeader
 * ----------------------------------------------------------------------------
 * Writes the opening DOT graph declaration and global graph settings.
 * ============================================================================
 */
func writeGraphHeader(f *os.File) {
	fmt.Fprintln(f, "digraph \"\" {")
	fmt.Fprintln(f, "  rankdir=LR;")
}

/* ============================================================================
 * writeCluster
 * ----------------------------------------------------------------------------
 * Writes a cluster using its attributes. All styling is pre-configured.
 * ============================================================================
 */
func writeCluster(c *DotCluster, f *os.File) {
	fmt.Fprintf(f, "  subgraph %q {\n", c.ID)

	/* -------------------------------------------------------
	 * Cluster label (structural, not styling)
	 * ------------------------------------------------------- */
	if c.Label != "" {
		fmt.Fprintf(f, "    label=%q;\n", c.Label)
	}

	/* -------------------------------------------------------
	 * Cluster attributes (fully style-driven)
	 * ------------------------------------------------------- */
	writeAttrsGeneric(f, c.Attrs, false, 4)

	/* -------------------------------------------------------
	 * Cluster nodes
	 * ------------------------------------------------------- */
	for _, n := range c.Nodes {
		writeClusterNodeToDot(n, f)
	}

	fmt.Fprintln(f, "  }")
}

/* ============================================================================
 * writeClusterNodeToDot
 * ----------------------------------------------------------------------------
 * Writes a node inside a cluster. Assumes styling has been pre-applied.
 * ============================================================================
 */
func writeClusterNodeToDot(n *DotNode, f *os.File) {
	fmt.Fprintf(f, "    %q", n.ID)
	writeAttrsGeneric(f, n.Attrs, true, 0)
	fmt.Fprintln(f, ";")
}

/* ============================================================================
 * writeNode
 * ----------------------------------------------------------------------------
 * Writes a node outside clusters.
 * ============================================================================
 */
func writeNode(n *DotNode, f *os.File) {
	fmt.Fprintf(f, "  %q", n.ID)
	writeAttrsGeneric(f, n.Attrs, true, 0)
	fmt.Fprintln(f, ";")
}

/* ============================================================================
 * writeEdge
 * ----------------------------------------------------------------------------
 * Writes a directed edge.
 * ============================================================================
 */
func writeEdge(e *DotEdge, f *os.File) {
	fmt.Fprintf(f, "  %q -> %q", e.From, e.To)
	writeAttrsGeneric(f, e.Attrs, true, 0)
	fmt.Fprintln(f, ";")
}

/* ============================================================================
 * writeAttrsGeneric
 * ----------------------------------------------------------------------------
 * Writes attributes to file.
 *
 * inline: true  -> write as [key="val", key="val"]
 * inline: false -> write indented, one per line
 * indent : number of spaces if not inline
 *
 * Safe to sort keys as attribute maps are small.
 * ============================================================================
 */
func writeAttrsGeneric(
	f 			*os.File, attrs map[string]string, 
	inline 		bool	, indent int,
) {
	if len(attrs) == 0 {
		return
	}

	/* -------------------------------------------------------
	 * Sort keys
	 * ------------------------------------------------------- */
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if inline {
		/* -------------------------------------------------------
		 * Inline format [key="val", key="val"]
		 * ------------------------------------------------------- */
		var b strings.Builder
		b.WriteString(" [")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s=%q", k, attrs[k])
		}
		b.WriteString("]")
		_, _ = f.WriteString(b.String())
	} else {
		/* -------------------------------------------------------
		 * Indented format for clusters
		 * ------------------------------------------------------- */
		padding := strings.Repeat(" ", indent)
		for _, k := range keys {
			fmt.Fprintf(f, "%s%s=%q;\n", padding, k, attrs[k])
		}
	}
}