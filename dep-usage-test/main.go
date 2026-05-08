package main

import "example.com/depusagetest/something"

/* ============================================================================
 * Function Registry (Leaves)
 * ----------------------------------------------------------------------------
 * These remain the targets of our edges.
 * ============================================================================
 */

func cleanup()                                    { _ = 0 }
func add(a, b int) int                            { return a + b }
func applyInt(f func(int, int) int, a, b int) int { return f(a, b) }
func square(x float32) float32                    { return x * x }
func triangle(b, h float32) float32               { return 0.5 * b * h }
func rect(w, h float32) float32                   { return w * h }
func scale(x, _ float32) float32                  { return x * 2 }
func halve(x, _ float32) float32                  { return x / 2 }
func cubed(x int) int                             { return x * x * x }
func double(x int) int                            { return x * 2 }

var storedFn func(float32, float32) float32
type Pinger interface{ Ping() }

/* ============================================================================
 * Orchestration Layer (Depth 1)
 * ----------------------------------------------------------------------------
 * main calls these. They then call the target functions.
 * ============================================================================
 */

func runConcurrencyTests(jobs chan int, results chan int) {
    /* -------------------------------------------------------
    * *ssa.Go: GoEdge -> worker
    * ------------------------------------------------------- */
    go worker(1, jobs, results)

    /* -------------------------------------------------------
    * Anonymous Functions & Static Calls -> cubed
    * ------------------------------------------------------- */
    go func() {
        defer close(jobs)
        jobs <- cubed(2)
    }()
}

func runDispatchTests() {
    /* -------------------------------------------------------
    * ssa.CallInstruction: concrete vs interface dispatch
    * ------------------------------------------------------- */
    bob := &something.Bob{}
    bob.Ping() // Concrete

    var p Pinger = bob
    p.Ping() // Interface
}

func runFunctionalTests(fnChan chan func(float32, float32) float32) {
    /* -------------------------------------------------------
    * Higher-order logic: Argument passing & Map updates
    * ------------------------------------------------------- */
    _ = applyInt(add, 2, 3)

    fnMap := map[string]func(float32, float32) float32{"rect": rect}
    _ = fnMap

    /* -------------------------------------------------------
    * Channels and Globals: Send & Store
    * ------------------------------------------------------- */
    fnChan <- halve
    storedFn = scale
}

func runEdgeCaseTests() {
    /* -------------------------------------------------------
    * Type Assertion & Panic
    * ------------------------------------------------------- */
    var anyFn interface{} = double
    if f, ok := anyFn.(func(int) int); ok {
        _ = f
    }

    _ = maybePanic(1)
}

/* ============================================================================
 * Support Functions
 * ============================================================================
 */

func worker(_ int, jobs <-chan int, results chan<- int) {
    for j := range jobs { results <- j * j }
}

func maybePanic(x int) int {
    if x < 0 { panic("negative value") }
    return x
}

/* ============================================================================
 * Main (Entry Point)
 * ============================================================================
 */

func main() {
    // ── *ssa.Defer ───────────────────────────────────────────────
    defer cleanup()

    // ── Setup Channels ──────────────────────────────────────────
    jobs := make(chan int)
    results := make(chan int)
    fnChan := make(chan func(float32, float32) float32, 1)

    // ── Execute Depth 1 Functions ────────────────────────────────
    runConcurrencyTests(jobs, results)
    runDispatchTests()
    runFunctionalTests(fnChan)
    runEdgeCaseTests()

    // ── Generics Test (Directly in main for variety) ─────────────
    gBob := &something.GenericBob[int]{Data: 42}
    gBob.Process(10)

    // Drain results to prevent hang
    select {
    case <-results:
    default:
    }
}