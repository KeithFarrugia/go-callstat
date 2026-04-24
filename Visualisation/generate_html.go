package visualisation

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	cs_callgraph "callstat/CS-Callgraph"
)

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
 *   cg          - the call graph to visualise
 *   dotDir      - directory for intermediate DOT files
 *   svgDir      - directory for intermediate SVG files
 *   htmlOut     - path of the HTML file to write
 *   concurrency - passed through; unused here (sequential is fine for I/O)
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
    for _, pkg := range pkgs {
        if maxDepth != -1 {
            if d, ok := depthMap[pkg]; !ok || d > maxDepth {
                continue
            }
        }

        san     := sanitizePkg(pkg)
        dotPath := filepath.Join(dotDir, san+".dot")
        svgPath := filepath.Join(svgDir, san+".svg")

        if err := graphs[pkg].WriteDOTToFile(dotPath); err != nil {
            log.Printf("[WARN] dot write %s: %v", pkg, err)
            continue
        }
        if err := generateSVG(dotPath, svgPath); err != nil {
            log.Printf("[WARN] svg gen  %s: %v", pkg, err)
        }
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
     *    (embedded directly in the <script> block)
     * ------------------------------------------------------- */
    svgJSON, err := json.Marshal(svgMap)
    if err != nil {
        return fmt.Errorf("marshal svg map: %w", err)
    }

    /* -------------------------------------------------------
     * 8. BUILD SIDEBAR ITEMS
     * ------------------------------------------------------- */
    var sb strings.Builder
    fmt.Fprintf(&sb, "\n")
    for _, pkg := range pkgs {
        if _, ok := svgMap[pkg]; !ok {
            continue
        }
        label   := html.EscapeString(shortPkgName(pkg))
        escaped := html.EscapeString(pkg)
        fmt.Fprintf(&sb,
            "       <div class=\"pkg-item\" data-pkg=\"%s\" title=\"%s\""+
                " onclick=\"switchPackage(this.dataset.pkg)\">%s</div>\n",
            escaped, escaped, label,
        )
    }

    /* -------------------------------------------------------
     * 9. RENDER & WRITE HTML
     * ------------------------------------------------------- */
    out := strings.NewReplacer(
        "{{SVG_DATA_JSON}}",  string(svgJSON),
        "{{PACKAGE_LIST}}",   sb.String(),
    ).Replace(htmlReportTemplate)

    return os.WriteFile(htmlOut, []byte(out), 0o644)
}

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
const htmlReportTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Callgraph Report</title>
<style>
/* ============================================================================ 
 * Baseline Styling
 * ============================================================================
 */

*{
    box-sizing    :   border-box;
    margin      :   0;
    padding     :   0
}
body{
    display     : flex;
    height      : 100vh;
    overflow    : hidden;
    font-family : 'Cascadia Mono',monospace;
    background  : #0d1117;
    color       : #c9d1d9
}

/* ============================================================================ 
 * Sidebar Styling
 * ============================================================================
 */
#sidebar{
    width       :   260px       ;   min-width       : 160px;
    display     :   flex        ;   flex-direction  : column;
    background  :   #161b22   ;   border-right    : 1px solid #30363d
}

#sidebar_header{
    padding         : 1rem 0.4rem 1rem 0.4rem;
    border-bottom   : 1px solid #30363d
}
#sidebar_title{
    font-size       :   1.2rem  ;    font-weight    : bold;
    margin-bottom   :   8px     ;    color          : #8db1ff
}

/* ---------------------------------------------------------------------------- 
 * Search Styling
 * ----------------------------------------------------------------------------
 */
#search{
    width           : 100%      ;   padding         : 0.2rem 0.4rem;
    background      : #0d1117 ;   border          : 0.1rem solid #30363d;
    color           : #c9d1d9 ;   border-radius   : 0.3rem;
    font-size       : 1.5rem    ;   font            : inherit
}

#search:focus{
    outline         : none;
    border-color    : #4c5770
}

/* ---------------------------------------------------------------------------- 
 * Package List Styling
 * ----------------------------------------------------------------------------
 */
#pkg-list{
    overflow-y      : auto;
    flex            : 1
}

.pkg-item{
    padding           : 0.2rem 0.4rem;
    cursor            : pointer;
    font-size         : 0.8rem;
    border-bottom     : 1px solid #21262d;
    white-space       : nowrap;
    overflow          : hidden;
    text-overflow     : ellipsis;
    color             : #8b949e
}
.pkg-item:hover{
    background      : #21262d;
    color           : #c9d1d9
}
.pkg-item.active{
    background      : #4c5770;
    color           : #fff
}


/* ============================================================================ 
 * Main Component Styling
 * ============================================================================
 */
#main{
    flex            : 1     ;   display     : flex;
    flex-direction  : column;   overflow    : hidden
}

#topbar{
    display       : flex      ; align-items   : center;
    gap           : 0.6rem    ; padding       : 8px 14px;
    min-height    : 44px      ; background    : #161b22;
    border-bottom : 1px solid #30363d
}

/* ---------------------------------------------------------------------------- 
 * Back & Forward Buttons Styling
 * ----------------------------------------------------------------------------
 */
#back_btn,#fwd_btn{
    padding       : 0.2rem 0.7rem;
    background    : #21262d;
    border        : 1px solid #30363d;
    color         : #c9d1d9;
    cursor        : pointer;
    border-radius : 0.2rem;
    font-family   : 'Cascadia Mono',monospace;
    font-size     : 0.6rem;
    height        : 100%
}

#back_btn:hover:not(:disabled),
#fwd_btn:hover :not(:disabled){
    background  : #30363d
}
#back_btn:disabled,#fwd_btn:disabled{
    opacity     : .4;
    cursor      : default
}

/* ---------------------------------------------------------------------------- 
 * Current Package Field Styling
 * ----------------------------------------------------------------------------
 */
#curr_package{
    flex          : 1         ; font-size     : 11px;
    color         : #8b949e ; overflow      : hidden;
    text-overflow : ellipsis  ; white-space   : nowrap
}

/* ---------------------------------------------------------------------------- 
 * Zoom Styling
 * ----------------------------------------------------------------------------
 */
#zoom-controls{
    display : flex;
    gap     : 4px
}
.zbtn{
    padding         : 2px 8px;
    background      : #21262d;
    border          : 1px solid #30363d;
    color           : #c9d1d9;
    cursor          : pointer;
    border-radius   : 4px;
    font-size       : 13px;
    line-height     : 1.4;
    font-family     : 'Cascadia Mono',monospace
}
  
.zbtn:hover{
    background      : #30363d
}

#hint{
    font-size       : 0.6rem;
    color           : #606872;
    white-space     : nowrap;
    font-family     : 'Cascadia Mono',monospace
}

/* ============================================================================ 
 * SVG Canvas Styling
 * ============================================================================
 */
#canvas{
    flex        :1          ; overflow  : hidden;
    position    :relative   ; cursor    : default; 
    background  : #0d1117
}

/*#canvas.grabbing{
    cursor:grabbing
}*/

#wrapper{
    position    : absolute;     top              : 0;
    left        : 0;            transform-origin : 0 0
}
#wrapper svg{
    display:block
}

/* Nodes that are navigation targets get a pointer cursor */
/* #wrapper a[href^="pkg://"]{cursor:pointer} */

#empty{
    position          : absolute  ;   inset           : 0;
    display           : flex      ;   align-items     : center;
    color             : #484f58 ;   font-size       : 1.2rem;
    justify-content   : center    ;   pointer-events  : none
}
</style>
</head>
<body>


<!-- 
 - ============================================================================
 - Side Bar Components
 - ============================================================================
/ -->
<div id="sidebar">
    <div id="sidebar_header">
        <div id="sidebar_title"> Packages</div>
            <input 
                id="search" 
                type="text" 
                placeholder="🔍︎"
                oninput="filterPkgs(this.value)"
            >
        </div>
    <div id="pkg-list">{{PACKAGE_LIST}}    </div>
</div>

<!-- 
 - ============================================================================
 - Main Components
 - ============================================================================
/ -->
<div id="main">
    <div id="topbar">
        <button id="back_btn" onclick="goBack()" disabled>⬅ Back</button>
        <button id="fwd_btn"  onclick="goFwd()"  disabled>Fwd ➡</button>
        <div id="curr_package">select a package from the sidebar</div>
            <div id="zoom-controls">
                <button class="zbtn" onclick="zoom(1.2)"   title="Zoom in"    >
                    +
                </button>
                <button class="zbtn" onclick="zoom(0.8)"   title="Zoom out"   >
                    -
                </button>
                <button class="zbtn" onclick="resetView()" title="Reset view" >
                    ⟳
                </button>
            </div>
            <div id="hint"> scroll=zoom | drag=pan | click ext-node=navigate </div>
        </div>
        <div id="canvas">
        <div id="empty">⤾ select a package from the sidebar</div>
        <div id="wrapper"></div>
    </div>
</div>

<script>

/* ============================================================================ 
 * Functions
 * ============================================================================
 */

const svgData = {{SVG_DATA_JSON}};

/* ============================================================================ 
 * Navigation
 * ============================================================================
 */

let     current     = null;
const   backStack   = [];
const   fwdStack    = [];

/* ============================================================================ 
 * Zoom State
 * ============================================================================
 */

let scale       = 1     , px = 0, py = 0;
let dragging    = false , lx = 0, ly = 0;

const canvas  = document.getElementById('canvas');
const wrapper = document.getElementById('wrapper');
const empty   = document.getElementById('empty');

/* ============================================================================ 
 * Transform
 * ============================================================================
 */

function applyTransform() {
    wrapper.style.transform = 'translate('+px+'px,'+py+'px) scale('+scale+')';
}

function resetView() {
    scale = 1; px = 24; py = 24;
    applyTransform();
}

/* ============================================================================ 
 * Zoom Towards Point
 * ============================================================================
 */
function zoom(factor, cx, cy) {
    const r = canvas.getBoundingClientRect();
    cx = (cx !== undefined) ? cx : r.width  / 2;
    cy = (cy !== undefined) ? cy : r.height / 2;
    px = cx - (cx - px) * factor;
    py = cy - (cy - py) * factor;
    scale *= factor;
    applyTransform();
}


/* ============================================================================ 
 * Mouse Events
 * ============================================================================
 */
canvas.addEventListener('wheel', function(e) {
    e.preventDefault();
    const r = canvas.getBoundingClientRect();
    zoom(e.deltaY < 0 ? 1.1 : 0.9, e.clientX - r.left, e.clientY - r.top);
}, { passive: false });

canvas.addEventListener('mousedown', function(e) {
    if (e.button !== 0) return;
    dragging = true; lx = e.clientX; ly = e.clientY;
    canvas.classList.add('grabbing');
});

window.addEventListener('mousemove', function(e) {
    if (!dragging) return;
    px += e.clientX - lx; py += e.clientY - ly;
    lx = e.clientX; ly = e.clientY;
    applyTransform();
});

window.addEventListener('mouseup', function() {
    dragging = false;
    canvas.classList.remove('grabbing');
});

/* ============================================================================ 
 * Navigation
 * ============================================================================
 */
function syncButtons() {
    document.getElementById('back_btn').disabled = backStack.length === 0;
    document.getElementById('fwd_btn').disabled  = fwdStack.length  === 0;
}

function switchPackage(pkg, pushBack) {
    if (pushBack === undefined) pushBack = true;
    if (!(pkg in svgData)) { console.warn('no SVG for', pkg); return; }

    if (current !== null && pushBack) {
        backStack.push(current);
        fwdStack.length = 0;
    }

    current = pkg;
    empty.style.display = 'none';
    wrapper.innerHTML   = svgData[pkg];
    resetView();
    // wireClicks(); <-- remove this

    document.querySelectorAll('.pkg-item').forEach(function(el) {
        el.classList.toggle('active', el.dataset.pkg === pkg);
    });

    document.getElementById('curr_package').textContent = pkg;
    syncButtons();
}

function goBack() {
    if (backStack.length === 0) return;
    fwdStack.push(current);
    switchPackage(backStack.pop(), false);
}

function goFwd() {
    if (fwdStack.length === 0) return;
    backStack.push(current);
    switchPackage(fwdStack.pop(), false);
}


/* ============================================================================ 
 * Wire pkg:// links inside the SVG
 * ============================================================================
 */
wrapper.addEventListener('click', function(e) {
    const a = e.target.closest('a[*|href^="pkg://"]');
    if (!a) return;
    e.preventDefault();
    e.stopPropagation();
    const href = a.getAttribute('href') || a.getAttributeNS('http://www.w3.org/1999/xlink', 'href');
    switchPackage(href.slice(6));
});

/* ============================================================================ 
 * Sidebar filter
 * ============================================================================
 */
function filterPkgs(q) {
    q = q.toLowerCase();
        document.querySelectorAll('.pkg-item').forEach(function(el) {
        el.style.display = el.dataset.pkg.toLowerCase().includes(q) ? '' : 'none';
    });
}

/* ============================================================================ 
 * Keyboard shortcuts
 * ============================================================================
 */
window.addEventListener('keydown', function(e) {
    if (e.altKey && e.key === 'ArrowLeft')  { e.preventDefault(); goBack(); }
    if (e.altKey && e.key === 'ArrowRight') { e.preventDefault(); goFwd();  }
});

/* ============================================================================ 
 * Boot: show first package
 * ============================================================================
 */
var first = document.querySelector('.pkg-item');
if (first) switchPackage(first.dataset.pkg, false);
</script>
</body>
</html>`