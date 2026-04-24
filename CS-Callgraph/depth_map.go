package cs_callgraph

import (
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