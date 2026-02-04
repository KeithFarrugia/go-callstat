package main

import (
	cs_callgraph "callstat/CS-Callgraph"
	"fmt"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// --- Dot model --------------------------------------------------------------

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

// --- Filtering --------------------------------------------------------------

type EdgeFilter struct {
	FocusPkg     *types.Package
	LimitPaths   []string
	IgnorePaths  []string
	IncludePaths []string
	NoStd        bool
	NoInter      bool
}

// Allow returns true if the edge should be included according to the filter.
func (f *EdgeFilter) Allow(e *cs_callgraph.Edge) bool {
	if e == nil || e.Caller == nil || e.Callee == nil {
		return false
	}
	if e.Caller.Func == nil || e.Callee.Func == nil {
		return false
	}

	// Focus on main package
	if f.FocusPkg != nil && !f.isFocused(e) {
		return false
	}

	if f.NoStd && (inStdCS(e.Caller) || inStdCS(e.Callee)) {
		return false
	}

	return true
}
func (f *EdgeFilter) isFocused(edge *cs_callgraph.Edge) bool {
	if edge == nil || edge.Caller == nil || edge.Callee == nil {
		return false
	}
	if edge.Caller.Func == nil || edge.Callee.Func == nil {
		return false
	}
	// Only include edges where the caller is in FocusPkg
	return f.FocusPkg != nil && edge.Caller.Func.Pkg.Pkg.Path() == f.FocusPkg.Path()
}

func (f *EdgeFilter) isInter(e *cs_callgraph.Edge) bool {
	if e == nil || e.Callee == nil || e.Callee.Func == nil {
		return false
	}
	if e.Callee.Func.Object() != nil && !e.Callee.Func.Object().Exported() {
		return true
	}
	return false
}

func (f *EdgeFilter) inIgnoreTypes(n *cs_callgraph.Node) bool {
	if n == nil || n.Func == nil {
		return false
	}
	params := n.Func.Params
	if len(params) == 0 || params[0] == nil {
		return false
	}
	t := params[0].Type().String()
	for _, ig := range f.IgnorePaths {
		if t == ig {
			return true
		}
	}
	return false
}

// --- Helpers ---------------------------------------------------------------

func hasPrefix(n *cs_callgraph.Node, paths []string) bool {
	if n == nil || n.Func == nil || n.Func.Pkg == nil || n.Func.Pkg.Pkg == nil {
		return false
	}
	pkgPath := n.Func.Pkg.Pkg.Path()
	for _, pref := range paths {
		if strings.HasPrefix(pkgPath, pref) {
			return true
		}
	}
	return false
}

func inStdCS(n *cs_callgraph.Node) bool {
	if n == nil || n.Func == nil || n.Func.Pkg == nil || n.Func.Pkg.Pkg == nil {
		return false
	}
	return isStdPkgPath(n.Func.Pkg.Pkg.Path())
}

// isStdPkgPath heuristically treats a path without a '.' as stdlib (common heuristic).
// Standard-library paths like "fmt", "net/http" have no domain component; third-party modules normally contain a dot.
func isStdPkgPath(pkgPath string) bool {
	// If there is a '.' in the path (e.g. "github.com/..." or "golang.org/..."), we treat it as non-stdlib.
	return !strings.Contains(pkgPath, ".")
}

// --- Rendering -------------------------------------------------------------

// RenderGraphviz traverses the cs_callgraph and produces a Graphviz DOT string.
// pass in a filter (EdgeFilter) to control focus/limit/include/ignore/std/inter behaviour.
func RenderGraphviz(
	prog *ssa.Program,
	cg *cs_callgraph.Graph,
	filter *EdgeFilter,
) (string, error) {

	dg := newDotGraph()

	for _, n := range cg.Nodes {
		for _, e := range n.Out {
			if !filter.Allow(e) {
				continue
			}

			addNode(dg, prog, e.Caller, true, filter)
			addNode(dg, prog, e.Callee, false, filter)
			addEdge(dg, prog, e, filter)
		}
	}

	return writeDOT(dg), nil
}

func addNode(
	g *DotGraph,
	prog *ssa.Program,
	n *cs_callgraph.Node,
	isCaller bool,
	filter *EdgeFilter,
) {
	if n == nil || n.Func == nil {
		return
	}
	id := n.Func.String()
	if _, ok := g.Nodes[id]; ok {
		return
	}

	pos := prog.Fset.Position(n.Func.Pos())
	label := n.Func.RelString(n.Func.Pkg.Pkg)

	attrs := map[string]string{
		"label":   label,
		"tooltip": fmt.Sprintf("%s:%d", filepath.Base(pos.Filename), pos.Line),
		"shape":   "box",
	}

	if filter != nil && filter.FocusPkg != nil && n.Func.Pkg.Pkg.Path() == filter.FocusPkg.Path() {
		attrs["style"] = "filled"
		attrs["fillcolor"] = "lightblue"
	}

	g.Nodes[id] = &DotNode{
		ID:    id,
		Label: label,
		Attrs: attrs,
	}
}

func addEdge(
	g *DotGraph,
	prog *ssa.Program,
	e *cs_callgraph.Edge,
	filter *EdgeFilter,
) {
	if e == nil || e.Caller == nil || e.Callee == nil {
		return
	}
	pos := prog.Fset.Position(e.Pos())

	attrs := map[string]string{
		"tooltip": fmt.Sprintf("%s:%d: calling [%s]", filepath.Base(pos.Filename), pos.Line, e.Callee.Func.String()),
	}

	switch e.Site.(type) {
	case *ssa.Go:
		attrs["arrowhead"] = "dot"
	case *ssa.Defer:
		attrs["arrowhead"] = "diamond"
	}

	g.Edges = append(g.Edges, &DotEdge{
		From:  e.Caller.Func.String(),
		To:    e.Callee.Func.String(),
		Attrs: attrs,
	})
}

// --- DOT output -----------------------------------------------------------

func writeDOT(g *DotGraph) string {
	var b strings.Builder

	b.WriteString("digraph G {\n")
	b.WriteString("  rankdir=LR;\n")

	for _, n := range g.Nodes {
		b.WriteString(fmt.Sprintf("  %q", n.ID))
		writeAttrs(&b, n.Attrs)
		b.WriteString(";\n")
	}

	for _, e := range g.Edges {
		b.WriteString(fmt.Sprintf("  %q -> %q", e.From, e.To))
		writeAttrs(&b, e.Attrs)
		b.WriteString(";\n")
	}

	b.WriteString("}\n")
	return b.String()
}

func writeAttrs(b *strings.Builder, attrs map[string]string) {
	if len(attrs) == 0 {
		return
	}
	b.WriteString(" [")
	i := 0
	// note: map iteration order is random; that's fine for DOT but if you need deterministic output,
	// collect keys and sort them here.
	for k, v := range attrs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("%s=%q", k, v))
		i++
	}
	b.WriteString("]")
}
