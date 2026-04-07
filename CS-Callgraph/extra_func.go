package cs_callgraph

// func handleInstruction(instr ssa.Instruction, callerNode *Node, g *Graph, visit func(*ssa.Function)) {
// 	switch i := instr.(type) {

// 	// --------------------------------------------------
// 	// Goroutines
// 	// --------------------------------------------------
// 	case *ssa.Go:
// 		call := i.Common()
// 		if callee := call.StaticCallee(); callee != nil {
// 			calleeNode := g.GenNode(callee)
// 			GenEdge(callerNode, i, calleeNode, GoEdge)
// 			visit(callee)
// 		}

// 	// --------------------------------------------------
// 	// Deferred calls
// 	// --------------------------------------------------
// 	case *ssa.Defer:
// 		call := i.Common()
// 		if callee := call.StaticCallee(); callee != nil {
// 			calleeNode := g.GenNode(callee)
// 			GenEdge(callerNode, i, calleeNode, DeferEdge)
// 			visit(callee)
// 		}

// 	// --------------------------------------------------
// 	// Calls
// 	// --------------------------------------------------
// 	case ssa.CallInstruction:
// 		call := i.Common()

// 		// Static call
// 		if callee := call.StaticCallee(); callee != nil {
// 			calleeNode := g.GenNode(callee)
// 			GenEdge(callerNode, i, calleeNode, CallEdge)
// 			visit(callee)
// 			return
// 		}

// 		// Dynamic call via function value
// 		if fnVal, ok := isFuncValue(call.Value); ok {
// 			calleeNode := g.GenNode(fnVal)
// 			GenEdge(callerNode, i, calleeNode, CallEdge)
// 		}

// 	// --------------------------------------------------
// 	// Function assignment (escaping function value)
// 	// --------------------------------------------------
// 	case *ssa.Store:
// 		fmt.Printf("Store Instruction \n\t %s\n", i.String())
// 		if fnVal, ok := isFuncValue(i.Val); ok {
// 			calleeNode := g.GenNode(fnVal)
// 			GenEdge(callerNode, i, calleeNode, AssignEdge)
// 		}

// 	// --------------------------------------------------
// 	// Panic
// 	// --------------------------------------------------
// 	case *ssa.Panic:
// 		GenEdge(callerNode, i, g.Root, PanicEdge)

// 	// --------------------------------------------------
// 	// Channel send (optional modeling)
// 	// --------------------------------------------------
// 	case *ssa.Send:
// 		GenEdge(callerNode, i, g.Root, SendEdge)

// 	// --------------------------------------------------
// 	// Other instructions can be ignored or handled later
// 	// --------------------------------------------------
// 	default:
// 		// no-op
// 	}
// }

// func analyseFunction(fn *ssa.Function) {
// 	for _, block := range fn.Blocks {
// 		for _, instr := range block.Instrs {

// 			switch i := instr.(type) {

// 			// Anything that can invoke code
// 			case ssa.CallInstruction:
// 				call := i.Common()
// 				if callee := call.StaticCallee(); callee != nil {
// 					fmt.Printf("    invoke -> %s\n", callee.String())
// 				} else {
// 					fmt.Printf("    invoke -> dynamic: %s\n", call.Value)
// 				}

// 			// Abnormal control flow
// 			case *ssa.Panic:
// 				fmt.Printf("    panic  -> %s\n", i.X)

// 			// (Optional) channel, goroutine, or other effects
// 			case *ssa.Send:
// 				fmt.Printf("    send   -> %s\n", i.Chan)

// 			case *ssa.Select:
// 				fmt.Printf("    select\n")

// 			case *ssa.Store:
// 				if fnVal, ok := isFuncValue(i.Val); ok {
// 					fmt.Printf(
// 						"    assign -> %s assigns function %s\n",
// 						fn.String(),
// 						fnVal.String(),
// 					)
// 				}
// 			}
// 		}
// 	}
// }
