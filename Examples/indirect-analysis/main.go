package main

import "fmt"

/* ============================================================================
 * Types used across test cases
 * ----------------------------------------------------------------------------
 * mathFunc     — a named function type (tests that Underlying() stripping
 *                works; a store of mathFunc should still count as a func store)
 * HandlerStore — a struct with a function field (tests *ssa.FieldAddr path
 *                inside *ssa.Store)
 * Registry     — a struct holding a map of funcs (tests MapUpdate into a
 *                struct-owned map)
 * ============================================================================
 */
type mathFunc func(int, int) int

type HandlerStore struct {
    fn       mathFunc
    fallback func(int) int
}

type Registry struct {
    handlers map[string]func(int) int
}

/* ============================================================================
 * Package-level variables for Store tests
 * ----------------------------------------------------------------------------
 * SSA reliably emits *ssa.Store for package-level vars. Local variable
 * assignments are often optimised away — either as dead stores (overwritten
 * before read) or inlined directly to the use site.
 *
 *   storedClosure  — case 1: FuncLiteralStores (capturing closure)
 *   storedClosure2 — case 2: FuncLiteralStores (different signature)
 *   storedBinary   — case 3: FuncInitStores    (named func)
 *   typedFn        — case 4: FuncInitStores    (named type → ChangeType)
 *   anotherBinary  — case 5: FuncPropagations
 *   yetAnother     — case 6: FuncPropagations  (chained)
 * ============================================================================
 */
var storedClosure  func(int, int) int
var storedClosure2 func(int) int
var storedBinary   func(int, int) int
var typedFn        mathFunc
var anotherBinary  func(int, int) int
var yetAnother     func(int, int) int

/* ============================================================================
 * Named functions used as values
 * ----------------------------------------------------------------------------
 * Referenced by name (not called) in the test cases below. Each reference
 * as a value produces an *ssa.Function node as the Store Val, hitting
 * FuncInitStores and PotentialTargets.
 * ============================================================================
 */
func add(a, b int) int { return a + b }
func sub(a, b int) int { return a - b }
func mul(a, b int) int { return a * b }
func double(x int) int { return x * 2 }
func negate(x int) int { return -x }

/* ============================================================================
 * applyBinary / applyUnary — callback pattern
 * ----------------------------------------------------------------------------
 * Effect inside their bodies:
 *   f(a, b) / f(x) — call through a parameter → FuncVarCallSites++
 * ============================================================================
 */
func applyBinary(f func(int, int) int, a, b int) int { return f(a, b) }
func applyUnary(f func(int) int, x int) int           { return f(x) }

/* ============================================================================
 * returnFunc — exercises *ssa.Return with a func-typed result
 * ----------------------------------------------------------------------------
 * Effect inside its body:
 *   each return path → PotentialTargets++ for func(int,int)int  (x2)
 * ============================================================================
 */
func returnFunc(useAdd bool) func(int, int) int {
    if useAdd {
        return add
    }
    return sub
}

/* ============================================================================
 * pickFromMap — forces a genuine FuncVarCallSites hit
 * ----------------------------------------------------------------------------
 * SSA cannot determine which value comes out of a map lookup, so calling
 * the result cannot be optimised into a static call — unlike assigning a
 * named func to a local variable, which SSA sees through directly.
 * ============================================================================
 */
func pickFromMap(
    m   map[string]func(int, int) int,
    key string,
) func(int, int) int {
    return m[key]
}

/* ============================================================================
 * Doer — interface for the interface dispatch test (InterfaceCallSites)
 * ============================================================================
 */
type Doer interface{ Do(int) int }

type AddDoer struct{}
type NegDoer struct{}

func (a AddDoer) Do(x int) int { return x + 1 }
func (n NegDoer) Do(x int) int { return -x }

/* ============================================================================
 * dispatcher — exercises FuncsReceivedForCall
 * ----------------------------------------------------------------------------
 * Uses an explicit two-result receive loop (f, ok := <-jobs) rather than
 * `range` or `select`. The `range` sugar hides the channel receive inside a
 * compiler-generated Next call, making the *ssa.UnOp invisible to our
 * analysis. The explicit form emits:
 *
 *   t0 = <-jobs,ok       (*ssa.UnOp, Op=ARROW, CommaOk=true)
 *   t1 = t0[0]           (*ssa.Extract, Index=0) — the function value
 *   t2 = t0[1]           (*ssa.Extract, Index=1) — the ok bool
 *   t3 = t1(input)       (ssa.CallInstruction, Value=t1)
 *
 * Our Extract path walks t1's referrers and finds t3 where Value==t1,
 * satisfying FuncsReceivedForCall.
 *
 * Effect on report (requires depth >= 1):
 *   *ssa.UnOp ARROW → Extract[0] → called directly → FuncsReceivedForCall++
 *   t1(input)                                       → FuncVarCallSites++
 * ============================================================================
 */
func dispatcher(
    jobs  <-chan func(int) int,
    input int,
    out   chan<- int,
) {
    for {
        f, ok := <-jobs // explicit two-result receive → *ssa.UnOp visible
        if !ok {
            return
        }
        out <- f(input) // Extract[0] referrer is this call → FuncsReceivedForCall++
    }
}

func main() {

    /* -----------------------------------------------------------------------
     * This should have 6 increases
     *  - Function Literal Store           (storedClosure = func ..... )
     *  - Function variable call           (storedClosure(1, 2))
     *  - Function Static Call             (fmt.Println( ... ) )
     * 
     * The same exact thing for the second set
     * However 2 different Function signitures should be recorded
     *  - "func(int) int" & "func(int, int) int"
     * ----------------------------------------------------------------------- */
    offset := 10
    storedClosure = func(a, b int) int {
        return a + b + offset
    }
    fmt.Println(storedClosure(1, 2))

    scale := 3
    storedClosure2 = func(x int) int {
        return x * scale
    }
    fmt.Println(storedClosure2(5))

    /* -----------------------------------------------------------------------
     *  - Function Named Store
     *  - Function variable call           (storedClosure(1, 2))
     *  - Function Static Call             (fmt.Println( ... ) )
     * ----------------------------------------------------------------------- */
    storedBinary = add
    fmt.Println(storedBinary(2, 3))

    /* -----------------------------------------------------------------------
     *  - Function Named Store            (ChangeType(*ssa.Function(&mul)) -> unwrap)
     *  - Function variable call          (storedClosure(1, 2))
     *  - Function Static Call            (fmt.Println( ... ) )
     * ----------------------------------------------------------------------- */
    typedFn = mul
    fmt.Println(typedFn(2, 3))
    
    /* -----------------------------------------------------------------------
     *  - Function Named Store            (in a struct and changeType)
     *  - Function variable call          (storedClosure(1, 2))
     *  - Function Static Call            (fmt.Println( ... ) )
     * ----------------------------------------------------------------------- */
    hs := HandlerStore{}
    hs.fn = mul
    fmt.Println(hs.fn(2, 3))

    /* -----------------------------------------------------------------------
     *  - Function Propogation Store      (Moving the stored pointer)
     *  - Function variable call          (storedClosure(1, 2))
     *  - Function Static Call            (fmt.Println( ... ) )
     * Times 2
     * ----------------------------------------------------------------------- */
    anotherBinary = storedBinary
    fmt.Println(anotherBinary(1, 1))

    yetAnother = anotherBinary
    fmt.Println(yetAnother(1, 1))


    /* -----------------------------------------------------------------------
     * Function literal store inside a struct
     * ----------------------------------------------------------------------- */
    extra := 100
    hs.fallback = func(x int) int {
        return x + extra
    }
    fmt.Println(hs.fallback(1))

    /* -----------------------------------------------------------------------
     * 2 map update functions
     * ----------------------------------------------------------------------- */
    fnMap := map[string]func(int, int) int{
        "add": add, // *ssa.MapUpdate → FuncInStructOrMap++
        "sub": sub, // *ssa.MapUpdate → FuncInStructOrMap++
    }

    /* -----------------------------------------------------------------------
     *  - Static Function Call            (pickFromMap)
     *  - Variable Function Call
     *  - Function Static Call            (fmt.Println( ... ) )
     * ----------------------------------------------------------------------- */
    picked := pickFromMap(fnMap, "add")
    fmt.Println(picked(3, 4))

    /* -----------------------------------------------------------------------
     * Only count chanels created of type function
     * ----------------------------------------------------------------------- */
    funcJobs := make(chan func(int) int, 3) // FuncChans++
    intJobs  := make(chan int, 3)           // skip: elem is int
    out      := make(chan int, 3)
    _ = intJobs

    /* -----------------------------------------------------------------------
     * Should detect function withen the recieved channel (FuncsReceivedForCall++)
     * and a FuncVarCallSites++
     * ----------------------------------------------------------------------- */
    go dispatcher(funcJobs, 10, out)

    /* -----------------------------------------------------------------------
     * Should detect function sent into channel 3 and 3 potential
     * ----------------------------------------------------------------------- */
    funcJobs <- double
    funcJobs <- negate
    extra2   := 1
    funcJobs <- func(x int) int { return x + extra2 }
    close(funcJobs)

    fmt.Println(<-out, <-out, <-out)

    /* -----------------------------------------------------------------------
     * This should basically be a function variable cal site + static
     * also should increase the call site for each signiture by 1
     * ----------------------------------------------------------------------- */
    fmt.Println(applyBinary(add, 2, 3))
    fmt.Println(applyUnary(double, 5))

    /* -----------------------------------------------------------------------
     * testing interfaces
     * ----------------------------------------------------------------------- */
    var d Doer
    d = AddDoer{}
    fmt.Println(d.Do(5))
    d = NegDoer{}
    fmt.Println(d.Do(5))

    /* -----------------------------------------------------------------------
     * Return test
     * ----------------------------------------------------------------------- */
    fn := returnFunc(true)
    fmt.Println(fn(1, 2))
}