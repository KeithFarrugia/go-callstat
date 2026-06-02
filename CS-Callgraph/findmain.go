package cs_callgraph

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
)

type FoundMain struct {
    Packg  *ssa.Package
    Funct *ssa.Function
}

func ResolveMain(
	prog        *ssa.Program,
	projectRoot string,
	mainFlag    string,
) *FoundMain {
	if mainFlag != "" {
		return resolveExplicitMain(prog, mainFlag)
	}
	return findPossibleMain(prog, projectRoot)
}

/* ============================================================================
 * resolveExplicitMainPackage
 * ----------------------------------------------------------------------------
 * Extracts the target package path from a fully qualified string flag 
 * (e.g. "github.com/restic/restic/cmd/restic.main") and checks if it exists.
 * ============================================================================
 */
func resolveExplicitMain(prog *ssa.Program, mainFlag string) *FoundMain {
    lastDot := strings.LastIndex(mainFlag, ".")
    if lastDot == -1 {
        fmt.Printf("[ResolveMain] invalid explicit main flag format: %s\n", mainFlag)
        os.Exit(-1)
    }
    targetPkgPath := mainFlag[:lastDot]
    impPkg := prog.ImportedPackage(targetPkgPath)
    if impPkg == nil {
        fmt.Printf("[ResolveMain] error: package %q not found\n", targetPkgPath)
        os.Exit(-1)
    }
    mainPkg := prog.Package(impPkg.Pkg)
    if mainPkg == nil || mainPkg.Pkg == nil {
        fmt.Printf("[ResolveMain] error: structural package missing for %q\n", targetPkgPath)
        os.Exit(-1)
    }

    funcName := mainFlag[lastDot+1:]
    mainFunc := mainPkg.Func(funcName)
    if mainFunc == nil {
        fmt.Printf("[ResolveMain] error: function %q not found in package %q\n", funcName, targetPkgPath)
        os.Exit(-1)
    }

    fmt.Printf("[ResolveMain] Exact match: %s\n", mainFlag)
    return &FoundMain{Packg: mainPkg, Funct: mainFunc}
}


/* ============================================================================
 * findPossibleMainPackage
 * ----------------------------------------------------------------------------
 * Scans all packages in the ssa.Program to discover and rank potential 
 * main entry points.
 *
 * Enforces your strict selection hierarchy across project root packages:
 *   Priority 1: A package named "main" containing a main() func within projectRoot.
 *   Priority 2: Any other named package containing a main() func within projectRoot.
 * ============================================================================
 */

func findPossibleMain(prog *ssa.Program, projectRoot string) *FoundMain {
    type candidate struct {
        pkg  *ssa.Package
        fn   *ssa.Function
    }
    var priority1, priority2 []candidate

    for _, pkg := range prog.AllPackages() {
        if pkg.Pkg == nil {
            continue
        }
        pkgPath := pkg.Pkg.Path()
        if !(
            pkgPath == projectRoot || 
            strings.HasPrefix(pkgPath, projectRoot+"/") ){
            continue
        }
        mainFunc := pkg.Func("main")
        if mainFunc == nil {
            continue
        }
        c := candidate{pkg: pkg, fn: mainFunc}
        if pkg.Pkg.Name() == "main" {
            priority1 = append(priority1, c)
        } else {
            priority2 = append(priority2, c)
        }
    }

    sort.Slice(priority1, func(i, j int) bool {
        return priority1[i].pkg.Pkg.Path() < priority1[j].pkg.Pkg.Path()
    })
    sort.Slice(priority2, func(i, j int) bool {
        return priority2[i].pkg.Pkg.Path() < priority2[j].pkg.Pkg.Path()
    })

    total := len(priority1) + len(priority2)
    if total > 0 {
        fmt.Printf("[ResolveMain] Found %d potential main(s):\n", total)
        for _, c := range priority1 {
            fmt.Printf("  -> [Priority 1]: %s.main\n", c.pkg.Pkg.Path())
        }
        for _, c := range priority2 {
            fmt.Printf("  -> [Priority 2]: %s.main\n", c.pkg.Pkg.Path())
        }
    }
    if len(priority1) > 1 || (len(priority1) == 0 && len(priority2) > 1) {
        fmt.Println("[WARNING]: Multiple possible main packages found.")
    }

    if len(priority1) > 0 {
        fmt.Printf("[ResolveMain] selected: %s\n", priority1[0].pkg.Pkg.Path())
        return &FoundMain{Packg: priority1[0].pkg, Funct: priority1[0].fn}
    }
    if len(priority2) > 0 {
        fmt.Printf("[ResolveMain] selected: %s\n", priority2[0].pkg.Pkg.Path())
        return &FoundMain{Packg: priority2[0].pkg, Funct: priority2[0].fn}
    }

    fmt.Printf("[ResolveMain] error: no main found\n")
    os.Exit(-1)
    return nil
}