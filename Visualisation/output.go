package visualisation

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	cs_callgraph "callstat/CS-Callgraph"
)

/* ============================================================================
 * GenerateDOTAndSVG
 * ----------------------------------------------------------------------------
 * Builds DotGraphs per package from a call graph and generates DOT + SVG files.
 * Ensures styles are loaded once.
 * Supports optional concurrency (goroutines per package).
 *
 * Params:
 *   - cg: callgraph to visualize
 *   - dotDir: folder for DOT files
 *   - svgDir: folder for SVG files
 *   - concurrency: number of goroutines (0 = sequential)
 * ============================================================================
 */
func GenerateDOTAndSVG(
    cg          *cs_callgraph.Graph,
    dotDir      string,
    svgDir      string,
    concurrency int,
    skipPkg     map[string]struct{},   // <-- add this
) {
	/* -------------------------------------------------------
	 * Ensure styles are loaded once
	 * ------------------------------------------------------- */
	if global_styles == nil {
		if err := LoadInternalStyles(); err != nil {
			log.Fatalf("failed to load internal styles: %v", err)
		}
	}

	/* -------------------------------------------------------
	 * Build DOT graphs per package
	 * ------------------------------------------------------- */
	graphs := BuildDotGraphPerPackage(cg, skipPkg)

	/* -------------------------------------------------------
	 * Ensure output directories exist
	 * ------------------------------------------------------- */
	if err := os.MkdirAll(dotDir, os.ModePerm); err != nil {
		log.Fatalf("failed to create dot folder: %v", err)
	}
	if err := os.MkdirAll(svgDir, os.ModePerm); err != nil {
		log.Fatalf("failed to create svg folder: %v", err)
	}

	/* -------------------------------------------------------
	 * Worker function to process a single package
	 * ------------------------------------------------------- */
	processPkg := func(pkg, sanitized string, dg *DotGraph) {
		dotPath := filepath.Join(dotDir, sanitized+".dot")
		svgPath := filepath.Join(svgDir, sanitized+".svg")

		// Write DOT file
		if err := dg.WriteDOTToFile(dotPath); err != nil {
			log.Printf("[ERROR] Failed to write DOT %s: %v", dotPath, err)
			return
		}

		// Generate SVG
		if err := generateSVG(dotPath, svgPath); err != nil {
			log.Printf("[ERROR] Failed to generate SVG %s: %v", svgPath, err)
			return
		}

		log.Printf("[OK] Generated DOT + SVG for package: %s", pkg)
	}

	/* -------------------------------------------------------
	 * Concurrent / Sequential execution
	 * ------------------------------------------------------- */
	if concurrency <= 0 {
		// Sequential
		for pkg, dg := range graphs {
			sanitized := strings.ReplaceAll(pkg, "/", "_")
			processPkg(pkg, sanitized, dg)
		}
	} else {
		// Concurrent
		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup

		for pkg, dg := range graphs {
			wg.Add(1)
			sem <- struct{}{}

			go func(pkg string, dg *DotGraph) {
				defer wg.Done()
				defer func() { <-sem }()
				sanitized := strings.ReplaceAll(pkg, "/", "_")
				processPkg(pkg, sanitized, dg)
			}(pkg, dg)
		}

		wg.Wait()
	}
}

/* ============================================================================
 * generateSVG
 * ----------------------------------------------------------------------------
 * Calls Graphviz dot CLI to produce an SVG from a DOT file
 * ============================================================================
 */
func generateSVG(dotFile, svgFile string) error {
	cmd := exec.Command("dot", "-Tsvg", dotFile, "-o", svgFile)
	return cmd.Run()
}