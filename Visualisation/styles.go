package visualisation

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
)

/* ============================================================================
 * CONFIG TYPES
 * ============================================================================
 */

type NodeStyle string
type EdgeStyle string

const (
	ns_normal   NodeStyle = "normal"
	ns_anon     NodeStyle = "anonymous"
	ns_external NodeStyle = "external"

	es_call    EdgeStyle = "call"
	es_go      EdgeStyle = "go"
	es_defer   EdgeStyle = "defer"
	es_panic   EdgeStyle = "panic"
	es_default EdgeStyle = "default"
)

/* ============================================================================
 * StyleConfig
 * ----------------------------------------------------------------------------
 * Represents the JSON structure used to configure visual styles.
 *
 * Structure:
 *   - NodeStyles : map[nodeType] -> map[attr]value
 *   - EdgeStyles : map[edgeType] -> map[attr]value
 *   - Cluster    : shared attributes for clusters
 * ============================================================================
 */
type StyleConfig struct {
	NodeStyles map[string]map[string]string `json:"nodeStyles"`
	EdgeStyles map[string]map[string]string `json:"edgeStyles"`
	Cluster    map[string]string            `json:"cluster"`
}

/* ============================================================================
 * Global State
 * ----------------------------------------------------------------------------
 * Holds the currently loaded style configuration.
 * Must be initialized before any graph building occurs.
 * ============================================================================
 */
var global_styles *StyleConfig

//go:embed format.json
var defaultStyleJSON []byte

/* ============================================================================
 * validate
 * ----------------------------------------------------------------------------
 * Ensures that required style categories exist in the configuration.
 * Prevents invalid or incomplete DOT output.
 *
 * Required:
 *   - NodeStyles: "normal", "external"
 *   - EdgeStyles: "call"
 * ============================================================================
 */
func validate() error {

	if global_styles == nil {
		return fmt.Errorf("styles not loaded")
	}

	/* -------------------------------------------------------
	 * Required Node Styles
	 * ------------------------------------------------------- */
	for _, key := range []string{"normal", "external"} {
		if _, ok := global_styles.NodeStyles[key]; !ok {
			return fmt.Errorf("missing required node style: %s", key)
		}
	}

	/* -------------------------------------------------------
	 * Required Edge Styles
	 * ------------------------------------------------------- */
	for _, key := range []string{"call"} {
		if _, ok := global_styles.EdgeStyles[key]; !ok {
			return fmt.Errorf("missing required edge style: %s", key)
		}
	}

	return nil
}

/* ============================================================================
 * LoadInternalStyles
 * ----------------------------------------------------------------------------
 * Loads the embedded default JSON configuration into global_styles.
 * This should typically be called at startup.
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

/* ============================================================================
 * LoadStyles
 * ----------------------------------------------------------------------------
 * Loads a JSON style configuration from disk and replaces global_styles.
 *
 * Behaviour:
 *   1. Opens file from given path
 *   2. Decodes JSON into StyleConfig
 *   3. Validates required fields
 *   4. Sets global_styles if valid
 * ============================================================================
 */
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

	/* -------------------------------------------------------
	 * Validation
	 * ------------------------------------------------------- */
	if err := validate(); err != nil {
		return err
	}

	fmt.Printf("Successfully loaded styles from: %s\n", path)
	return nil
}