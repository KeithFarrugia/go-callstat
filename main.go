package main

import (
	"log"

	cs_callgraph "callstat/CS-Callgraph"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func main() {
	// --- Load Go packages ---
	cfg := &packages.Config{
		Mode: packages.LoadAllSyntax,
		Dir:  "../dep-usage-test/", // path to your project
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		log.Fatal(err)
	}

	// --- Build SSA program ---
	prog, _ := ssautil.AllPackages(pkgs, ssa.BuilderMode(0))
	prog.Build()

	// --- Build call graph ---
	cg := cs_callgraph.BuildExtendedCallGraph(prog)

	dot := BuildDotGraphFromCS(cg)
	err = dot.WriteDOTToFile("cg.dot")
	if err != nil {
		panic(err)
	}
}
