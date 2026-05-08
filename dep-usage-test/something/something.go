package something

import (
	"fmt"
	"net/http"
	"time"
)

type Bob struct {
    x int
}

type GenericBob[T any] struct {
    Data T
}

func (g *GenericBob[T]) Process(val T) {
    fmt.Println("Processing generic data")
}

func (b *Bob) DoingAThing() int {
    return 0
}

func (b *Bob) Ping() {
    fmt.Println("ping")
    _ = http.MethodGet
    _ = time.Second
}