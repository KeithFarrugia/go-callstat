package main

import (
    "fmt"
    "strings"

    "golang.org/x/tools/go/ssa"
    "golang.org/x/tools/go/ssa/ssautil"
)
// ------------------ Data Structures ------------------
// sendSite stores a sender function and the function it sends
type sendSite struct {
    fn  *ssa.Function // The function performing the send
    val *ssa.Function // The function being sent
}

// recvSite stores a receiver function and the receive instruction
type recvSite struct {
    fn   *ssa.Function // The function performing the receive
    recv ssa.Value     // The SSA value representing the receive instruction
}


// analyseFuncChannelsCHA analyses channels that carry function values in a Go program
// and prints which receiver functions may execute which functions, in a CHA-style approach.
func analyseFuncChannelsCHA(prog *ssa.Program, target string) {

    // ================= High-Level Overview =================
    // This analysis inspects all SSA functions in the target package.
    // It searches for two things:
    // 1. "Send" instructions sending functions over channels
    // 2. "Receive" instructions reading functions from channels
    //
    // Once all sends and receives are collected, we match them conceptually:
    // every receiver function may execute every function sent on the channel
    // (similar to a CHA call graph: it over-approximates potential flows).
    // =======================================================

    
    var sends []sendSite // All collected sends
    var recvs []recvSite // All collected receives

    fmt.Println("================ Channel Function Links (CHA-style) ================\n")

    // ================= Step 1: Collect all sends and receives =================
    // We iterate over all SSA functions in the program and inspect each instruction.
    // If a send instruction carries a function or closure, we record it.
    // If a receive instruction reads from a channel, we record it.
    // This step builds the "potential edges" between senders and receivers.
    // =========================================================================
    for fn := range ssautil.AllFunctions(prog) {
        if fn == nil || fn.Pkg == nil {
            continue
        }
        // Only inspect functions in the target package
        if !strings.Contains(fn.Pkg.Pkg.Path(), target) {
            continue
        }

        // Iterate over basic blocks and their instructions
        for _, block := range fn.Blocks {
            for _, instr := range block.Instrs {

                // ---------------- Collect Sends ----------------
                if s, ok := instr.(*ssa.Send); ok {
                    // Check if we are sending a function value
                    if f, ok := s.X.(*ssa.Function); ok {
                        sends = append(sends, sendSite{fn: fn, val: f})
                        fmt.Printf("[debug] SEND in %s of func %s\n", fn.Name(), f.Name())
                    } else if cl, ok := s.X.(*ssa.MakeClosure); ok {
                        // If sending a closure, track the underlying function
                        if f, ok := cl.Fn.(*ssa.Function); ok {
                            sends = append(sends, sendSite{fn: fn, val: f})
                            fmt.Printf("[debug] SEND in %s of closure %s\n", fn.Name(), f.Name())
                        }
                    }
                }

                // ---------------- Collect Receives ----------------
                if u, ok := instr.(*ssa.UnOp); ok && u.Op.String() == "<-" {
                    // This is a receive operation from a channel
                    recvs = append(recvs, recvSite{fn: fn, recv: u})
                    fmt.Printf("[debug] RECV in %s of func channel\n", fn.Name())
                }
            }
        }
    }

    // Early exit if there are no function-valued channel operations
    if len(sends) == 0 || len(recvs) == 0 {
        fmt.Println("(no function-valued channel links found)")
        return
    }

    fmt.Println("[debug] Matching sends and receives...")

    // ================= Step 2: Group by receiver =================
    // For each receiver function, we create a list of all functions it may
    // execute. We over-approximate by connecting each receiver to every
    // function that is sent on any channel.
    // ===============================================================
    recvMap := make(map[*ssa.Function][]*ssa.Function)
    for _, r := range recvs {
        for _, s := range sends {
            // Append the sender function to the receiver's potential executions
            recvMap[r.fn] = append(recvMap[r.fn], s.val)
        }
    }

    // ================= Step 3: Print results =================
    // For each receiver function, list all functions it may execute
    // due to receiving them via channels.
    // ===========================================================
    for recvFn, funcs := range recvMap {
        fmt.Printf("%s may execute:\n", recvFn.Name())
        for _, f := range funcs {
            fmt.Printf("  - %s\n", f.Name())
        }
    }
    fmt.Println()
}
