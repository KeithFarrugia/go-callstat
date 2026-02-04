package cs_callgraph

import (
	"fmt"
	"go/token"

	"golang.org/x/tools/go/ssa"
)

type EdgeKind int

const (
	CallEdge EdgeKind = iota
	AssignEdge
	SendEdge
	ReceiveEdge
	GoEdge
	DeferEdge
	PanicEdge
)

func (k EdgeKind) String() string {
	switch k {
	case CallEdge:
		return "call"
	case AssignEdge:
		return "assign"
	case SendEdge:
		return "send"
	case ReceiveEdge:
		return "receive"
	case GoEdge:
		return "go"
	case DeferEdge:
		return "defer"
	case PanicEdge:
		return "panic"
	default:
		return "unknown"
	}
}

/* ============================================================================
 * Call Tree Structer
 * ============================================================================
 */
type Node struct {
	Func *ssa.Function
	ID   int
	In   []*Edge
	Out  []*Edge
}

type Graph struct {
	Root  *Node                   // the distinguished root node (Root.Func may be nil)
	Nodes map[*ssa.Function]*Node // all nodes by function
}

type Edge struct {
	Caller *Node
	Site   ssa.Instruction
	Callee *Node
	Kind   EdgeKind
}

/* ============================================================================
 * Graph Construction
 * ============================================================================
 */

// New creates a new graph with an optional root function.
func InitGraph(root *ssa.Function) *Graph {
	g := &Graph{
		Nodes: make(map[*ssa.Function]*Node),
	}
	g.Root = g.GenNode(root)
	return g
}

// CreateNode returns the node for fn, creating it if necessary.
func (g *Graph) GenNode(fn *ssa.Function) *Node {
	n, ok := g.Nodes[fn]
	if !ok {
		n = &Node{
			Func: fn,
			ID:   len(g.Nodes),
		}
		g.Nodes[fn] = n
	}
	return n
}

func GenEdge(
	caller *Node,
	site ssa.Instruction,
	callee *Node,
	kind EdgeKind,
) {
	e := &Edge{
		Caller: caller,
		Site:   site,
		Callee: callee,
		Kind:   kind,
	}
	caller.Out = append(caller.Out, e)
	callee.In = append(callee.In, e)
}

/* ============================================================================
 * Edge Management
 * ============================================================================
 */

// A Node represents a node in a call graph.

func (n *Node) String() string {
	if n.Func == nil {
		return fmt.Sprintf("n%d:<root>", n.ID)
	}
	return fmt.Sprintf("n%d:%s", n.ID, n.Func.String())
}

func (e *Edge) String() string {
	return fmt.Sprintf(
		"%s -[%s]-> %s",
		e.Caller,
		e.Kind,
		e.Callee,
	)
}

func (e *Edge) Description() string {
	if e.Site == nil {
		return "synthetic edge"
	}

	switch e.Site.(type) {
	case *ssa.Go:
		return "concurrent " + e.Site.String()
	case *ssa.Defer:
		return "deferred " + e.Site.String()
	}
	return e.Site.String()
}

func (e *Edge) Pos() token.Pos {
	if e.Site == nil {
		return token.NoPos
	}
	return e.Site.Pos()
}
