package cs_callgraph

import "golang.org/x/tools/go/packages"

var stdPackages = map[string]struct{}{}

func InitSTDLib() {
    pkgs, err := packages.Load(nil, "std")
    if err != nil {
        panic(err)
    }
    for _, p := range pkgs {
        stdPackages[p.PkgPath] = struct{}{}
    }
}

func IsStdlib(pkgPath string) bool {
    _, ok := stdPackages[pkgPath]
    return ok
}