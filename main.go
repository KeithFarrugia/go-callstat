package main

import (
	"bufio"
	cs_callgraph "callstat/CS-Callgraph"
	visualisation "callstat/Visualisation"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// GetModuleName climbs the tree from the target directory to find the go.mod file
func GetModuleName(targetDir string) string {
    absDir, err := filepath.Abs(targetDir)
    if err != nil {
        return ""
    }

    curr := absDir
    for {
        modPath := filepath.Join(curr, "go.mod")
        if _, err := os.Stat(modPath); err == nil {
            file, _ := os.Open(modPath)
            defer file.Close()
            scanner := bufio.NewScanner(file)
            if scanner.Scan() {
                line := scanner.Text()
                return strings.TrimPrefix(line, "module ")
            }
        }
        parent := filepath.Dir(curr)
        if parent == curr {
            break
        }
        curr = parent
    }
    return ""
}

func main() {
    // 1. Setup Flags
    depthFlag := flag.Int("depth", -1, "Depth of external package analysis (-1 for infinite)")
    targetDir := flag.String("dir", "../dep-usage-test/", "Directory of the project to analyze")
    flag.Parse()

    // 2. Resolve Project Root Dynamically
    projectRoot := GetModuleName(*targetDir)
    if projectRoot == "" {
        log.Printf("Warning: Could not find go.mod in %s or parents. External package depth logic might fail.", *targetDir)
    } else {
        fmt.Printf("[info] Detected project root: %s\n", projectRoot)
    }

    const runs = 1
    var totalMs int64

    for i := 0; i < runs; i++ {
        cs_callgraph.EffectivePkgCache = sync.Map{}
        start := time.Now()

        // 3. Load Packages
        cfg := &packages.Config{
            Mode: packages.LoadAllSyntax,
            Dir:  *targetDir,
        }
        pkgs, err := packages.Load(cfg, "./...")
        if err != nil {
            log.Fatal(err)
        }

        // 4. Build SSA
        prog, _ := ssautil.AllPackages(pkgs, ssa.BuilderMode(0))
        prog.Build()

        // 5. Build CallGraph with Depth and Dynamic Root
        // Ensure your BuildExtendedCallGraph2 signature is updated to:
        // func BuildExtendedCallGraph2(prog *ssa.Program, maxDepth int, projectRoot string)
        depthMap := cs_callgraph.BuildPackageDepthMap(prog, "example.com/depusagetest")
        cg := cs_callgraph.BuildExtendedCallGraph2(prog, *depthFlag, depthMap)

        // 6. Visualization
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
            depthMap,
            *depthFlag,
        ); err != nil {
            log.Fatal(err)
        }

        elapsed := time.Since(start).Milliseconds()
        fmt.Printf("[run %2d] %dms\n", i+1, elapsed)
        totalMs += elapsed
    }

    fmt.Printf("\n[average] %dms over %d runs\n", totalMs/runs, runs)
}