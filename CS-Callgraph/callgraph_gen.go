package cs_callgraph

import (
	"fmt"

	"golang.org/x/tools/go/ssa"
)

type AnalysisCtx struct {
	CG     *Graph
	Caller *Node
	Visit  func(*ssa.Function)
}

func handleInstruction(instr ssa.Instruction, callerNode *Node, g *Graph, visit func(*ssa.Function)) {
	switch i := instr.(type) {

	// --------------------------------------------------
	// Goroutines
	// --------------------------------------------------
	case *ssa.Go:
		call := i.Common()
		if callee := call.StaticCallee(); callee != nil {
			calleeNode := g.GenNode(callee)
			GenEdge(callerNode, i, calleeNode, GoEdge)
			visit(callee)
		}

	// --------------------------------------------------
	// Deferred calls
	// --------------------------------------------------
	case *ssa.Defer:
		call := i.Common()
		if callee := call.StaticCallee(); callee != nil {
			calleeNode := g.GenNode(callee)
			GenEdge(callerNode, i, calleeNode, DeferEdge)
			visit(callee)
		}

	// --------------------------------------------------
	// Calls
	// --------------------------------------------------
	case ssa.CallInstruction:
		call := i.Common()

		// Static call
		if callee := call.StaticCallee(); callee != nil {
			calleeNode := g.GenNode(callee)
			GenEdge(callerNode, i, calleeNode, CallEdge)
			visit(callee)
			return
		}

		// Dynamic call via function value
		if fnVal, ok := isFuncValue(call.Value); ok {
			calleeNode := g.GenNode(fnVal)
			GenEdge(callerNode, i, calleeNode, CallEdge)
		}

	// --------------------------------------------------
	// Function assignment (escaping function value)
	// --------------------------------------------------
	case *ssa.Store:
		if fnVal, ok := isFuncValue(i.Val); ok {
			calleeNode := g.GenNode(fnVal)
			GenEdge(callerNode, i, calleeNode, AssignEdge)
		}

	// --------------------------------------------------
	// Panic
	// --------------------------------------------------
	case *ssa.Panic:
		GenEdge(callerNode, i, g.Root, PanicEdge)

	// --------------------------------------------------
	// Channel send (optional modeling)
	// --------------------------------------------------
	case *ssa.Send:
		GenEdge(callerNode, i, g.Root, SendEdge)

	// --------------------------------------------------
	// Other instructions can be ignored or handled later
	// --------------------------------------------------
	default:
		// no-op
	}
}

func isFuncValue(v ssa.Value) (*ssa.Function, bool) {
	switch v := v.(type) {
	case *ssa.Function:
		return v, true

	case *ssa.MakeClosure:
		if fn, ok := v.Fn.(*ssa.Function); ok {
			return fn, true
		}
	}
	return nil, false
}

func analyseFunction(fn *ssa.Function) {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {

			switch i := instr.(type) {

			// Anything that can invoke code
			case ssa.CallInstruction:
				call := i.Common()
				if callee := call.StaticCallee(); callee != nil {
					fmt.Printf("    invoke -> %s\n", callee.String())
				} else {
					fmt.Printf("    invoke -> dynamic: %s\n", call.Value)
				}

			// Abnormal control flow
			case *ssa.Panic:
				fmt.Printf("    panic  -> %s\n", i.X)

			// (Optional) channel, goroutine, or other effects
			case *ssa.Send:
				fmt.Printf("    send   -> %s\n", i.Chan)

			case *ssa.Select:
				fmt.Printf("    select\n")

			case *ssa.Store:
				if fnVal, ok := isFuncValue(i.Val); ok {
					fmt.Printf(
						"    assign -> %s assigns function %s\n",
						fn.String(),
						fnVal.String(),
					)
				}
			}
		}
	}
}

type edgeKey struct {
	from *Node
	to   *Node
	kind EdgeKind
}

// BuildExtendedCallGraph builds the full call graph for a program.
func BuildExtendedCallGraph(prog *ssa.Program) *Graph {
	cg := InitGraph(nil)
	seen := map[*ssa.Function]bool{}
	seenEdges := map[edgeKey]bool{}

	// Recursive visit
	var visit func(fn *ssa.Function)
	visit = func(fn *ssa.Function) {
		if fn == nil || seen[fn] {
			return
		}
		seen[fn] = true

		callerNode := cg.GenNode(fn)

		// Iterate instructions
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				calleeNode, kind := extractCalleeAndKind(cg, instr)
				if calleeNode == nil {
					continue
				}

				key := edgeKey{from: callerNode, to: calleeNode, kind: kind}
				if !seenEdges[key] {
					GenEdge(callerNode, instr, calleeNode, kind)
					seenEdges[key] = true
				}

				// Recursively visit callee functions (including closures)
				if calleeNode.Func != nil && calleeNode.Func != fn {
					visit(calleeNode.Func)
				}
			}
		}
	}

	// Start from all package-level functions
	for _, pkg := range prog.AllPackages() {
		for _, mem := range pkg.Members {
			if fn, ok := mem.(*ssa.Function); ok {
				visit(fn)
			}
		}
	}

	return cg
}

// extractCalleeAndKind returns the callee Node and edge kind for an instruction
func extractCalleeAndKind(cg *Graph, instr ssa.Instruction) (*Node, EdgeKind) {
	switch i := instr.(type) {

	case *ssa.Go:
		call := i.Common()
		if callee := call.StaticCallee(); callee != nil {
			return cg.GenNode(callee), GoEdge
		}

	case *ssa.Defer:
		call := i.Common()
		if callee := call.StaticCallee(); callee != nil {
			return cg.GenNode(callee), DeferEdge
		}

	case ssa.CallInstruction:
		call := i.Common()

		// Static call
		if callee := call.StaticCallee(); callee != nil {
			return cg.GenNode(callee), CallEdge
		}

		// Dynamic call via function value (closure)
		if fnVal, ok := isFuncValue(call.Value); ok {
			return cg.GenNode(fnVal), CallEdge
		}

	case *ssa.Panic:
		return cg.Root, PanicEdge

	case *ssa.Send:
		return cg.Root, SendEdge
	}

	return nil, 0
}
