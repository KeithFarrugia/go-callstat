package tracing

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

func mapFuncsReturningChanFunc(pkgs []*ssa.Package) map[*ssa.Function]bool {
	result := make(map[*ssa.Function]bool)

	for _, pkg := range pkgs {
		for _, mem := range pkg.Members {
			fn, ok := mem.(*ssa.Function)
			if !ok || fn.Signature == nil {
				continue
			}

			// Handle multiple return values
			results := fn.Signature.Results()
			if results == nil {
				continue
			}

			visited := make(map[types.Type]bool)
			if containsChanOfFunc(results, visited) {
				result[fn] = true
			}
		}
	}

	return result
}
