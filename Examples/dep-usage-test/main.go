package main

import secondary "example.com/depusagetest/secondary"

/* ============================================================================
 * DoerPlus interface
 * ----------------------------------------------------------------------------
 * Extends secondary.Doer with an additional method.
 *   DoAndDouble → used via interface dispatch below
 * ============================================================================
 */
type DoerPlus interface {
    secondary.Doer
    DoAndDouble(x int) int
}

/* ============================================================================
 * Concrete implementation of DoerPlus
 * ============================================================================
 */
type BigDoer struct{ secondary.AddDoer }

func (b BigDoer) DoAndDouble(x int) int { return b.Do(x) * 2 }

/* ============================================================================
 * Function registry
 * ----------------------------------------------------------------------------
 * Each function below is the primary subject of exactly one case in
 * extractEdges. Internal calls within a function's own body are not
 * counted as "uses" for this mapping.
 *
 *   cleanup    → *ssa.Defer       deferred named function
 *   worker     → *ssa.Go          goroutine static callee
 *   square     → CallInstruction  direct static call
 *   add        → CallInstruction  passed as callback argument
 *   test.Ping   → CallInstruction  method on concrete type
 *   p.Ping     → CallInstruction  interface dispatch
 *   triangle   → *ssa.Return      returned as a function value
 *   rect       → *ssa.MapUpdate   stored as a map value
 *   scale      → *ssa.Store       written to package-level var
 *   maybePanic → *ssa.Panic       body contains the panic instruction
 *   halve      → *ssa.Send        sent over a function-typed channel
 *   double     → *ssa.TypeAssert  held in interface{}, asserted back
 * ============================================================================
 */

func cleanup()                                         	{ _ = 0 }
func add(a, b int) int                                 	{ return a + b }
func applyInt(f func(int, int) int, a, b int) int      	{ return f(a, b) }
func square(x float32) float32                         	{ return x * x }
func triangle(b, h float32) float32                   	{ return 0.5 * b * h }
func rect(w, h float32) float32                        	{ return w * h }
func scale(x, _ float32) float32                       	{ return x * 2 }
func halve(x, _ float32) float32                       	{ return x / 2 }
func cubed(x int)	int							        { return x*x*x }
func double(x int) int                                 	{ return x * 2 }

func worker(_ int, jobs <-chan int, results chan<- int) {
    for j := range jobs {
        results <- j * j
    }
}

func callOthers() func(float32, float32) float32 {
    return triangle
}

func maybePanic(x int) int {
    if x < 0 {
        panic("negative value")
    }
    return x
}

var storedFn func(float32, float32) float32

type Pinger interface{ Ping() }

func main() {
    jobs   := make(chan int)
    results := make(chan int)
    fnChan := make(chan func(float32, float32) float32, 1)

    /* -------------------------------------------------------
    * *ssa.Defer: DeferEdge -> cleanup
    * ------------------------------------------------------- */
    defer cleanup()

    /* -------------------------------------------------------
    * *ssa.Go: GoEdge -> worker
    * ------------------------------------------------------- */
    go worker(1, jobs, results)
    go worker(2, jobs, results)
	
    /* -------------------------------------------------------
    * Anonymous Functions with Static call -> cubed
    * ------------------------------------------------------- */
    go func() {
        defer close(jobs)
        jobs <- cubed(2)
        jobs <- cubed(3)
    }()
    _, _ = <-results, <-results

    /* -------------------------------------------------------
    * ssa.CallInstruction: static call -> square
    * ------------------------------------------------------- */
    _ = square(4.0)

    /* -------------------------------------------------------
    * ssa.CallInstruction: method on concrete type -> bob.Ping
    * ------------------------------------------------------- */
    test := &secondary.Test{}
    test.Ping()

    /* -------------------------------------------------------
    * ssa.CallInstruction: functional argument -> add
    * ------------------------------------------------------- */
    _ = applyInt(add, 2, 3)

    /* -------------------------------------------------------
    * *ssa.Return: AssignEdge -> triangle
    * ------------------------------------------------------- */
    fn := callOthers()
    _ = fn

    /* -------------------------------------------------------
    * *ssa.MapUpdate: AssignEdge -> rect
    * ------------------------------------------------------- */
    fnMap := map[string]func(float32, float32) float32{}
    fnMap["rect"] = rect
    _ = fnMap

    /* -------------------------------------------------------
    * *ssa.Store: AssignEdge -> scale
    * ------------------------------------------------------- */
    storedFn = scale
    _ = storedFn

    /* -------------------------------------------------------
    * *ssa.Panic: PanicEdge (emitted from maybePanic body)
    * ------------------------------------------------------- */
    _ = maybePanic(1)

    /* -------------------------------------------------------
    * *ssa.Send: SendEdge -> halve
    * ------------------------------------------------------- */
    fnChan <- halve
    _ = <-fnChan

    /* -------------------------------------------------------
    * *ssa.TypeAssert: AssignEdge -> double
    * ------------------------------------------------------- */
    var anyFn interface{} = double
    if f, ok := anyFn.(func(int) int); ok {
        _ = f
    }

	/* -------------------------------------------------------
    * Generics / Templates: GenericBob[int].Process
    * Tests: resolveServiceableFunc unwrapping instantiations
    * ------------------------------------------------------- */
    gBob := &secondary.GenericTest[int]{Data: 42}
    gBob.Process(10)


    /* -------------------------------------------------------
     * ssa.CallInstruction: interface dispatch -> secondary.Doer.Do
     * Tests: IsInvoke with interface defined in external package
     * ------------------------------------------------------- */
    var d secondary.Doer = secondary.AddDoer{}
    _ = d.Do(5)
    d = secondary.NegDoer{}
    _ = d.Do(5)

    /* -------------------------------------------------------
     * ssa.CallInstruction: interface dispatch -> DoerPlus.DoAndDouble
     * Tests: IsInvoke with extended interface defined in main package
     * ------------------------------------------------------- */
    var dp DoerPlus = BigDoer{}
    _ = dp.DoAndDouble(3)

    /* -------------------------------------------------------
     * secondary.Doer.Report is intentionally never called
     * Tests: unused interface method detection
     * ------------------------------------------------------- */
}