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