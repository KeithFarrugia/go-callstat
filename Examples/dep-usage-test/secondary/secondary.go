package secondary

import (
	"fmt"
	"net/http"
	"time"
)

type Test struct {
    x int
}

type GenericTest[T any] struct {
    Data T
}

func (g *GenericTest[T]) Process(val T) {
    fmt.Println("Processing generic data")
}

func (b *Test) DoingAThing() int {
    return 0
}

func (b *Test) Ping() {
    fmt.Println("ping")
    _ = http.MethodGet
    _ = time.Second
}

func (b* Test)Say(){
    fmt.Println("Hello");
}
/* ============================================================================
 * Doer interface (secondary package)
 * ----------------------------------------------------------------------------
 *   Do        → used via interface dispatch in main
 *   Report    → never called through interface (unused interface method)
 * ============================================================================
 */
type Doer interface {
    Do(x int) int
    Report() string
}

/* ============================================================================
 * Concrete implementations of Doer
 * ============================================================================
 */
type AddDoer struct{}
type NegDoer struct{}

func (a AddDoer) Do(x int) int    { return x + 1 }
func (n NegDoer) Do(x int) int    { return -x }
func (a AddDoer) Report() string  { return "add" }
func (n NegDoer) Report() string  { return "neg" }