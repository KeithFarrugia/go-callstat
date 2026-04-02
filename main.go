package main

import (
	"log"
	"os"
	"os/exec"
	"strings"

	cs_callgraph "callstat/CS-Callgraph"
	visualisation "callstat/Visualisation"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func ensureFolders() (dotDir, svgDir string) {
	dotDir = "./output/dotfiles"
	svgDir = "./output/svgs"

	if err := os.MkdirAll(dotDir, os.ModePerm); err != nil {
		log.Fatalf("failed to create dot folder: %v", err)
	}
	if err := os.MkdirAll(svgDir, os.ModePerm); err != nil {
		log.Fatalf("failed to create svg folder: %v", err)
	}
	return
}

// generateSVG calls Graphviz dot CLI to produce an SVG from a DOT file
func generateSVG(dotFile, svgFile string) error {
	cmd := exec.Command("dot", "-Tsvg", dotFile, "-o", svgFile)
	return cmd.Run()
}

func main() {// Get the path to the executable
    if err := visualisation.LoadInternalStyles(); err != nil {
        log.Fatalf("Failed to initialize styles: %v", err)
    }
	err := visualisation.LoadStyles("Visualisation/format.json");

	dotDir, svgDir := ensureFolders()

	// --- Load Go packages ---
	cfg := &packages.Config{
		Mode: packages.LoadAllSyntax,
		Dir:  "../dep-usage-test/", // path to your project
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		log.Fatal(err)
	}

	// --- Build SSA program ---
	prog, _ := ssautil.AllPackages(pkgs, ssa.BuilderMode(0))
	prog.Build()

	// --- Build call graph ---
	cg := cs_callgraph.BuildExtendedCallGraph(prog)

	for pkg, dg := range visualisation.BuildDotGraphPerPackage(cg) {
		// sanitize pkg path for filesystem
		pkgName := strings.ReplaceAll(pkg, "/", "_")
		dotPath := dotDir + "/" + pkgName + ".dot"
		svgPath := svgDir + "/" + pkgName + ".svg"

		// Write DOT file
		if err := dg.WriteDOTToFile(dotPath); err != nil {
			log.Printf("failed to write %s: %v", dotPath, err)
			continue
		}

		// Generate SVG using Graphviz
		if err := generateSVG(dotPath, svgPath); err != nil {
			log.Printf("failed to generate svg for %s: %v", dotPath, err)
			continue
		}

		log.Printf("Generated DOT + SVG for package: %s", pkg)
	}
}
