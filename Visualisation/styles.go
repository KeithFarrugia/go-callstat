package visualisation

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
)

//
// ─────────────────────────────────────────────────────────────
// CONFIG STRUCTS (MAP-OF-MAPS)
// ─────────────────────────────────────────────────────────────
//

type NodeStyle string
type EdgeStyle string

const(
    ns_normal 	NodeStyle 	= "normal"
    ns_anon		NodeStyle 	= "anonymous"
    ns_external	NodeStyle 	= "external"
    
    es_call		EdgeStyle	= "call"
    es_go		EdgeStyle 	= "go"
    es_defer   	EdgeStyle 	= "defer"
    es_panic	EdgeStyle 	= "panic"
    es_default	EdgeStyle 	= "default"
)

type StyleConfig struct {
    NodeStyles map[string]map[string]string `json:"nodeStyles"`
    EdgeStyles map[string]map[string]string `json:"edgeStyles"`
    Cluster    map[string]string            `json:"cluster"`
}

var global_styles *StyleConfig
//go:embed format.json
var defaultStyleJSON []byte
/* ============================================================================
 * validation
 * ----------------------------------------------------------------------------
 * Since we are using maps, we validate that the "must-have" styles exist
 * in the JSON so the program doesn't produce broken DOT files.
 * ============================================================================
 */

func validate() error {
    if global_styles == nil {
        return fmt.Errorf("styles not loaded")
    }

    // Ensure the basic expected node categories exist
    requiredNodes := []string{"normal", "external"}
    for _, req := range requiredNodes {
        if _, ok := global_styles.NodeStyles[req]; !ok {
            return fmt.Errorf("missing required node style: %s", req)
        }
    }

    // Ensure basic edge types exist
    requiredEdges := []string{"call"}
    for _, req := range requiredEdges {
        if _, ok := global_styles.EdgeStyles[req]; !ok {
            return fmt.Errorf("missing required edge style: %s", req)
        }
    }

    return nil
}

/* ============================================================================
 * LoadStyles
 * ----------------------------------------------------------------------------
 * Loads the JSON configuration from the provided path into the global
 * style pointer and runs validation.
 * ============================================================================
 */
func LoadInternalStyles() error {
    var cfg StyleConfig
    if err := json.Unmarshal(defaultStyleJSON, &cfg); err != nil {
        return fmt.Errorf("failed to parse embedded JSON: %w", err)
    }

    global_styles = &cfg
    return validate()
}

func LoadStyles(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("could not open config file at %s: %w", path, err)
    }
    defer f.Close()

    var cfg StyleConfig
    if err := json.NewDecoder(f).Decode(&cfg); err != nil {
        return fmt.Errorf("failed to decode JSON: %w", err)
    }
    
    global_styles = &cfg

    // Run validation
    if err := validate(); err != nil {
        return err
    }

    // If we get here, everything is perfect
    fmt.Printf("Successfully loaded styles from: %s\n", path)
    return nil
}