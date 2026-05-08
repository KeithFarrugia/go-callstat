package main

import (
	"bufio"
	cs_callgraph "callstat/CS-Callgraph"
	stats "callstat/Statistics"
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
    depthFlag := flag.Int("depth", 1, "Depth of external package analysis (-1 for infinite)")
    targetDir := flag.String("dir", "../go-callvis/", "Directory of the project to analyze")
    flag.Parse()

    projectRoot := GetModuleName(*targetDir)
    if projectRoot == "" {
        log.Printf("Warning: Could not find go.mod in %s or parents.", *targetDir)
    } else {
        fmt.Printf("[info] Detected project root: %s\n", projectRoot)
    }

    const runs = 1
    var totalMs int64

    for i := 0; i < runs; i++ {
        cs_callgraph.EffectivePkgCache = sync.Map{}
        loopStart := time.Now()

        // --- STAGE 1: LOAD PACKAGES ---
        tLoad := time.Now()
        cfg := &packages.Config{
            Mode: packages.LoadAllSyntax,
            Dir:  *targetDir,
        }
        pkgs, err := packages.Load(cfg, "./...")
        if err != nil {
            log.Fatal(err)
        }
        fmt.Printf("[timer] Package Load: %v\n", time.Since(tLoad))

        // --- STAGE 2: BUILD SSA ---
        tSSA := time.Now()
        prog, _ := ssautil.AllPackages(pkgs, ssa.BuilderMode(0))
        prog.Build()
        fmt.Printf("[timer] SSA Build:    %v\n", time.Since(tSSA))

        // --- STAGE 3: CALLGRAPH GENERATION ---
        tCG := time.Now()
        depthMap := cs_callgraph.BuildPackageDepthMap(prog, projectRoot)
        cg := cs_callgraph.BuildExtendedCallGraph2(prog, *depthFlag, depthMap)
        fmt.Printf("[timer] CallGraph:   %v\n", time.Since(tCG))

        // --- STAGE 4: STATISTICS ---
        tStats := time.Now()
        statsObj := stats.GatherCallGraphStats(cg, depthMap, *depthFlag, projectRoot)
        statsObj.WriteJSONToFile("output/callgraph_report.json")
        fmt.Printf("[timer] Statistics:  %v\n", time.Since(tStats))

        // --- STAGE 5: VISUALIZATION (The Concurrent Part) ---
        tVis := time.Now()
        skipPkg := map[string]struct{}{
            "runtime":          {},
            "runtime/internal": {},
            "sync":             {},
            "go_types":         {},
            "types":            {},
        }
        
        err = visualisation.GenerateHTMLReport(
            cg,
            "./output/dot",
            "./output/svg",
            "./report.html",
            0, // Your concurrency factor
            skipPkg,
            depthMap,
            *depthFlag,
            "output/callgraph_report.json",
            projectRoot,
        )
        if err != nil {
            log.Fatal(err)
        }
        fmt.Printf("[timer] Visuals:     %v\n", time.Since(tVis))

        elapsed := time.Since(loopStart).Milliseconds()
        fmt.Printf("[run %2d] Total: %dms\n", i+1, elapsed)
        totalMs += elapsed
    }

    fmt.Printf("\n[average] %dms over %d runs\n", totalMs/runs, runs)
}