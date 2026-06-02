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

/* ============================================================================
 * stringSlice
 * ----------------------------------------------------------------------------
 * A repeatable flag value - each --flag=x appends to the slice.
 *
 *   --skip-vis=runtime/ --skip-vis=go/types
 * ============================================================================
 */
type stringSlice []string

func (s *stringSlice) String() string     { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

/* ============================================================================
 * GetModuleName
 * ----------------------------------------------------------------------------
 * Climbs the directory tree from targetDir looking for a go.mod file.
 * Returns the declared module name, or "" if none is found.
 *
 * Uses os.ReadFile to avoid a defer-inside-loop leak.
 * ============================================================================
 */
func GetModuleName(targetDir string) string {
    absDir, err := filepath.Abs(targetDir)
    if err != nil {
        return ""
    }
    for curr := absDir; ; curr = filepath.Dir(curr) {
        data, err := os.ReadFile(filepath.Join(curr, "go.mod"))
        if err == nil {
            scanner := bufio.NewScanner(strings.NewReader(string(data)))
            if scanner.Scan() {
                line := strings.TrimSpace(scanner.Text())
                return strings.TrimPrefix(line, "module ")
            }
        }
        if parent := filepath.Dir(curr); parent == curr {
            break
        }
    }
    return ""
}


/* ============================================================================
 * matchesPattern
 * ----------------------------------------------------------------------------
 * Reports whether pkgPath matches any pattern in the list.
 *
 * Two match modes:
 *   "runtime"   exact match - only "runtime" itself
 *   "runtime/"  prefix match - "runtime", "runtime/internal", etc.
 * ============================================================================
 */
func matchesPattern(pkgPath string, patterns []string) bool {
    for _, p := range patterns {
        if strings.HasSuffix(p, "/") {
            base := strings.TrimSuffix(p, "/")
            if pkgPath == base || strings.HasPrefix(pkgPath, base+"/") {
                return true
            }
        } else if pkgPath == p {
            return true
        }
    }
    return false
}

/* ============================================================================
 * buildSkipMap
 * ----------------------------------------------------------------------------
 * Expands a list of patterns (and optionally all stdlib packages) into a
 * concrete map[pkgPath]struct{} by testing every known package path.
 * ============================================================================
 */
func buildSkipMap(
    patterns      []string,
    excludeStdlib bool,
    allPkgPaths   []string,
) map[string]struct{} {
    result := make(map[string]struct{})
    for _, path := range allPkgPaths {
        patternMatch := matchesPattern(path, patterns)
        stdlibMatch  := excludeStdlib && cs_callgraph.IsStdlib(path)
        if patternMatch || stdlibMatch {
            result[path] = struct{}{}
        }
    }
    return result
}

/* ============================================================================
 * main
 * ============================================================================
 */
func main() {
    /* -------------------------------------------------------
     * Flags
     * ------------------------------------------------------- */
    var skipCGPatterns  stringSlice
    var skipVisPatterns stringSlice

    depthFlag := flag.Int("depth", 2,
        "Depth of external package traversal (-1 = unlimited)")
    targetDir := flag.String("dir", "../dep-usage-test/",
        "Directory of the project to analyse")
    reportOut := flag.String("report", "./report.html",
        "Path for the HTML report output")
    dotDir := flag.String("dot-dir", "./output/dot",
        "Directory for intermediate DOT files")
    svgDir := flag.String("svg-dir", "./output/svg",
        "Directory for intermediate SVG files")
    statsOut := flag.String("stats", "./output/callgraph_report.json",
        "Path for the stats JSON output")
    noStdlib := flag.Bool("no-stdlib", false,
        "Exclude the standard library from callgraph traversal")
    
    noStats := flag.Bool("no-stats", false, 
        "Disable statistics calculation and JSON output")
    noVis := flag.Bool("no-vis", false, 
        "Disable DOT/SVG generation and visualization parts")
    
    mainEntry := flag.String("main", "",
        "Fully qualified main function to use as entry point "+
            "(e.g. 'github.com/you/repo/cmd/serve.main'); "+
            "defaults to automatic detection")
            
    flag.Var(&skipCGPatterns, "skip-cg",
        "Exclude from callgraph (repeatable; trailing / = prefix match)")
    flag.Var(&skipVisPatterns, "skip-vis",
        "Exclude from visualisation (repeatable; trailing / = prefix match)")


    flag.Parse()
    /* -------------------------------------------------------
     * Project root detection
     * ------------------------------------------------------- */
    projectRoot := GetModuleName(*targetDir)
    if projectRoot == "" {
        log.Printf("[warn] could not find go.mod in %s or parents", *targetDir)
    } else {
        fmt.Printf("[info] project root: %s\n", projectRoot)
    }

    /* -------------------------------------------------------
     * Benchmark loop
     * ------------------------------------------------------- */
    cs_callgraph.InitSTDLib();
    cs_callgraph.EffectivePkgCache = sync.Map{}
    totalTimeStart := time.Now()

    /* -------------------------------------------------------
    * Load Packages
    * ------------------------------------------------------- */
    t := time.Now()
    pkgs, err := packages.Load(&packages.Config{
        Mode: packages.LoadAllSyntax,
        Dir:  *targetDir,
    }, "./...")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("[timer] package load  %v\n", time.Since(t))

    /* -------------------------------------------------------
    * Build SSA
    * ------------------------------------------------------- */
    t = time.Now()
    prog, _ := ssautil.AllPackages(pkgs, ssa.BuilderMode(0))
    prog.Build()
    fmt.Printf("[timer] SSA build     %v\n", time.Since(t))

    // - Collect all known package paths for skip expansion
    allPkgPaths := make([]string, 0, len(prog.AllPackages()))
    for _, pkg := range prog.AllPackages() {
        if pkg.Pkg != nil {
            allPkgPaths = append(allPkgPaths, pkg.Pkg.Path())
        }
    }

    /* -------------------------------------------------------
    * Callgraph
    * ------------------------------------------------------- */
    t = time.Now()
    targetMainPkg := cs_callgraph.ResolveMain(prog, projectRoot, *mainEntry)
    skipCGMap := buildSkipMap(skipCGPatterns, *noStdlib, allPkgPaths)
    depthMap  := cs_callgraph.BuildPackageDepthMapFromMain(prog, projectRoot, targetMainPkg.Packg)
    
    cg        := cs_callgraph.BuildExtendedCallGraph2(
        prog, *depthFlag, depthMap, skipCGMap,
    )
    fmt.Printf("[timer] callgraph     %v\n", time.Since(t))

    /* -------------------------------------------------------
    * Statistics
    * ------------------------------------------------------- */
    if !*noStats {
        t = time.Now()
        statsObj := stats.GatherCallGraphStats(
            cg, depthMap, *depthFlag, projectRoot, targetMainPkg.Funct, skipCGMap,
        )
        statsObj.WriteJSONToFile(*statsOut)
        fmt.Printf("[timer] statistics    %v\n", time.Since(t))
    }

    /* -------------------------------------------------------
    * Visualisation
    * ------------------------------------------------------- */
    if !*noVis {
        t = time.Now()
        skipVisMap := buildSkipMap(skipVisPatterns, *noStdlib, allPkgPaths)
        err = visualisation.GenerateHTMLReport(
            cg, *dotDir, *svgDir, *reportOut,
            0, skipVisMap, depthMap, *depthFlag, *statsOut, projectRoot,
        )
        if err != nil {
            log.Fatal(err)
        }
        fmt.Printf("[timer] visualisation %v\n", time.Since(t))
    }

    totalTimeFinished := time.Since(totalTimeStart).Milliseconds()

    fmt.Printf("\n[average] %dms", totalTimeFinished)
}