package visualisation

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cs_callgraph "callstat/CS-Callgraph"
)

/* ============================================================================
 * htmlReportTemplate
 * ----------------------------------------------------------------------------
 * Self-contained HTML template.  Two placeholders are substituted at runtime:
 *
 *   {{SVG_DATA_JSON}}  JSON object mapping package path → bare SVG string
 *   {{PACKAGE_LIST}}   HTML <div> elements for the sidebar
 *
 * Navigation works like a browser:
 *   - Sidebar click           → switchPackage (clears fwd stack)
 *   - Click ext-package node  → switchPackage (same)
 *   - ← Back / Alt+←         → goBack
 *   - Fwd → / Alt+→           → goFwd
 *   - Scroll                  → zoom toward cursor
 *   - Drag                    → pan
 *   - ⌂ button               → reset view
 * ============================================================================
 */
//go:embed report_template.html
var htmlReportTemplate string

/* ============================================================================
 * stripSVGPreamble
 * ----------------------------------------------------------------------------
 * Removes the XML declaration and DOCTYPE so the bare <svg> element can be
 * safely embedded inline inside an HTML document.
 * ============================================================================
 */
func stripSVGPreamble(raw string) string {
    if idx := strings.Index(raw, "<svg"); idx >= 0 {
        return raw[idx:]
    }
    return raw
}

/* ============================================================================
 * sanitizePkg
 * ----------------------------------------------------------------------------
 * Replaces path separators with underscores to produce a safe filename stem.
 * ============================================================================
 */
func sanitizePkg(pkg string) string {
    return strings.ReplaceAll(pkg, "/", "_")
}

/* ============================================================================
 * escapeJSTemplateLiteral
 * ----------------------------------------------------------------------------
 * Escapes a JSON string so it can be safely embedded inside a JS backtick
 * template literal: escapes backslashes, backticks, and ${.
 * ============================================================================
 */
func escapeJSTemplateLiteral(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "${", "\\${")
	return s
}

/* ============================================================================
 * pkgGroup
 * ----------------------------------------------------------------------------
 * Classifies a package path into one of three sidebar groups:
 *
 *   "internal"  - belongs to the project (path has projectRoot as prefix)
 *   "stdlib"    - Go standard library (first path segment contains no dot)
 *   "external"  - third-party module (everything else)
 * ============================================================================
 */
func pkgGroup(path, projectRoot string) string {
	if projectRoot != "" && strings.HasPrefix(path, projectRoot) {
		return "internal"
	}
	if cs_callgraph.IsStdlib(path) {
        return "stdlib"
    }
	return "external"
}
/* ============================================================================
 * buildSidebarHTML
 * ----------------------------------------------------------------------------
 * Renders the sidebar package list as three collapsible <details> groups.
 * Groups that are empty are omitted entirely.
 * ============================================================================
 */
func buildSidebarHTML(pkgs []string, svgMap map[string]string, projectRoot string) string {
	type group struct {
		key    string
		label  string
		open   bool
	}
	groups := []group{
		{"internal" , "Project"         , true},
		{"stdlib"   , "Standard Library", false},
		{"external" , "External"        , false},
	}
 
	// Bucket pkgs into groups preserving sorted order.
	buckets := map[string][]string{}
	for _, pkg := range pkgs {
		if _, ok := svgMap[pkg]; !ok {
			continue
		}
		buckets[pkgGroup(pkg, projectRoot)] = append(buckets[pkgGroup(pkg, projectRoot)], pkg)
	}
 
	var sb strings.Builder
	for _, g := range groups {
		items := buckets[g.key]
		if len(items) == 0 {
			continue
		}
		openAttr := ""
		if g.open {
			openAttr = " open"
		}
		fmt.Fprintf(&sb, "<details class=\"pkg-group\"%s>\n", openAttr)
		fmt.Fprintf(&sb, "  <summary class=\"pkg-group-summary\">%s <span class=\"pkg-group-count\">%d</span></summary>\n",
			html.EscapeString(g.label), len(items))
		for _, pkg := range items {
			label   := html.EscapeString(shortPkgName(pkg))
			escaped := html.EscapeString(pkg)
			fmt.Fprintf(&sb,
				"  <div class=\"pkg-item\" data-pkg=\"%s\" title=\"%s\""+
					" onclick=\"switchPackage(this.dataset.pkg)\">%s</div>\n",
				escaped, escaped, label,
			)
		}
		fmt.Fprintf(&sb, "</details>\n")
	}
	return sb.String()
}

/* ============================================================================
 * GenerateHTMLReport
 * ----------------------------------------------------------------------------
 * Builds DOT + SVG artefacts per package, then writes a single self-contained
 * HTML file with:
 *   - A filterable package sidebar
 *   - A pan/zoomable SVG canvas
 *   - Back / Forward navigation
 *   - Click-through from external-package nodes to their own graph
 *     (works because buildLinkClusterNode sets URL="pkg://…" on external
 *     nodes, which graphviz renders as <a href="pkg://…"> in the SVG)
 *
 * Parameters:
 *   cg             - the call graph to visualise
 *   dotDir         - directory for intermediate DOT files
 *   svgDir         - directory for intermediate SVG files
 *   htmlOut        - path of the HTML file to write
 *   concurrency    - passed through; unused here (sequential is fine for I/O)
 *   projectRoot    - module path prefix used to identify internal packages
 *                   (e.g. "github.com/you/yourrepo"); pass "" to skip grouping

 * ============================================================================
 */
func GenerateHTMLReport(
    cg          *cs_callgraph.Graph,
    dotDir      string,
    svgDir      string,
    htmlOut     string,
    concurrency int,
    skipPkg     map[string]struct{},
    depthMap    map[string]int,
    maxDepth    int,
	statsJSONPath string,
	projectRoot   string,

) error {

    /* -------------------------------------------------------
     * 1. STYLES
     * ------------------------------------------------------- */
    if global_styles == nil {
        if err := LoadInternalStyles(); err != nil {
            return fmt.Errorf("load styles: %w", err)
        }
    }

    /* -------------------------------------------------------
     * 2. BUILD GRAPHS
     * ------------------------------------------------------- */
    graphs := BuildDotGraphPerPackage(cg, skipPkg)

    /* -------------------------------------------------------
     * 3. ENSURE OUTPUT DIRECTORIES
     * ------------------------------------------------------- */
    for _, dir := range []string{dotDir, svgDir} {
        if err := os.MkdirAll(dir, os.ModePerm); err != nil {
            return fmt.Errorf("mkdir %s: %w", dir, err)
        }
    }

    /* -------------------------------------------------------
     * 4. SORTED PACKAGE LIST (deterministic output)
     * ------------------------------------------------------- */
    pkgs := make([]string, 0, len(graphs))
    for pkg := range graphs {
        pkgs = append(pkgs, pkg)
    }
    sort.Strings(pkgs)

    /* -------------------------------------------------------
     * 5. GENERATE DOT + SVG FILES
     * ------------------------------------------------------- */
    fmt.Printf("%-60s | %-12s | %-12s\n", "Package", "DOT Gen", "SVG Gen")
    fmt.Println(strings.Repeat("-", 90))

    for _, pkg := range pkgs {
        if _, skip := skipPkg[pkg]; skip {
            continue
        }
        if maxDepth != -1 {
            if d, ok := depthMap[pkg]; !ok || d > maxDepth {
                continue
            }
        }

        san     := sanitizePkg(pkg)
        dotPath := filepath.Join(dotDir, san+".dot")
        svgPath := filepath.Join(svgDir, san+".svg")

        // Timer for DOT generation (Writing the file)
        tDotStart := time.Now()
        if err := graphs[pkg].WriteDOTToFile(dotPath); err != nil {
            log.Printf("[WARN] dot write %s: %v", pkg, err)
            continue
        }
        dotElapsed := time.Since(tDotStart)

        // Timer for SVG generation (Calling the external 'dot' command)
        tSvgStart := time.Now()
        if err := generateSVG(dotPath, svgPath); err != nil {
            log.Printf("[WARN] svg gen  %s: %v", pkg, err)
        }
        svgElapsed := time.Since(tSvgStart)

        // Print results for this package
        fmt.Printf("%-60s | %-12v | %-12v\n", 
            shortPkgName(pkg), 
            dotElapsed.Round(time.Millisecond), 
            svgElapsed.Round(time.Millisecond),
        )
    }

    /* -------------------------------------------------------
     * 6. READ SVGs INTO MEMORY
     * ------------------------------------------------------- */
    svgMap := make(map[string]string, len(pkgs))
    for _, pkg := range pkgs {
        if maxDepth != -1 {
            if d, ok := depthMap[pkg]; !ok || d > maxDepth {
                continue
            }
        }

        san := sanitizePkg(pkg)
        raw, err := os.ReadFile(filepath.Join(svgDir, san+".svg"))
        if err != nil {
            log.Printf("[WARN] read svg %s: %v", pkg, err)
            continue
        }
        svgMap[pkg] = stripSVGPreamble(string(raw))
    }

    /* -------------------------------------------------------
     * 7. MARSHAL SVG MAP → JSON
     * ------------------------------------------------------- */
    svgBytes, err := json.Marshal(svgMap)
    if err != nil {
        return fmt.Errorf("marshal svg map: %w", err)
    }

	svgJSONStr := escapeJSTemplateLiteral(string(svgBytes))


    /* -------------------------------------------------------
	 * 8. READ STATS JSON
	 * ------------------------------------------------------- */
	statsJSONStr := "null"
	if statsJSONPath != "" {
		raw, err := os.ReadFile(statsJSONPath)
		if err != nil {
			log.Printf("[WARN] read stats json %s: %v", statsJSONPath, err)
		} else {
			// Validate it is well-formed JSON before embedding.
			var probe json.RawMessage
			if err := json.Unmarshal(raw, &probe); err != nil {
				log.Printf("[WARN] stats json malformed %s: %v", statsJSONPath, err)
			} else {
				statsJSONStr = escapeJSTemplateLiteral(string(raw))
			}
		}
	}

    
	/* -------------------------------------------------------
	 * 9. BUILD SIDEBAR ITEMS
	 * ------------------------------------------------------- */
	sidebarHTML := buildSidebarHTML(pkgs, svgMap, projectRoot)


    /* -------------------------------------------------------
     * 10. RENDER & WRITE HTML
     * ------------------------------------------------------- */
    out := strings.NewReplacer(
        "{{SVG_DATA_JSON}}", svgJSONStr, // Use our escaped string here
        "{{STATS_DATA_JSON}}", statsJSONStr,
		"{{PACKAGE_LIST}}",   sidebarHTML,
    ).Replace(htmlReportTemplate)

    return os.WriteFile(htmlOut, []byte(out), 0o644)
}


