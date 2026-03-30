package main

import (
	cs_callgraph "callstat/CS-Callgraph"
	"fmt"
	"strings"
)

type EdgeStyle struct {
	Color     string
	Style     string
	ArrowHead string
	Label     string
}

var EdgeStyles = map[cs_callgraph.EdgeKind]EdgeStyle{
	cs_callgraph.CallEdge: {
		Color:     "#2b2b2b",
		Style:     "solid",
		ArrowHead: "normal",
		Label:     "call",
	},
	cs_callgraph.GoEdge: {
		Color:     "#1f77b4",
		Style:     "dashed",
		ArrowHead: "dot",
		Label:     "go",
	},
	cs_callgraph.DeferEdge: {
		Color:     "#9467bd",
		Style:     "dotted",
		ArrowHead: "diamond",
		Label:     "defer",
	},
	cs_callgraph.PanicEdge: {
		Color:     "#d62728",
		Style:     "bold",
		ArrowHead: "tee",
		Label:     "panic",
	},
}

type NodeStyle struct {
	Shape string
	Style string
	Color string
}

var (
	NormalFuncNode = NodeStyle{
		Shape: "box",
		Style: "solid",
		Color: "#484061",
	}

	AnonFuncNode = NodeStyle{
		Shape: "box",
		Style: "dashed",
		Color: "#6c3636",
	}
)

func isAnonFunc(fn *cs_callgraph.Node) bool {
	if fn.Func == nil {
		return false
	}
	return strings.Contains(fn.Func.Name(), "$")
}

func shortFuncName(fn *cs_callgraph.Node) string {
	if fn.Func == nil {
		return "<root>"
	}
	return fn.Func.Name()
}

func fullFuncName(fn *cs_callgraph.Node) string {
	if fn.Func == nil {
		return "<root>"
	}
	return fn.Func.String()
}

func dotNodeFromCS(n *cs_callgraph.Node) *DotNode {
	style := NormalFuncNode
	if isAnonFunc(n) {
		style = AnonFuncNode
	}

	attrs := map[string]string{
		"shape": style.Shape,
		"style": style.Style,
		"color": style.Color,
		"label": shortFuncName(n),
	}

	// Tooltip shows full path + signature
	if n.Func != nil {
		attrs["tooltip"] = fullFuncName(n)
	}

	return &DotNode{
		ID:    fmt.Sprintf("n%d", n.ID),
		Attrs: attrs,
	}
}

func dotEdgeFromCS(e *cs_callgraph.Edge) *DotEdge {
	style, ok := EdgeStyles[e.Kind]
	if !ok {
		style = EdgeStyle{Color: "black", Style: "solid", ArrowHead: "normal"}
	}

	attrs := map[string]string{
		"color":     style.Color,
		"style":     style.Style,
		"arrowhead": style.ArrowHead,
		"label":     style.Label,
	}

	if e.Site != nil {
		attrs["tooltip"] = e.Description()
	}

	return &DotEdge{
		From:  fmt.Sprintf("n%d", e.Caller.ID),
		To:    fmt.Sprintf("n%d", e.Callee.ID),
		Attrs: attrs,
	}
}

func BuildDotGraphFromCS(g *cs_callgraph.Graph) *DotGraph {
	dg := newDotGraph()

	// Nodes
	for _, n := range g.Nodes {
		dg.Nodes[fmt.Sprintf("n%d", n.ID)] = dotNodeFromCS(n)
	}

	// Edges
	for _, n := range g.Nodes {
		for _, e := range n.Out {
			dg.Edges = append(dg.Edges, dotEdgeFromCS(e))
		}
	}

	return dg
}

// // --- Filtering --------------------------------------------------------------

// type EdgeFilter struct {
// 	FocusPkg     *types.Package
// 	LimitPaths   []string
// 	IgnorePaths  []string
// 	IncludePaths []string
// 	NoStd        bool
// 	NoInter      bool
// }

// // Allow returns true if the edge should be included according to the filter.
// func (f *EdgeFilter) Allow(e *cs_callgraph.Edge) bool {
// 	if e == nil || e.Caller == nil || e.Callee == nil {
// 		return false
// 	}
// 	if e.Caller.Func == nil || e.Callee.Func == nil {
// 		return false
// 	}

// 	// Focus on main package
// 	if f.FocusPkg != nil && !f.isFocused(e) {
// 		return false
// 	}

// 	if f.NoStd && (inStdCS(e.Caller) || inStdCS(e.Callee)) {
// 		return false
// 	}

// 	return true
// }
// func (f *EdgeFilter) isFocused(edge *cs_callgraph.Edge) bool {
// 	if edge == nil || edge.Caller == nil || edge.Callee == nil {
// 		return false
// 	}
// 	if edge.Caller.Func == nil || edge.Callee.Func == nil {
// 		return false
// 	}
// 	// Only include edges where the caller is in FocusPkg
// 	return f.FocusPkg != nil && edge.Caller.Func.Pkg.Pkg.Path() == f.FocusPkg.Path()
// }

// func (f *EdgeFilter) isInter(e *cs_callgraph.Edge) bool {
// 	if e == nil || e.Callee == nil || e.Callee.Func == nil {
// 		return false
// 	}
// 	if e.Callee.Func.Object() != nil && !e.Callee.Func.Object().Exported() {
// 		return true
// 	}
// 	return false
// }

// func (f *EdgeFilter) inIgnoreTypes(n *cs_callgraph.Node) bool {
// 	if n == nil || n.Func == nil {
// 		return false
// 	}
// 	params := n.Func.Params
// 	if len(params) == 0 || params[0] == nil {
// 		return false
// 	}
// 	t := params[0].Type().String()
// 	for _, ig := range f.IgnorePaths {
// 		if t == ig {
// 			return true
// 		}
// 	}
// 	return false
// }

// // --- Helpers ---------------------------------------------------------------

// func hasPrefix(n *cs_callgraph.Node, paths []string) bool {
// 	if n == nil || n.Func == nil || n.Func.Pkg == nil || n.Func.Pkg.Pkg == nil {
// 		return false
// 	}
// 	pkgPath := n.Func.Pkg.Pkg.Path()
// 	for _, pref := range paths {
// 		if strings.HasPrefix(pkgPath, pref) {
// 			return true
// 		}
// 	}
// 	return false
// }

// func inStdCS(n *cs_callgraph.Node) bool {
// 	if n == nil || n.Func == nil || n.Func.Pkg == nil || n.Func.Pkg.Pkg == nil {
// 		return false
// 	}
// 	return isStdPkgPath(n.Func.Pkg.Pkg.Path())
// }

// // isStdPkgPath heuristically treats a path without a '.' as stdlib (common heuristic).
// // Standard-library paths like "fmt", "net/http" have no domain component; third-party modules normally contain a dot.
// func isStdPkgPath(pkgPath string) bool {
// 	// If there is a '.' in the path (e.g. "github.com/..." or "golang.org/..."), we treat it as non-stdlib.
// 	return !strings.Contains(pkgPath, ".")
// }

// // --- Rendering -------------------------------------------------------------

// // RenderGraphviz traverses the cs_callgraph and produces a Graphviz DOT string.
// // pass in a filter (EdgeFilter) to control focus/limit/include/ignore/std/inter behaviour.
// func RenderGraphviz(
// 	prog *ssa.Program,
// 	cg *cs_callgraph.Graph,
// 	filter *EdgeFilter,
// ) (string, error) {

// 	dg := newDotGraph()

// 	for _, n := range cg.Nodes {
// 		for _, e := range n.Out {
// 			if !filter.Allow(e) {
// 				continue
// 			}

// 			addNode(dg, prog, e.Caller, true, filter)
// 			addNode(dg, prog, e.Callee, false, filter)
// 			addEdge(dg, prog, e, filter)
// 		}
// 	}

// 	return writeDOT(dg), nil
// }

// func addNode(
// 	g *DotGraph,
// 	prog *ssa.Program,
// 	n *cs_callgraph.Node,
// 	isCaller bool,
// 	filter *EdgeFilter,
// ) {
// 	if n == nil || n.Func == nil {
// 		return
// 	}
// 	id := n.Func.String()
// 	if _, ok := g.Nodes[id]; ok {
// 		return
// 	}

// 	pos := prog.Fset.Position(n.Func.Pos())
// 	label := n.Func.RelString(n.Func.Pkg.Pkg)

// 	attrs := map[string]string{
// 		"label":   label,
// 		"tooltip": fmt.Sprintf("%s:%d", filepath.Base(pos.Filename), pos.Line),
// 		"shape":   "box",
// 	}

// 	if filter != nil && filter.FocusPkg != nil && n.Func.Pkg.Pkg.Path() == filter.FocusPkg.Path() {
// 		attrs["style"] = "filled"
// 		attrs["fillcolor"] = "lightblue"
// 	}

// 	g.Nodes[id] = &DotNode{
// 		ID:    id,
// 		Label: label,
// 		Attrs: attrs,
// 	}
// }

// func addEdge(
// 	g *DotGraph,
// 	prog *ssa.Program,
// 	e *cs_callgraph.Edge,
// 	filter *EdgeFilter,
// ) {
// 	if e == nil || e.Caller == nil || e.Callee == nil {
// 		return
// 	}
// 	pos := prog.Fset.Position(e.Pos())

// 	attrs := map[string]string{
// 		"tooltip": fmt.Sprintf("%s:%d: calling [%s]", filepath.Base(pos.Filename), pos.Line, e.Callee.Func.String()),
// 	}

// 	switch e.Site.(type) {
// 	case *ssa.Go:
// 		attrs["arrowhead"] = "dot"
// 	case *ssa.Defer:
// 		attrs["arrowhead"] = "diamond"
// 	}

// 	g.Edges = append(g.Edges, &DotEdge{
// 		From:  e.Caller.Func.String(),
// 		To:    e.Callee.Func.String(),
// 		Attrs: attrs,
// 	})
// }
