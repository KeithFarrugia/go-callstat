package cs_callgraph

import (
	"go/types"
)

func (g *Graph) GenIfaceNode(m *types.Func) *Node {
    if m.Pkg() == nil {
        return nil
    }
    if n, ok := g.IfaceNodes[m]; ok {
        return n
    }
    n := &Node{
        IfaceMethod: m,
        ID:          -(len(g.IfaceNodes) + 100),
    }
    g.IfaceNodes[m] = n
    return n
}