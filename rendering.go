package main

// import (
// 	"bytes"
// 	cs_callgraph "callstat/CS-Callgraph"
// 	"go/types"

// 	"golang.org/x/tools/go/callgraph"
// 	"golang.org/x/tools/go/ssa"
// )

// func printOutput(
// 	prog *ssa.Program,
// 	mainPkg *ssa.Package,
// 	cg *cs_callgraph.Graph,
// 	focusPkg *types.Package,
// 	limitPaths,
// 	ignorePaths,
// 	includePaths []string,
// 	groupBy []string,
// 	nostd,
// 	nointer bool,
// ) ([]byte, error) {
// 	var groupType, groupPkg bool
// 	for _, g := range groupBy {
// 		switch g {
// 		case "pkg":
// 			groupPkg = true
// 		case "type":
// 			groupType = true
// 		}
// 	}

// 	cluster := NewDotCluster("focus")
// 	cluster.Attrs = dotAttrs{
// 		"bgcolor":   "white",
// 		"label":     "",
// 		"labelloc":  "t",
// 		"labeljust": "c",
// 		"fontsize":  "18",
// 	}
// 	if focusPkg != nil {
// 		cluster.Attrs["bgcolor"] = "#e6ecfa"
// 		cluster.Attrs["label"] = focusPkg.Name()
// 	}

// 	var (
// 		nodes []*dotNode
// 		edges []*dotEdge
// 	)

// 	nodeMap := make(map[string]*dotNode)
// 	edgeMap := make(map[string]*dotEdge)

// 	cg.DeleteSyntheticNodes()

// 	var isFocused = func(edge *callgraph.Edge) bool {
// 		caller := edge.Caller
// 		callee := edge.Callee
// 		if focusPkg != nil && (caller.Func.Pkg.Pkg.Path() == focusPkg.Path() || callee.Func.Pkg.Pkg.Path() == focusPkg.Path()) {
// 			return true
// 		}
// 		fromFocused := false
// 		for _, e := range caller.In {
// 			if !isSynthetic(e) && focusPkg != nil && e.Caller.Func.Pkg.Pkg.Path() == focusPkg.Path() {
// 				fromFocused = true
// 				break
// 			}
// 		}
// 		toFocused := false
// 		for _, e := range callee.Out {
// 			if !isSynthetic(e) && focusPkg != nil && e.Callee.Func.Pkg.Pkg.Path() == focusPkg.Path() {
// 				toFocused = true
// 				break
// 			}
// 		}
// 		if fromFocused && toFocused {
// 			logf("edge semi-focus: %s", edge)
// 			return true
// 		}
// 		return false
// 	}

// 	var buf bytes.Buffer
// 	if err := dot.WriteDot(&buf); err != nil {
// 		return nil, err
// 	}

// 	return buf.Bytes(), nil
// }
