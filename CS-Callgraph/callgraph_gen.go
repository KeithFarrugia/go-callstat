package cs_callgraph

import (
	"sync"

	"golang.org/x/tools/go/ssa"
)

/* ============================================================================
 * EffectivePkg
 * ----------------------------------------------------------------------------
 * Returns the SSA package that logically owns fn. SSA does not always populate
 * fn.Pkg directly — generic instantiations, synthetic wrappers, and anonymous
 * functions all leave it nil, with ownership residing on their origin, parent,
 * or enclosing function instead.
 *
 * Resolution order:
 *   1. fn.Pkg          — direct, fast path
 *   2. fn.Origin()     — generic instantiation → template function
 *   3. fn.Parent()     — closure / anonymous function → enclosing function
 *
 * Results are memoized in EffectivePkgCache to avoid redundant traversals
 * across repeated lookups of the same function. Returns nil if no package
 * can be structurally determined.
 * ============================================================================
 */
var EffectivePkgCache sync.Map // map[*ssa.Function]*ssa.Package

func EffectivePkg(fn *ssa.Function) *ssa.Package {
    if fn == nil {
        return nil
    }
    if cached, ok := EffectivePkgCache.Load(fn); ok {
        return cached.(*ssa.Package)
    }

    var result *ssa.Package

    if fn.Pkg != nil {
        result = fn.Pkg
    } else if origin := fn.Origin(); origin != nil && origin != fn {
        result = EffectivePkg(origin)
    } else if parent := fn.Parent(); parent != nil {
        result = EffectivePkg(parent)
    }

    EffectivePkgCache.Store(fn, result)
    return result
}
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
    if fn.Pkg != nil {
        return fn
    }
    if origin := fn.Origin(); origin != nil && origin != fn {
        return resolveServiceableFunc(origin)
    }
    if parent := fn.Parent(); parent != nil {
        return resolveServiceableFunc(parent)
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

    case *ssa.MapUpdate:
        if fnVal, ok := isFuncValue(i.Value); ok {
            return []nodeKind{{cg.GenNode(fnVal), AssignEdge}}
        }
        
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
    existingEdges := map[edgeKey]*Edge{}

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
                    
                    if edge, exists := existingEdges[key]; exists {
                        // We've seen this relationship before; just add the new call site
                        edge.Sites = append(edge.Sites, instr)
                    } else {
                        // New relationship; create the edge and track it
                        newEdge := GenEdge(callerNode, instr, e.node, e.kind)
                        existingEdges[key] = newEdge
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