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

	// case *ssa.Go:
	// 	call := i.Common()
	// 	if callee := call.StaticCallee(); callee != nil {
	// 		return cg.GenNode(callee), GoEdge
	// 	}

	// case *ssa.Defer:
	// 	call := i.Common()
	// 	if callee := call.StaticCallee(); callee != nil {
	// 		return cg.GenNode(callee), DeferEdge
	// 	}

	// case ssa.CallInstruction:
	// 	call := i.Common()

	// 	// Static call
	// 	if callee := call.StaticCallee(); callee != nil {
	// 		return cg.GenNode(callee), CallEdge
	// 	}

	// 	// Dynamic call via function value (closure)
	// 	if fnVal, ok := isFuncValue(call.Value); ok {
	// 		return cg.GenNode(fnVal), CallEdge
	// 	}

	// case *ssa.Panic:
	// 	return cg.Root, PanicEdge

	// case *ssa.Send:
	// 	return cg.Root, SendEdge

	case *ssa.Store:
		fmt.Printf("Store Instruction \n\t %s\n", i.String())
        if fnVal, ok := isFuncValue(i.Val); ok {
            return cg.GenNode(fnVal), AssignEdge
        }
    default :
		fmt.Printf("Instruction \n\t %s\n \t Type: %s \n", instr.String(), instr.Parent().Type())
	}
	return nil, 0
}


type nodeKind struct {
	node *Node
	kind EdgeKind
}
func extractEdges(cg *Graph, instr ssa.Instruction) []nodeKind {
	fmt.Printf("Instruction\n \t%s\n", instr.String())
	switch i := instr.(type) {

	case *ssa.Go:
		call := i.Common()
		if callee := call.StaticCallee(); callee != nil {
			return []nodeKind{{cg.GenNode(callee), GoEdge}}
		}

	case *ssa.Defer:
		call := i.Common()
		if callee := call.StaticCallee(); callee != nil {
			return []nodeKind{{cg.GenNode(callee), DeferEdge}}
		}

	case ssa.CallInstruction:
		call := i.Common()
		var results []nodeKind

		// The callee itself
		if callee := call.StaticCallee(); callee != nil {
			results = append(results, nodeKind{cg.GenNode(callee), CallEdge})
		} else if fnVal, ok := isFuncValue(call.Value); ok {
			results = append(results, nodeKind{cg.GenNode(fnVal), CallEdge})
		}

		// Arguments — catches callOthers(triangle) / callOthers(area)
		for _, arg := range call.Args {
			if fnVal, ok := isFuncValue(arg); ok {
				results = append(results, nodeKind{cg.GenNode(fnVal), AssignEdge})
			}
		}

		return results
	case *ssa.Return:
		var results []nodeKind
		for _, val := range i.Results {
			if fnVal, ok := isFuncValue(val); ok {
				results = append(results, nodeKind{cg.GenNode(fnVal), AssignEdge})
			}
		}
		return results
	case *ssa.Store:
		if fnVal, ok := isFuncValue(i.Val); ok {
			return []nodeKind{{cg.GenNode(fnVal), AssignEdge}}
		}

	case *ssa.Panic:
		return []nodeKind{{cg.Root, PanicEdge}}

	case *ssa.Send:
		return []nodeKind{{cg.Root, SendEdge}}
	}

	return nil
}


func BuildExtendedCallGraph2(prog *ssa.Program) *Graph {
	cg := InitGraph(nil)
	seen := map[*ssa.Function]bool{}
	seenEdges := map[edgeKey]bool{}
	
	var visit func(fn *ssa.Function)
	visit = func(fn *ssa.Function) {
		if fn == nil || seen[fn] {
			return
		}
		seen[fn] = true

		callerNode := cg.GenNode(fn)

		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				for _, e := range extractEdges(cg, instr) {
					key := edgeKey{from: callerNode, to: e.node, kind: e.kind}
					if !seenEdges[key] {
						GenEdge(callerNode, instr, e.node, e.kind)
						seenEdges[key] = true
					}

					if e.node.Func != nil && e.node.Func != fn {
						visit(e.node.Func)
					}
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
