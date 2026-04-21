package cs_callgraph

import (
	"fmt"
	"go/token"

	"golang.org/x/tools/go/ssa"
)

/* ============================================================================
 * EdgeKind
 * ----------------------------------------------------------------------------
 * Represents the specific type of flow between two nodes in the call graph.
 * ============================================================================
 */
type EdgeKind int

const (
    CallEdge    EdgeKind = iota
    AssignEdge
    SendEdge
    ReceiveEdge
    GoEdge
    DeferEdge
    PanicEdge
)

/* ============================================================================
 * String (EdgeKind)
 * ----------------------------------------------------------------------------
 * Returns a human-readable string representation of the EdgeKind.
 * ============================================================================
 */
func (k EdgeKind) String() string {
    switch k {
    case CallEdge:      return "call"
    case AssignEdge:    return "assign"
    case SendEdge:      return "send"
    case ReceiveEdge:   return "receive"
    case GoEdge:        return "go"
    case DeferEdge:     return "defer"
    case PanicEdge:     return "panic"
    default:            return "unknown"
    }
}

/* ============================================================================
 * Call Tree Structures
 * ----------------------------------------------------------------------------
 * Core data structures representing the nodes, edges, and graph container.
 * ============================================================================
 */
type Node struct {
    Func *ssa.Function
    ID   int
    In   []*Edge
    Out  []*Edge
}

type Graph struct {
    Root      *Node                   // Distinguished root (Func may be nil)
    Nodes     map[*ssa.Function]*Node // All nodes indexed by SSA function
    PanicNode *Node                   // Single global sentinel sink
}

type Edge struct {
    Caller *Node
    Sites  []ssa.Instruction // Changed from 'Site ssa.Instruction'
    Callee *Node
    Kind   EdgeKind
}

/* ============================================================================
 * AnalysisCtx
 * ----------------------------------------------------------------------------
 * Contextual information used during the call graph traversal.
 * ============================================================================
 */
type AnalysisCtx struct {
    CG     *Graph
    Caller *Node
    Visit  func(*ssa.Function)
}

/* ============================================================================
 * edgeKey
 * ----------------------------------------------------------------------------
 * A unique identifier for an edge to prevent duplicate entries in the graph.
 * ============================================================================
 */
type edgeKey struct {
    from *Node
    to   *Node
    kind EdgeKind
}

/* ============================================================================
 * nodeKind
 * ----------------------------------------------------------------------------
 * A simple pair linking a target node with the type of relationship it has.
 * ============================================================================
 */
type nodeKind struct {
    node *Node
    kind EdgeKind
}

/* ============================================================================
 * InitGraph
 * ----------------------------------------------------------------------------
 * Initializes a new Call Graph and sets up the root and panic sentinel nodes.
 * ============================================================================
 */
func InitGraph(root *ssa.Function) *Graph {
    g := &Graph{
        Nodes: make(map[*ssa.Function]*Node),
    }

    g.Root = g.GenNode(root)

    // Create the sink node once. It has no ssa.Function.
    g.PanicNode = &Node{
        ID:   -99,
        Func: nil,
    }

    return g
}

/* ============================================================================
 * GenNode
 * ----------------------------------------------------------------------------
 * Returns the node for a given function, creating it if it does not exist.
 * ============================================================================
 */
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

/* ============================================================================
 * GenEdge
 * ----------------------------------------------------------------------------
 * Creates a new edge between two nodes and registers it in their in/out lists.
 * ============================================================================
 */
func GenEdge(
    caller *Node,
    site   ssa.Instruction,
    callee *Node,
    kind   EdgeKind,
) *Edge {
    e := &Edge{
        Caller: caller,
        Sites:  []ssa.Instruction{site},
        Callee: callee,
        Kind:   kind,
    }
    caller.Out = append(caller.Out, e)
    callee.In  = append(callee.In , e)
    return e
}

/* ============================================================================
 * String (Node)
 * ----------------------------------------------------------------------------
 * Returns a formatted string representing the node ID and function name.
 * ============================================================================
 */
func (n *Node) String() string {
    if n.Func == nil {
        return fmt.Sprintf("n%d:<root>", n.ID)
    }
    return fmt.Sprintf("n%d:%s", n.ID, n.Func.String())
}

/* ============================================================================
 * String (Edge)
 * ----------------------------------------------------------------------------
 * Returns a visual representation of the edge and its kind.
 * ============================================================================
 */
func (e *Edge) String() string {
    return fmt.Sprintf(
        "%s -[%s]-> %s",
        e.Caller,
        e.Kind,
        e.Callee,
    )
}

/* ============================================================================
 * Description
 * ----------------------------------------------------------------------------
 * Returns a text description of the call site, identifying special dispatch.
 * ============================================================================
 */
func (e *Edge) Description() string {
    if len(e.Sites) == 0 {
        return "synthetic edge"
    }

    var res string
    for i, site := range e.Sites {
        var prefix string
        switch site.(type) {
        case *ssa.Go:    prefix = "concurrent "
        case *ssa.Defer: prefix = "deferred "
        default:         prefix = ""
        }

        res += prefix + site.String()
        if i < len(e.Sites)-1 {
            res += "\n"
        }
    }

    return res
}
/* ============================================================================
 * Pos
 * ----------------------------------------------------------------------------
 * Returns the source code position of the first instruction in the site list.
 * Returns token.NoPos if the edge is synthetic or has no associated sites.
 * ============================================================================
 */
func (e *Edge) Pos() token.Pos {
    if len(e.Sites) == 0 {
        return token.NoPos
    }

    // Returns the position of the primary (first) call site
    if firstSite := e.Sites[0]; firstSite != nil {
        return firstSite.Pos()
    }

    return token.NoPos
}