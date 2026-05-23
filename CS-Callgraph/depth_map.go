package cs_callgraph

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/go/ssa"
)

/* ============================================================================
 * BuildPackageDepthMap
 * ----------------------------------------------------------------------------
 * Walks the import graph BFS-style from the internal packages (depth 0) and
 * assigns each reachable package the shallowest depth at which it appears.
 *
 * Internal packages are always depth 0. Each layer of imports increments
 * depth by 1. Returns a map of package path -> depth.
 * ============================================================================
 */
func BuildPackageDepthMap(
    prog        *ssa.Program,
    projectRoot string,
) map[string]int {
    depthMap := map[string]int{}
    queue    := []*ssa.Package{}

    // Seed with internal packages at depth 0
    for _, pkg := range prog.AllPackages() {
        if pkg.Pkg == nil {
            continue
        }
        if strings.HasPrefix(pkg.Pkg.Path(), projectRoot) {
            depthMap[pkg.Pkg.Path()] = 0
            queue = append(queue, pkg)
        }
    }

    // BFS over the import graph
    for len(queue) > 0 {
        current := queue[0]
        queue    = queue[1:]

        currentDepth := depthMap[current.Pkg.Path()]

        for _, imported := range current.Pkg.Imports() {
            importPath := imported.Path()
            nextDepth  := currentDepth + 1

            if prev, seen := depthMap[importPath]; !seen || nextDepth < prev {
                depthMap[importPath] = nextDepth

                if ssaPkg := prog.Package(imported); ssaPkg != nil {
                    queue = append(queue, ssaPkg)
                }
            }
        }
    }

    return depthMap
}



/* ============================================================================
 * BuildPackageDepthMapFromMain
 * ----------------------------------------------------------------------------
 * Performs a single-pass BFS traversal starting from the resolved main entry 
 * package. 
 *
 * Any package not reachable from this package's import chain is excluded.
 * Internal project packages are clamped to depth 0, while external or standard
 * library dependencies scale outward (+1 depth per hop).
 * ============================================================================
 */
func BuildPackageDepthMapFromMain(
    prog        *ssa.Program,
    projectRoot string,
    mainPkg     *ssa.Package,
) map[string]int {
    // Safety check to ensure the passed package structure is initialized
    if mainPkg == nil || mainPkg.Pkg == nil {
        fmt.Printf("[DepthMap] error: provided main package is nil or uninitialized\n")
        os.Exit(-1)
    }

    fmt.Printf("[DepthMap] Building depth map rooted at entry package: %s\n", mainPkg.Pkg.Path())

    depthMap := map[string]int{}
    queue    := []*ssa.Package{}

    // Seed the traversal exclusively with our main package at depth 0
    depthMap[mainPkg.Pkg.Path()] = 0
    queue = append(queue, mainPkg)

    for len(queue) > 0 {
        current := queue[0]
        queue    = queue[1:]

        currentDepth := depthMap[current.Pkg.Path()]

        for _, imported := range current.Pkg.Imports() {
            importPath := imported.Path()
            
            nextDepth := currentDepth + 1
            if strings.HasPrefix(importPath, projectRoot) {
                nextDepth = 0
            }
            
            if prev, seen := depthMap[importPath]; !seen || nextDepth < prev {
                depthMap[importPath] = nextDepth

                if ssaPkg := prog.Package(imported); ssaPkg != nil {
                    queue = append(queue, ssaPkg)
                }
            }
        }
    }

    return depthMap
}