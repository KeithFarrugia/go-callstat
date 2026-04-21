package main

import (
	"log"

	cs_callgraph "callstat/CS-Callgraph"
	visualisation "callstat/Visualisation"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)


func main() {
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
}