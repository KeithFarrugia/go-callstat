package stats

import (
	cs_callgraph "callstat/CS-Callgraph"
	"fmt"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

/* ============================================================================
 * IndirectAnalysisReport & SigMetric
 * ----------------------------------------------------------------------------
 * Tracks how function values move through a program - both where they are
 * called (call sites) and where they are treated as data (assignments,
 * propagation, channel passing, struct fields, etc.).
 *
 * Call site counters:
 *   StaticCallSites      - callee known at compile time
 *   InterfaceCallSites   - dispatch through an interface method
 *   FuncVarCallSites     - call through a function-valued variable
 *
 * Assignment / propagation counters:
 *   FuncLiteralStores    - `f := func() { ... }`  (closure/literal created)
 *   FuncNamedStores	  - `f := someNamedFunc`    (named func stored)
 *   FuncPropagations     - `b = a` where a is already a func-typed variable
 *   FuncInStructOrMap    - func stored into a struct field or map entry
 *
 * Channel / goroutine counters:
 *   FuncChans            - make(chan F) where F is a function type
 *   GoroutinesFuncChan   - goroutines whose channel param holds func pointers
 *   FuncsSentToFuncChan  - Send of a func value into a func-typed channel
 *   FuncsReceivedForCall - Receive from a func chan used directly as a callee
 *
 * SignatureMetrics - per-signature cross-product data:
 *   PotentialTargets - times a func of this signature is treated as a value
 *   ActualCallSites  - times an indirect call uses this signature
 * ============================================================================
 */
type IndirectAnalysisReport struct {
	// Call sites
	StaticCallSites    		int `json:"staticCallSites"`
	InterfaceCallSites 		int `json:"interfaceCallSites"`
	FuncVarCallSites   		int `json:"funcVarCallSites"`

	// Assignment and propagation
	FuncLiteralStores 		int `json:"funcLiteralStores"`
	FuncNamedStores			int `json:"funcNamedStores"`
	FuncPropagations  		int `json:"funcPropagations"`
	FuncInStructOrMap		int `json:"funcInStructOrMap"`

	// Channel / goroutine
	FuncChans				int `json:"funcChans"`
	GoroutinesFuncChan		int `json:"goroutinesFuncChan"`
	FuncsSentToFuncChan		int `json:"funcsSentToFuncChan"`
	FuncsReceivedForCall 	int `json:"funcsReceivedForCall"`

	SignatureMetrics map[string]*SigMetric `json:"signatureMetrics"`
}

type SigMetric struct {
	// Times a func of this signature is treated as a value
	PotentialTargets int `json:"potentialTargets"`
	// Times an indirect call uses this signature
	ActualCallSites int `json:"actualCallSites"`
}

func newIndirectReport() *IndirectAnalysisReport {
	return &IndirectAnalysisReport{
		SignatureMetrics: make(map[string]*SigMetric),
	}
}

func (r *IndirectAnalysisReport) getSig(sig *types.Signature) *SigMetric {
	s := sigKey(sig)
	if _, ok := r.SignatureMetrics[s]; !ok {
		r.SignatureMetrics[s] = &SigMetric{}
	}
	return r.SignatureMetrics[s]
}

func (r *IndirectAnalysisReport) countPotential(v ssa.Value) {
	if sig, ok := funcSig(v.Type()); ok {
		r.getSig(sig).PotentialTargets++
	}
}

/* ============================================================================
 * GatherResearchStats
 * ----------------------------------------------------------------------------
 * Entry point. Starts a DFS from main and analyses every in-depth function.
 * ============================================================================
 */
func GatherResearchStats(
	g           *cs_callgraph.Graph,
	depthMap    map[string]int,
	maxDepth    int,
	projectRoot string,
	mainNode 	*cs_callgraph.Node,
	skipPkg  map[string]struct{},
) *IndirectAnalysisReport {
	report  := newIndirectReport()
	inDepth := makeDepthGate(depthMap, maxDepth, skipPkg)

	if mainNode != nil {
		visited := make(map[int]struct{})
		traverseAndAnalyze(mainNode, visited, report, inDepth)
	}

	return report
}

/* ============================================================================
 * traverseAndAnalyze
 * ----------------------------------------------------------------------------
 * DFS over the call graph. Analyses instructions only for nodes whose
 * package passes the depth gate.
 * ============================================================================
 */
func traverseAndAnalyze(
	n       *cs_callgraph.Node,
	visited map[int]struct{},
	r       *IndirectAnalysisReport,
	inDepth func(string) bool,
) {
	if n == nil || n.Func == nil {
		return
	}
	if _, ok := visited[n.ID]; ok {
		return
	}
	visited[n.ID] = struct{}{}

	pkg := cs_callgraph.EffectivePkg(n.Func)
	if pkg != nil && pkg.Pkg != nil && inDepth(pkg.Pkg.Path()) {
		analyzeInstructions(n.Func, r)
	}

	for _, e := range n.Out {
		if e.Callee != nil {
			traverseAndAnalyze(e.Callee, visited, r, inDepth)
		}
	}
}
/* ============================================================================
 * analyzeInstructions
 * ----------------------------------------------------------------------------
 * Drop-in replacement for analyzeInstructions that prints a table of every
 * instruction it processes and which bucket(s) it hit.
 *
 * Output columns:
 *   FUNC     - the SSA function being analysed
 *   INSTR    - the SSA instruction type and its source representation
 *   BUCKET   - which counter(s) were incremented, or WHY it was skipped
 * ============================================================================
 */
func analyzeInstructions(fn *ssa.Function, r *IndirectAnalysisReport) {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {

			switch i := instr.(type) {

			case *ssa.MakeChan:
				ch, ok := i.Type().Underlying().(*types.Chan)
				if ok && containsFunc(ch.Elem()) {
					r.FuncChans++
				}

			case *ssa.Go:
				for _, arg := range i.Common().Args {
					ch, ok := arg.Type().Underlying().(*types.Chan)
					if ok && containsFunc(ch.Elem()) {
						r.GoroutinesFuncChan++
					}
				}

			case ssa.CallInstruction:
				call := i.Common()
				sig := call.Signature()

				if _, isBuiltin := call.Value.(*ssa.Builtin); isBuiltin {
					r.StaticCallSites++
					break
				}

				if call.StaticCallee() != nil {
					r.StaticCallSites++
				} else if call.Method != nil {
					r.InterfaceCallSites++
				} else {
					r.FuncVarCallSites++
					r.getSig(sig).ActualCallSites++
				}

			case *ssa.Store:
				val := i.Val
				if _, ok := funcSig(val.Type()); !ok {
					break
				}

				unwrapped := val
				if ct, ok := val.(*ssa.ChangeType); ok {
					unwrapped = ct.X
				}

				switch unwrapped.(type) {
				case *ssa.Function:
					r.FuncNamedStores++
					r.countPotential(val)

				case *ssa.MakeClosure:
					r.FuncLiteralStores++
					r.countPotential(val)

				default:
					r.FuncPropagations++
					r.countPotential(val)
				}

				if _, ok := i.Addr.(*ssa.FieldAddr); ok {
					r.FuncInStructOrMap++
				}

			case *ssa.MapUpdate:
				if containsFunc(i.Value.Type()) {
					r.FuncInStructOrMap++
					r.countPotential(i.Value)
				}
			
			case *ssa.TypeAssert:
				if containsFunc(i.Type()) {
					r.FuncPropagations++
					r.countPotential(i)
				}
			case *ssa.Send:
				ch, ok := i.Chan.Type().Underlying().(*types.Chan)
				if ok && containsFunc(ch.Elem()) {
					r.FuncsSentToFuncChan++
					r.countPotential(i.X)
				}

			case *ssa.UnOp:
				if i.Op != token.ARROW {
					break
				}

				ch, ok := i.X.Type().Underlying().(*types.Chan)
				if !ok || !containsFunc(ch.Elem()) {
					break
				}

				var leadsToCall func(v ssa.Value, depth int) bool
				leadsToCall = func(v ssa.Value, depth int) bool {
					if depth > 8 {
						return false
					}

					refs := v.Referrers()
					if refs == nil {
						return false
					}

					for _, ref := range *refs {
						switch r := ref.(type) {

						case ssa.CallInstruction:
							if r.Common().Value == v {
								return true
							}

						case *ssa.Extract:
							if r.Index == 0 && leadsToCall(r, depth+1) {
								return true
							}

						case *ssa.Field:
							if leadsToCall(r, depth+1) {
								return true
							}

						case *ssa.FieldAddr:
							if leadsToCall(r, depth+1) {
								return true
							}

						case *ssa.UnOp:
							if (r.Op == token.MUL || r.Op == token.ARROW) &&
								leadsToCall(r, depth+1) {
								return true
							}
						}
					}

					return false
				}

				if leadsToCall(i, 0) {
					r.FuncsReceivedForCall++
					r.countPotential(i)
				}

			case *ssa.Return:
				for _, res := range i.Results {
					if containsFunc(res.Type()) {
						r.countPotential(res)
					}
				}
			}
		}
	}
}
/* ============================================================================
 * Type helpers
 * ============================================================================
 */

/* -------------------------------------------------------
 * funcSig
 * Returns the *types.Signature if t's underlying type is
 * a function, otherwise (nil, false).
 * ------------------------------------------------------- */
func funcSig(t types.Type) (*types.Signature, bool) {
	sig, ok := t.Underlying().(*types.Signature)
	return sig, ok
}


func sigKey(sig *types.Signature) string {
    // types.TypeString with an empty qualifier drops package paths;
    // we also need to strip param names by rebuilding the string manually
    params := make([]string, sig.Params().Len())
    for i := range params {
        params[i] = types.TypeString(
            sig.Params().At(i).Type(), nil,
        )
    }
    results := make([]string, sig.Results().Len())
    for i := range results {
        results[i] = types.TypeString(
            sig.Results().At(i).Type(), nil,
        )
    }
    return fmt.Sprintf(
        "func(%s) %s",
        strings.Join(params, ", "),
        strings.Join(results, ", "),
    )
}



/* -------------------------------------------------------
 * containsFunc
 * Recursively checks if a type is a function or contains
 * one (in a struct field, array element, etc.).
 * ------------------------------------------------------- */
func containsFunc(t types.Type) bool {
    seen := make(map[types.Type]bool)
    return containsFuncRec(t, seen)
}

func containsFuncRec(t types.Type, seen map[types.Type]bool) bool {
    if seen[t] {
        return false
    }
    seen[t] = true

    switch T := t.Underlying().(type) {
    case *types.Signature:
        return true
    case *types.Struct:
        for i := 0; i < T.NumFields(); i++ {
            if containsFuncRec(T.Field(i).Type(), seen) {
                return true
            }
        }
    case *types.Array:
        return containsFuncRec(T.Elem(), seen)
    case *types.Slice:
        return containsFuncRec(T.Elem(), seen)
    case *types.Map:
        return containsFuncRec(T.Key(), seen) ||
               containsFuncRec(T.Elem(), seen)
    case *types.Chan:
        return containsFuncRec(T.Elem(), seen)
    case *types.Pointer:
        return containsFuncRec(T.Elem(), seen)
    }
    return false
}