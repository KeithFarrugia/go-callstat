package main

import "fmt"

/* ============================================================================
 * Types used across test cases
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
 * Nested struct types - for struct-in-struct func field tests
 * ----------------------------------------------------------------------------
 * Router      - top-level struct containing a Middleware
 * Middleware   - contains a func field and a nested HandlerStore
 * Pipeline     - slice of func steps (tests func in slice field)
 * EventBus     - map of func slices (tests func in map-of-slices field)
 * ============================================================================
 */
type Middleware struct {
	process  func(int) int // direct func field
	fallback HandlerStore  // nested struct with func fields
}

type Router struct {
	middleware Middleware            // nested struct
	onError    func(error)          // direct func field on outer struct
	routes     map[string]mathFunc  // map of named func type
}

type Pipeline struct {
	steps []func(int) int // slice of funcs as a struct field
}

type EventBus struct {
	listeners map[string][]func(int) // map of func slices
}

/* ============================================================================
 * Package-level variables for Store tests
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
 * ============================================================================
 */
func add(a, b int) int    { return a + b }
func sub(a, b int) int    { return a - b }
func mul(a, b int) int    { return a * b }
func double(x int) int    { return x * 2 }
func negate(x int) int    { return -x }
func square(x int) int    { return x * x }
func increment(x int) int { return x + 1 }

func applyBinary(f func(int, int) int, a, b int) int { return f(a, b) }
func applyUnary(f func(int) int, x int) int           { return f(x) }

func returnFunc(useAdd bool) func(int, int) int {
	if useAdd {
		return add
	}
	return sub
}

func pickFromMap(
	m   map[string]func(int, int) int,
	key string,
) func(int, int) int {
	return m[key]
}

type Doer interface{ Do(int) int }

type AddDoer struct{}
type NegDoer struct{}

func (a AddDoer) Do(x int) int { return x + 1 }
func (n NegDoer) Do(x int) int { return -x }

func dispatcher(
	jobs  <-chan func(int) int,
	input int,
	out   chan<- int,
) {
	for {
		f, ok := <-jobs
		if !ok {
			return
		}
		out <- f(input)
	}
}

func main() {

	/* -----------------------------------------------------------------------
	 * ORIGINAL CASES (unchanged)
	 * ----------------------------------------------------------------------- */

	/* -----------------------------------------------------------------------
	 *  - Function Literal Store           (storedClosure = func ..... )
	 *  - Function variable call           (storedClosure(1, 2))
	 *  - Function Static Call             (fmt.Println( ... ) )
	 *
	 * The same exact thing for the second set.
	 * Two different signatures recorded:
	 *   "func(int, int) int" & "func(int) int"
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
	 *  - Function variable call
	 *  - Function Static Call
	 * ----------------------------------------------------------------------- */
	storedBinary = add
	fmt.Println(storedBinary(2, 3))

	/* -----------------------------------------------------------------------
	 *  - Function Named Store  (ChangeType unwrap: mathFunc → func(int,int)int)
	 *  - Function variable call
	 *  - Function Static Call
	 * ----------------------------------------------------------------------- */
	typedFn = mul
	fmt.Println(typedFn(2, 3))

	/* -----------------------------------------------------------------------
	 *  - Function Named Store in struct field  (FieldAddr + ChangeType)
	 *  - Function variable call
	 *  - Function Static Call
	 * ----------------------------------------------------------------------- */
	hs := HandlerStore{}
	hs.fn = mul
	fmt.Println(hs.fn(2, 3))

	/* -----------------------------------------------------------------------
	 *  - Function Propagation Store x2
	 *  - Function variable call x2
	 *  - Function Static Call x2
	 * ----------------------------------------------------------------------- */
	anotherBinary = storedBinary
	fmt.Println(anotherBinary(1, 1))
	yetAnother = anotherBinary
	fmt.Println(yetAnother(1, 1))

	/* -----------------------------------------------------------------------
	 * Function literal store inside a struct field
	 *  - FuncLiteralStores++
	 *  - FuncInStructOrMap++  (FieldAddr)
	 * ----------------------------------------------------------------------- */
	extra := 100
	hs.fallback = func(x int) int {
		return x + extra
	}
	fmt.Println(hs.fallback(1))

	/* -----------------------------------------------------------------------
	 * 2 map update functions
	 *  - FuncInStructOrMap++ x2  (MapUpdate)
	 * ----------------------------------------------------------------------- */
	fnMap := map[string]func(int, int) int{
		"add": add,
		"sub": sub,
	}

	/* -----------------------------------------------------------------------
	 *  - Static Function Call  (pickFromMap)
	 *  - Variable Function Call  (result of map lookup)
	 *  - Function Static Call  (fmt.Println)
	 * ----------------------------------------------------------------------- */
	picked := pickFromMap(fnMap, "add")
	fmt.Println(picked(3, 4))

	funcJobs := make(chan func(int) int, 3)
	intJobs  := make(chan int, 3)
	out      := make(chan int, 3)
	_ = intJobs

	go dispatcher(funcJobs, 10, out)

	funcJobs <- double
	funcJobs <- negate
	extra2   := 1
	funcJobs <- func(x int) int { return x + extra2 }
	close(funcJobs)
	fmt.Println(<-out, <-out, <-out)

	fmt.Println(applyBinary(add, 2, 3))
	fmt.Println(applyUnary(double, 5))

	var d Doer
	d = AddDoer{}
	fmt.Println(d.Do(5))
	d = NegDoer{}
	fmt.Println(d.Do(5))

	fn := returnFunc(true)
	fmt.Println(fn(1, 2))

	/* -----------------------------------------------------------------------
	 * NEW: FUNC IN NESTED STRUCT - direct field on outer struct
	 * -----------------------------------------------------------------------
	 * Router.onError is a func field on the outer struct.
	 * Router.middleware.process is a func field one level deep.
	 *
	 * Expected:
	 *   FuncLiteralStores++      (closure capturing errCode)
	 *   FuncInStructOrMap++      (FieldAddr: onError field)
	 *   FuncNamedStores++        (named func: double)
	 *   FuncInStructOrMap++      (FieldAddr: middleware.process field)
	 * ----------------------------------------------------------------------- */
	errCode := 500
	r := Router{}
	r.onError = func(e error) {
		fmt.Println(errCode, e)
	}
	r.middleware.process = double

	/* -----------------------------------------------------------------------
	 * NEW: FUNC IN NESTED STRUCT - HandlerStore inside Middleware
	 * -----------------------------------------------------------------------
	 * r.middleware.fallback is itself a HandlerStore, which has func fields.
	 * Storing into r.middleware.fallback.fn goes two FieldAddr levels deep.
	 *
	 * Expected:
	 *   FuncNamedStores++      (named func: mul)
	 *   FuncInStructOrMap++    (FieldAddr: fallback.fn field)
	 * ----------------------------------------------------------------------- */
	r.middleware.fallback.fn = mul

	/* -----------------------------------------------------------------------
	 * NEW: MAP FIELD ON STRUCT - routes map[string]mathFunc
	 * -----------------------------------------------------------------------
	 * r.routes is a map held inside Router. Writing to it is a MapUpdate
	 * on a map that's accessed via a FieldAddr load.
	 *
	 * Expected:
	 *   FuncInStructOrMap++ x2  (MapUpdate: named func values)
	 *   PotentialTargets++ x2   for mathFunc (same underlying sig as add/mul)
	 * ----------------------------------------------------------------------- */
	r.routes = make(map[string]mathFunc)
	r.routes["add"] = add
	r.routes["mul"] = mul

	/* -----------------------------------------------------------------------
	 * NEW: CALL THROUGH NESTED STRUCT FIELD
	 * -----------------------------------------------------------------------
	 * Calling r.middleware.process and r.onError through the struct fields.
	 * Both are indirect (SSA cannot devirtualise field reads).
	 *
	 * Expected:
	 *   FuncVarCallSites++ x2
	 *   ActualCallSites++ for each signature
	 * ----------------------------------------------------------------------- */
	fmt.Println(r.middleware.process(4))
	r.onError(fmt.Errorf("test error"))

	/* -----------------------------------------------------------------------
	 * NEW: PIPELINE - slice of funcs as a struct field
	 * -----------------------------------------------------------------------
	 * Pipeline.steps is []func(int)int. Adding named functions to the slice
	 * shows up as a Store of a slice (containing func pointers) rather than
	 * direct FieldAddr stores - the individual funcs appear in MapUpdate-like
	 * append SSA instructions.
	 *
	 * Expected:
	 *   FuncNamedStores++ x3   (double, negate, square stored as values
	 *                           in the slice literal or append)
	 *   FuncInStructOrMap++    (FieldAddr: steps field on Pipeline)
	 * ----------------------------------------------------------------------- */
	p := Pipeline{
		steps: []func(int) int{double, negate, square},
	}
	result := 3
	for _, step := range p.steps {
		result = step(result)
	}
	fmt.Println(result)

	/* -----------------------------------------------------------------------
	 * NEW: EVENTBUS - map of func slices
	 * -----------------------------------------------------------------------
	 * EventBus.listeners is map[string][]func(int). Each listener slice
	 * holds multiple callbacks. This tests a map whose value type is a slice
	 * of functions - containsFunc must recurse through Map → Slice → Signature.
	 *
	 * Expected:
	 *   FuncInStructOrMap++ x2  (MapUpdate: listener slices stored)
	 *   PotentialTargets++      for func(int) (each listener counted)
	 * ----------------------------------------------------------------------- */
	bus := EventBus{
		listeners: make(map[string][]func(int)),
	}
	bus.listeners["click"] = []func(int){
		func(x int) { fmt.Println("click A", x) },
		func(x int) { fmt.Println("click B", x) },
	}
	bus.listeners["hover"] = []func(int){
		func(x int) { fmt.Println("hover", x) },
	}

	for _, handlers := range bus.listeners {
		for _, h := range handlers {
			h(42)
		}
	}

	/* -----------------------------------------------------------------------
	 * NEW: STRUCT PASSED AS VALUE CONTAINING FUNC FIELD
	 * -----------------------------------------------------------------------
	 * Passing a HandlerStore by value to a function. The struct itself is
	 * not a func type, but it contains one. This tests whether the analysis
	 * tracks func-containing structs being moved around as arguments.
	 *
	 * applyHandler receives a HandlerStore and calls its fn field -
	 * the call inside applyHandler is a FuncVarCallSites hit reached via
	 * a FieldAddr load on the parameter.
	 *
	 * Expected (inside applyHandler, depth >= 1):
	 *   FuncVarCallSites++   (call through h.fn field)
	 * ----------------------------------------------------------------------- */
	applyHandler := func(h HandlerStore, x int) int {
		return h.fn(x, x) // call through struct field → FuncVarCallSites++
	}
	filled := HandlerStore{fn: add}
	fmt.Println(applyHandler(filled, 5))

	/* -----------------------------------------------------------------------
	 * NEW: FUNC STORED IN STRUCT, THEN STRUCT STORED IN MAP
	 * -----------------------------------------------------------------------
	 * HandlerStore values (which contain func fields) stored into a map.
	 * containsFunc must recurse: Map → Struct → Signature.
	 *
	 * Expected:
	 *   FuncInStructOrMap++ x2  (MapUpdate: struct-containing-func values)
	 * ----------------------------------------------------------------------- */
	registry := map[string]HandlerStore{
		"add": {fn: add, fallback: increment},
		"mul": {fn: mul, fallback: square},
	}
	for _, entry := range registry {
		fmt.Println(entry.fn(2, 3))
	}
}