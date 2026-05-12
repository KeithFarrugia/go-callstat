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
 * Tracks how function values move through a program — both where they are
 * called (call sites) and where they are treated as data (assignments,
 * propagation, channel passing, struct fields, etc.).
 *
 * Call site counters:
 *   StaticCallSites      — callee known at compile time
 *   InterfaceCallSites   — dispatch through an interface method
 *   FuncVarCallSites     — call through a function-valued variable
 *
 * Assignment / propagation counters:
 *   FuncLiteralStores    — `f := func() { ... }`  (closure/literal created)
 *   FuncNamedStores	  — `f := someNamedFunc`    (named func stored)
 *   FuncPropagations     — `b = a` where a is already a func-typed variable
 *   FuncInStructOrMap    — func stored into a struct field or map entry
 *
 * Channel / goroutine counters:
 *   FuncChans            — make(chan F) where F is a function type
 *   GoroutinesFuncChan   — goroutines whose channel param holds func pointers
 *   FuncsSentToFuncChan  — Send of a func value into a func-typed channel
 *   FuncsReceivedForCall — Receive from a func chan used directly as a callee
 *
 * SignatureMetrics — per-signature cross-product data:
 *   PotentialTargets — times a func of this signature is treated as a value
 *   ActualCallSites  — times an indirect call uses this signature
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
) *IndirectAnalysisReport {
	fmt.Printf("Indirect ANALYSIS")
	report  := newIndirectReport()
	inDepth := makeDepthGate(depthMap, maxDepth)

	mainNode := resolveMainNode(g, depthMap, projectRoot, inDepth)
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
		debugAnalyzeInstructions(n.Func, r)
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
 * Walks every SSA instruction in fn and classifies it into one of the
 * report buckets. See inline comments for the SSA patterns each case covers.
 * ============================================================================
 */


/* ============================================================================
 * debugAnalyzeInstructions
 * ----------------------------------------------------------------------------
 * Drop-in replacement for analyzeInstructions that prints a table of every
 * instruction it processes and which bucket(s) it hit.
 *
 * Output columns:
 *   FUNC     — the SSA function being analysed
 *   INSTR    — the SSA instruction type and its source representation
 *   BUCKET   — which counter(s) were incremented, or WHY it was skipped
 * ============================================================================
 */
func debugAnalyzeInstructions(fn *ssa.Function, r *IndirectAnalysisReport) {
	fmt.Printf("\n%s\n%s\n",
		fn.Name(),
		strings.Repeat("─", 80),
	)
	fmt.Printf("  %-20s %-38s %s\n", "TYPE", "INSTRUCTION", "BUCKET")
	fmt.Printf("  %-20s %-38s %s\n",
		strings.Repeat("-", 20),
		strings.Repeat("-", 38),
		strings.Repeat("-", 18),
	)

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			instrType := fmt.Sprintf("%T", instr)
			instrType  = strings.TrimPrefix(instrType, "*ssa.")
			instrType  = strings.TrimPrefix(instrType, "ssa.")
			instrStr  := fmt.Sprintf("%.38s", instr)
			bucket    := ""

			switch i := instr.(type) {

			case *ssa.MakeChan:
				ch, ok := i.Type().Underlying().(*types.Chan)
				if !ok {
					bucket = "skip: not a chan type"
					break
				}
				if ok && containsFunc(ch.Elem()) {
					r.FuncChans++
					bucket = "FuncChans++"
				} else {
					bucket = fmt.Sprintf(
						"skip: elem=%s not func", ch.Elem(),
					)
				}

			case *ssa.Go:
				hit := false
				for _, arg := range i.Common().Args {
					ch, ok := arg.Type().Underlying().(*types.Chan)
					if !ok {
						continue
					}
					if ok && containsFunc(ch.Elem()) {
						r.GoroutinesFuncChan++
						bucket += "GoroutinesFuncChan++ "
						hit = true
					}
				}
				if !hit {
					bucket = "skip: no func-chan args"
				}

			case ssa.CallInstruction:
				call := i.Common()
				sig  := call.Signature()

				/* -------------------------------------------------------
				 * Builtins (close, len, cap, make, etc.) have no
				 * StaticCallee and no Method but are not indirect calls.
				 * Count them as static and skip further classification.
				 * ------------------------------------------------------- */
				if _, isBuiltin := call.Value.(*ssa.Builtin); isBuiltin {
					r.StaticCallSites++
					bucket = fmt.Sprintf(
						"StaticCallSites++ (builtin: %s)",
						call.Value.(*ssa.Builtin).Name(),
					)
					break
				}

				if call.StaticCallee() != nil {
					r.StaticCallSites++
					bucket = fmt.Sprintf(
						"StaticCallSites++ (%s)",
						call.StaticCallee().Name(),
					)
				} else if call.Method != nil {
					r.InterfaceCallSites++
					r.getSig(sig).ActualCallSites++
					bucket = fmt.Sprintf(
						"InterfaceCallSites++ sig=%s",
						sigKey(sig),
					)
				} else {
					r.FuncVarCallSites++
					r.getSig(sig).ActualCallSites++
					bucket = fmt.Sprintf(
						"FuncVarCallSites++ sig=%s",
						sigKey(sig),
					)
				}

			case *ssa.Store:
				val := i.Val
				if _, ok := funcSig(val.Type()); !ok {
					break // not a func type — skip silently (too noisy)
				}

				/* -------------------------------------------------------
				 * Unwrap ChangeType before classifying the source.
				 * ChangeType appears when a named func type is stored into
				 * a variable of the underlying type, e.g:
				 *   type mathFunc func(int,int)int
				 *   var f func(int,int)int = mathFunc(mul)
				 * Without unwrapping, mul would fall into FuncPropagations
				 * instead of FuncNamedStores.
				 * ------------------------------------------------------- */
				unwrapped := val
				if ct, ok := val.(*ssa.ChangeType); ok {
					unwrapped = ct.X
					bucket = fmt.Sprintf(
						"(unwrapped ChangeType → %T) ", unwrapped,
					)
				}

				switch v := unwrapped.(type) {
				case *ssa.Function:
					r.FuncNamedStores++
					r.countPotential(val)
					bucket += fmt.Sprintf(
						"FuncNamedStores++ (named: %s)", v.Name(),
					)
				case *ssa.MakeClosure:
					r.FuncLiteralStores++
					r.countPotential(val)
					bucket += "FuncLiteralStores++"
				default:
					r.FuncPropagations++
					r.countPotential(val)
					bucket += fmt.Sprintf(
						"FuncPropagations++ (unwrapped type: %T)", unwrapped,
					)
				}

				if _, ok := i.Addr.(*ssa.FieldAddr); ok {
					r.FuncInStructOrMap++
					bucket += " + FuncInStructOrMap++ (FieldAddr)"
				}

			case *ssa.MapUpdate:
				if containsFunc(i.Value.Type()) {
					r.FuncInStructOrMap++
					r.countPotential(i.Value)
					bucket = fmt.Sprintf(
						"FuncInStructOrMap++ (MapUpdate val type: %T)",
						i.Value,
					)
				} else {
					bucket = "skip: map value not func"
				}

			case *ssa.Send:
				ch, ok := i.Chan.Type().Underlying().(*types.Chan)
				if !ok {
					bucket = "skip: Send chan not *types.Chan"
					break
				}
				if ok && containsFunc(ch.Elem()) {
					r.FuncsSentToFuncChan++
					r.countPotential(i.X)
					bucket = fmt.Sprintf(
						"FuncsSentToFuncChan++ (val type: %T)", i.X,
					)
				} else {
					bucket = fmt.Sprintf(
						"skip: chan elem=%s not func", ch.Elem(),
					)
				}

			case *ssa.UnOp:
				if i.Op != token.ARROW { break }
				ch, ok := i.X.Type().Underlying().(*types.Chan)
				if !ok || !containsFunc(ch.Elem()) { break }

				bucket = "receive from func-container — tracing flow..."
				
				// We define a helper to recursively check if an SSA value 
				// eventually leads to a call site.
				var leadsToCall func(v ssa.Value, depth int) bool
				leadsToCall = func(v ssa.Value, depth int) bool {
					if depth > 5 { return false } // Prevent infinite loops in weird SSA edges
					
					refs := v.Referrers()
					if refs == nil { return false }

					for _, ref := range *refs {
						switch rnt := ref.(type) {
						case ssa.CallInstruction:
							// Direct call: v() or v.Method()
							if rnt.Common().Value == v {
								return true
							}
						case *ssa.Extract:
							// Handle the v, ok := <-ch case
							if leadsToCall(rnt, depth+1) {
								return true
							}
						case *ssa.Field, *ssa.FieldAddr:
							// This is what you were missing! f.fallback
							if leadsToCall(rnt.(ssa.Value), depth+1) {
								return true
							}
						case *ssa.UnOp:
							// Handle pointer dereferences if the struct is passed as *HandlerStore
							if rnt.Op == token.MUL {
								if leadsToCall(rnt, depth+1) {
									return true
								}
							}
						}
					}
					return false
				}

				if leadsToCall(i, 0) {
					r.FuncsReceivedForCall++
					r.countPotential(i) // Ensure this uses your updated containsFunc logic
					bucket = "FuncsReceivedForCall++ (traced to call site)"
				} else {
					bucket = "skip: received but not traced to a direct call"
				}

			case *ssa.Return:
				hits := 0
				for _, res := range i.Results {
					if containsFunc(res.Type()) {
						r.countPotential(res)
						hits++
					}
				}
				if hits > 0 {
					bucket = fmt.Sprintf(
						"Return: countPotential x%d", hits,
					)
				}
			}

			if bucket != "" {
				fmt.Printf("  %-20s %-38s %s\n",
					instrType, instrStr, bucket,
				)
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
	switch T := t.Underlying().(type) {
	case *types.Signature:
		return true
	case *types.Struct:
		for i := 0; i < T.NumFields(); i++ {
			if containsFunc(T.Field(i).Type()) {
				return true
			}
		}
	case *types.Array:
		return containsFunc(T.Elem())
	case *types.Slice:
		return containsFunc(T.Elem())
	case *types.Map:
		return containsFunc(T.Key()) || containsFunc(T.Elem())
	case *types.Chan:
		return containsFunc(T.Elem())
	case *types.Pointer:
		return containsFunc(T.Elem())
	}
	return false
}