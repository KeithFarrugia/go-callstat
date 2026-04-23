package main

import (
	cs_callgraph "callstat/CS-Callgraph"
	visualisation "callstat/Visualisation"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func track(name string) func() {
    start := time.Now()
    return func() {
        fmt.Printf("[timer] %-35s %dms\n", name, time.Since(start).Milliseconds())
    }
}

func main() {
    const runs = 20
    var totalMs int64

    for i := range runs {
        cs_callgraph.EffectivePkgCache = sync.Map{}
        start := time.Now()
        cfg := &packages.Config{
            Mode: packages.LoadAllSyntax,
            Dir:  "../dep-usage-test/",
        }
        pkgs, err := packages.Load(cfg, "./...")
        if err != nil {
            log.Fatal(err)
        }
        prog, _ := ssautil.AllPackages(pkgs, ssa.BuilderMode(0))
        prog.Build()
        cg := cs_callgraph.BuildExtendedCallGraph2(prog)
        skipPkg := map[string]struct{}{
            "runtime":          {},
            "runtime/internal": {},
            "sync":             {},
        }
        if err := visualisation.GenerateHTMLReport(
            cg,
            "./output/dot",
            "./output/svg",
            "./report.html",
            4,
            skipPkg,
        ); err != nil {
            log.Fatal(err)
        }

        elapsed := time.Since(start).Milliseconds()
        fmt.Printf("[run %2d] %dms\n", i+1, elapsed)
        totalMs += elapsed
    }

    fmt.Printf("\n[average] %dms over %d runs\n", totalMs/runs, runs)
}