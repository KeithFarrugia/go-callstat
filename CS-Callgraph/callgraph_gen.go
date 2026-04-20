package cs_callgraph

import (
	"golang.org/x/tools/go/ssa"
)

/* ============================================================================
 * resolveServiceableFunc
 * ----------------------------------------------------------------------------
 * Resolves a function to its logical origin. If a function is a generic
 * instantiation or a synthetic wrapper, this returns the original template
 * or method where the package info resides.
 * ============================================================================
 */
func resolveServiceableFunc(fn *ssa.Function) *ssa.Function {
    if fn == nil {
        return nil
    }
    // Origin() returns the generic template for specialized instances
    // or the original method for synthetic wrappers.
    if fn.Pkg == nil && fn.Origin() != nil {
        return fn.Origin()
    }
    return fn
}
/* ============================================================================
 * isFuncValue
 * ----------------------------------------------------------------------------
 * Determines if an ssa.Value is a function, closure, or a type-cast function.
 * Automatically resolves specialized generics or synthetic wrappers to 
 * their logical origins.
 * ============================================================================
 */
func isFuncValue(v ssa.Value) (*ssa.Function, bool) {
    switch v := v.(type) {
    case *ssa.Function:
        return resolveServiceableFunc(v), true

    case *ssa.MakeClosure:
        if fn, ok := v.Fn.(*ssa.Function); ok {
            return resolveServiceableFunc(fn), true
        }

    case *ssa.ChangeType:
        return isFuncValue(v.X)
    }
    return nil, false
}

/* ============================================================================
 * extractEdges
 * ----------------------------------------------------------------------------
 * Scans an instruction for all possible edges (calls, goroutines, returns).
 * ============================================================================
 */
func extractEdges(cg *Graph, instr ssa.Instruction) []nodeKind {
    switch i := instr.(type) {

    case *ssa.Go:
        call := i.Common()
        if callee := call.StaticCallee(); callee != nil {
            return []nodeKind{{cg.GenNode(resolveServiceableFunc(callee)), GoEdge}}
        }

    case *ssa.Defer:
        call := i.Common()
        if callee := call.StaticCallee(); callee != nil {
            return []nodeKind{{cg.GenNode(resolveServiceableFunc(callee)), DeferEdge}}
        }

    case ssa.CallInstruction:
        call := i.Common()
        var results []nodeKind

        // The primary callee
        if callee := call.StaticCallee(); callee != nil {
            target := resolveServiceableFunc(callee)
            results = append(results, nodeKind{cg.GenNode(target), CallEdge})
        } else if fnVal, ok := isFuncValue(call.Value); ok {
            results = append(results, nodeKind{cg.GenNode(fnVal), CallEdge})
        }

        // Functional arguments (e.g., passing a callback)
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
        return []nodeKind{{cg.PanicNode, PanicEdge}}

    case *ssa.Send:
        if fnVal, ok := isFuncValue(i.X); ok {
            return []nodeKind{{cg.GenNode(fnVal), SendEdge}}
        }
    }
    return nil
}
/* ============================================================================
 * BuildExtendedCallGraph2
 * ----------------------------------------------------------------------------
 * Entry point for building the graph by visiting all reachable functions.
 * ============================================================================
 */
func BuildExtendedCallGraph2(prog *ssa.Program) *Graph {
    cg        := InitGraph(nil)
    seen      := map[*ssa.Function]bool{}
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

    // Start traversal from all package-level members
    for _, pkg := range prog.AllPackages() {
        for _, mem := range pkg.Members {
            if fn, ok := mem.(*ssa.Function); ok {
                visit(fn)
            }
        }
    }

    return cg
}