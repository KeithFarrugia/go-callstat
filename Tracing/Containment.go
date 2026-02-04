package tracing

import "go/types"

func containsChanOfFunc(t types.Type, visited map[types.Type]bool) bool {
	if t == nil {
		return false
	}

	// Prevent infinite recursion on recursive types
	if visited[t] {
		return false
	}
	visited[t] = true

	switch tt := t.(type) {

	case *types.Chan:
		_, ok := tt.Elem().(*types.Signature)
		return ok

	case *types.Pointer:
		return containsChanOfFunc(tt.Elem(), visited)

	case *types.Slice:
		return containsChanOfFunc(tt.Elem(), visited)

	case *types.Array:
		return containsChanOfFunc(tt.Elem(), visited)

	case *types.Struct:
		for i := 0; i < tt.NumFields(); i++ {
			if containsChanOfFunc(tt.Field(i).Type(), visited) {
				return true
			}
		}

	case *types.Named:
		return containsChanOfFunc(tt.Underlying(), visited)

	case *types.Tuple:
		for i := 0; i < tt.Len(); i++ {
			if containsChanOfFunc(tt.At(i).Type(), visited) {
				return true
			}
		}

	case *types.Signature:
		// A function returning a function does NOT count
		// Only channels of functions matter
		return false
	}

	return false
}
func containsFunc(t types.Type, visited map[types.Type]bool) bool {
	if t == nil {
		return false
	}

	// Prevent infinite recursion for recursive types
	if visited[t] {
		return false
	}
	visited[t] = true

	switch tt := t.(type) {

	case *types.Signature:
		// This is a function itself → true
		return true

	case *types.Pointer:
		return containsFunc(tt.Elem(), visited)

	case *types.Slice:
		return containsFunc(tt.Elem(), visited)

	case *types.Array:
		return containsFunc(tt.Elem(), visited)

	case *types.Struct:
		for i := 0; i < tt.NumFields(); i++ {
			if containsFunc(tt.Field(i).Type(), visited) {
				return true
			}
		}

	case *types.Named:
		return containsFunc(tt.Underlying(), visited)

	case *types.Tuple:
		for i := 0; i < tt.Len(); i++ {
			if containsFunc(tt.At(i).Type(), visited) {
				return true
			}
		}

	case *types.Chan:
		return containsFunc(tt.Elem(), visited)

	case *types.Map:
		// Optional: if you want to check maps as well
		return containsFunc(tt.Key(), visited) || containsFunc(tt.Elem(), visited)
	}

	return false
}
