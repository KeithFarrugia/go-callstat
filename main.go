package main

import (
	cs_callgraph "callstat/CS-Callgraph"
	"fmt"
	"log"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// func getPackages(pkgs []*ssa.Package) ([]*ssa.Package, error) {
// 	var p_set []*ssa.Package
// 	for _, p := range pkgs {
// 		if p != nil {
// 			p_set = append(p_set, p)
// 		}
// 	}
// 	if len(p_set) == 0 {
// 		return nil, fmt.Errorf("No packages Found")
// 	}
// 	return p_set, nil
// }

// func printCalls(fn *ssa.Function) {
// 	for _, block := range fn.Blocks {
// 		for _, instr := range block.Instrs {

// 			var call *ssa.CallCommon
// 			var kind string

// 			switch i := instr.(type) {
// 			case *ssa.Call:
// 				call = &i.Call
// 				kind = "call"
// 			case *ssa.Go:
// 				call = &i.Call
// 				kind = "go"
// 			case *ssa.Defer:
// 				call = &i.Call
// 				kind = "defer"

// 			default:
// 				continue
// 			}

// 			// Direct call (static target)
// 			if callee := call.StaticCallee(); callee != nil {
// 				fmt.Printf("    %-6s -> %s\n", kind, callee.String())
// 				continue
// 			}

// 			// Interface / dynamic call
// 			fmt.Printf("    %-6s -> dynamic call: %s\n", kind, call.Value)
// 		}
// 	}
// }

// func parseFunctions(prog *ssa.Program, target string, op bool) {
// 	for fn := range ssautil.AllFunctions(prog) {
// 		if fn == nil || fn.Pkg == nil {
// 			continue
// 		}

// 		if !strings.Contains(fn.Pkg.Pkg.Path(), target) {
// 			continue
// 		}

// 		fmt.Println("\nSSA Function:", fn.String())
// 		if op {
// 			analyseFunction(fn)
// 		} else {
// 			printCalls(fn)
// 		}
// 	}
// }

// func mainPackages(pkgs []*ssa.Package) ([]*ssa.Package, error) {
// 	var mains []*ssa.Package
// 	for _, p := range pkgs {
// 		if p != nil && p.Pkg.Name() == "main" && p.Func("main") != nil {
// 			mains = append(mains, p)
// 		}
// 	}
// 	if len(mains) == 0 {
// 		return nil, fmt.Errorf("no main packages")
// 	}
// 	return mains, nil
// }

func main() {

	// =============================================================
	// 1. Load Go packages from the current module
	cfg := &packages.Config{
		Mode: packages.LoadAllSyntax,
		Dir:  "../dep-usage-test/", // project root
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		log.Fatal(err)
	}

	// 2. Build SSA
	prog, _ := ssautil.AllPackages(pkgs, ssa.BuilderMode(0))
	prog.Build()

	// =============================================================

	cg := cs_callgraph.BuildExtendedCallGraph(prog)

	var mainSSA *ssa.Package
	for _, p := range prog.AllPackages() {
		if p != nil && p.Pkg != nil && p.Pkg.Name() == "main" && p.Func("main") != nil {
			mainSSA = p
			break
		}
	}

	if mainSSA == nil {
		log.Fatal("no main package found")
	}

	filter := &EdgeFilter{
		FocusPkg: mainSSA.Pkg, // ✅ just Pkg, no extra .Pkg
		NoStd:    true,
		NoInter:  false,
		LimitPaths: []string{
			mainSSA.Pkg.Path(), // ✅ use Path() from types.Package
		},
	}

	dot, err := RenderGraphviz(prog, cg, filter)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(dot)
	// 3. Call your function
	// mains, err := getPackages(ssaPkgs)
	// if err != nil {
	//     log.Fatal(err)
	// }

	// // 4. Print the result
	// for _, p := range mains {
	//     fmt.Println("Found main package:", p.Pkg.Path())
	//     fmt.Println("  main function:", p.Func("main"))
	// }

	// parseFunctions(prog, "example.com/depusagetest", true)

	// fmt.Println("\n \n ============================================================= \n Channels \n============================================================= \n\n")

	// fmt.Println()
	// // analyseFuncChannelsCHA(prog, "example.com/depusagetest")
}
