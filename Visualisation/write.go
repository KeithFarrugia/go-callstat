package visualisation

import (
	"fmt"
	"os"
)



func writeClusterNodeToDot(n *DotNode, f *os.File) {
    fmt.Fprintf(f, "    %q", n.ID)

    // Ensure map exists to prevent panics
    if n.Attrs == nil {
        n.Attrs = make(map[string]string)
    }

    // Only apply tooltip if missing
    if n.Attrs["tooltip"] == "" && n.Attrs["label"] != "" {
        n.Attrs["tooltip"] = n.Attrs["label"]
    }

    writeAttrs(f, n.Attrs)
    fmt.Fprintln(f, ";")
}