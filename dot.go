package main

import (
	"fmt"
	"os"
	"sort"
)

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

type DotGraph struct {
	Nodes map[string]*DotNode
	Edges []*DotEdge
}

func newDotGraph() *DotGraph {
	return &DotGraph{
		Nodes: make(map[string]*DotNode),
		Edges: make([]*DotEdge, 0),
	}
}

func (g *DotGraph) WriteDOTToFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Header
	fmt.Fprintln(f, "digraph G {")
	fmt.Fprintln(f, "  rankdir=LR;")

	// Nodes (stable order for diffability)
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

func writeAttrs(f *os.File, attrs map[string]string) {
	if len(attrs) == 0 {
		return
	}

	fmt.Fprint(f, " [")
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, k := range keys {
		if i > 0 {
			fmt.Fprint(f, ", ")
		}
		fmt.Fprintf(f, "%s=%q", k, attrs[k])
	}
	fmt.Fprint(f, "]")
}
