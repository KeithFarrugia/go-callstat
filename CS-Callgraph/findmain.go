package cs_callgraph

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
)
func ResolveMainPackage(
	prog        *ssa.Program,
	projectRoot string,
	mainFlag    string,
) *ssa.Package {
	if mainFlag != "" {
		return resolveExplicitMainPackage(prog, mainFlag)
	}
	return findPossibleMainPackage(prog, projectRoot)
}

/* ============================================================================
 * resolveExplicitMainPackage
 * ----------------------------------------------------------------------------
 * Extracts the target package path from a fully qualified string flag 
 * (e.g. "github.com/restic/restic/cmd/restic.main") and checks if it exists.
 * ============================================================================
 */
func resolveExplicitMainPackage(prog *ssa.Program, mainFlag string) *ssa.Package {
	lastDot := strings.LastIndex(mainFlag, ".")
	if lastDot == -1 {
		fmt.Printf("[ResolveMainPackage] invalid explicit main flag format: %s\n", mainFlag)
		os.Exit(-1)
	}
	targetPkgPath := mainFlag[:lastDot]

	impPkg := prog.ImportedPackage(targetPkgPath)
	if impPkg == nil {
		fmt.Printf("[ResolveMainPackage] error: package %q not found in program\n", targetPkgPath)
		os.Exit(-1)
	}

	mainPkg := prog.Package(impPkg.Pkg)
	if mainPkg == nil || mainPkg.Pkg == nil {
		fmt.Printf("[ResolveMainPackage] error: structural package object missing for %q\n", targetPkgPath)
		os.Exit(-1)
	}

	fmt.Printf("[ResolveMainPackage] Exact match found: %s\n", mainPkg.Pkg.Path())
	return mainPkg
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
func findPossibleMainPackage(prog *ssa.Program, projectRoot string) *ssa.Package {
	var priority1 []*ssa.Package
	var priority2 []*ssa.Package

	for _, pkg := range prog.AllPackages() {
		if pkg.Pkg == nil {
			continue
		}

		pkgPath := pkg.Pkg.Path()

		// Verify this package belongs to the project codebase
		if strings.HasPrefix(pkgPath, projectRoot) {
			// Look for a physical function named "main" defined in this package block
			if mainFunc := pkg.Func("main"); mainFunc != nil {
				if pkg.Pkg.Name() == "main" {
					priority1 = append(priority1, pkg)
				} else {
					priority2 = append(priority2, pkg)
				}
			}
		}
	}

	// Replicate your alphabetical fallback sorting using the Package path strings
	sort.Slice(priority1, func(i, j int) bool {
		return priority1[i].Pkg.Path() < priority1[j].Pkg.Path()
	})
	sort.Slice(priority2, func(i, j int) bool {
		return priority2[i].Pkg.Path() < priority2[j].Pkg.Path()
	})

	totalMains := len(priority1) + len(priority2)
	if totalMains > 0 {
		fmt.Printf("[ResolveMainPackage] Found %d total potential main package(s):\n", totalMains)
		for _, pkg := range priority1 {
			fmt.Printf("  -> [Priority 1] (pkg: main): %s.main\n", pkg.Pkg.Path())
		}
		for _, pkg := range priority2 {
			fmt.Printf("  -> [Priority 2] (pkg: other): %s.main\n", pkg.Pkg.Path())
		}
	}

	if len(priority1) > 1 || (len(priority1) == 0 && len(priority2) > 1) {
		fmt.Println("[WARNING]: Multiple possible main packages found.")
	}

	if len(priority1) > 0 {
		fmt.Printf("[ResolveMainPackage] selected priority main package: %s\n", priority1[0].Pkg.Path())
		return priority1[0]
	} else if len(priority2) > 0 {
		fmt.Printf("[ResolveMainPackage] selected priority main package: %s\n", priority2[0].Pkg.Path())
		return priority2[0]
	}

	fmt.Printf("[ResolveMainPackage] error: no package containing a main function found\n")
	os.Exit(-1)
	return nil
}