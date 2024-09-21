package discardedmuts

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var DiscardedModificationAnalyzer = &analysis.Analyzer{
	Name: "discardedmod",
	Doc:  "reports modifications to function parameters that are passed by value or indirectly mutated via pointers to local copies",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Iterate over each file in the package being analyzed
	for _, file := range pass.Files {
		// Inspect each node (AST element) in the file
		ast.Inspect(file, func(n ast.Node) bool {
			// Check if the current node is a function declaration
			fn, ok := n.(*ast.FuncDecl)
			if !ok {
				// If not a function, continue traversing the AST
				return true
			}

			// Collect parameter names and their types in a map
			// This allows us to check if a variable is a function parameter and its type later
			// Example: for a function func foo(a int, b string), paramNames would be {"a": int, "b": string}
			paramNames := make(map[string]types.Type)
			for _, param := range fn.Type.Params.List {
				for _, ident := range param.Names {
					obj := pass.TypesInfo.ObjectOf(ident)
					paramNames[ident.Name] = obj.Type()
				}
			}

			// Traverse the function body to look for specific cases
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				// Case 1: Check for assignments to value parameters
				// Example: func foo(a int) { a = 10 }
				// Here, 'a' is passed by value, so modifying it has no effect outside the function
				if assign, ok := n.(*ast.AssignStmt); ok {
					// Iterate over the left-hand side (LHS) of the assignment
					for _, lhs := range assign.Lhs {
						var ident *ast.Ident
						// Check if the LHS is an identifier (e.g., 'a') or a selector expression (e.g., 'ts.Name')
						switch expr := lhs.(type) {
						case *ast.Ident:
							ident = expr
						case *ast.SelectorExpr:
							// If it's a selector, get the base identifier (e.g., 'ts' in 'ts.Name')
							if baseIdent, ok := expr.X.(*ast.Ident); ok {
								ident = baseIdent
							}
						}

						// If we found an identifier and it's a function parameter that's not a pointer
						// then report a warning because modifying it has no effect outside the function
						if ident != nil {
							typ, exists := paramNames[ident.Name]
							if exists && !isPointer(typ) {
								pass.Reportf(ident.Pos(), "modification to '%s' is discarded because it is passed by value; pass by reference instead", ident.Name)
							}
						}
					}
				}

				// Case 2: Check for modifications to slice elements in a range loop
				// Example: func foo(s []int) { for _, v := range s { v = 10 } }
				// Here, 'v' is a copy of each element, so modifying it has no effect on the original slice 's'
				if rangeStmt, ok := n.(*ast.RangeStmt); ok {
					if ident, ok := rangeStmt.Value.(*ast.Ident); ok {
						typ := pass.TypesInfo.TypeOf(rangeStmt.X)
						// If ranging over a slice or array and the value variable is not '_'
						if isSliceOrArray(typ) && ident.Name != "_" {
							// Inspect the range loop body for assignments to the value variable
							ast.Inspect(rangeStmt.Body, func(n ast.Node) bool {
								if assign, ok := n.(*ast.AssignStmt); ok {
									for _, lhs := range assign.Lhs {
										var lhsIdent *ast.Ident
										switch expr := lhs.(type) {
										case *ast.Ident:
											lhsIdent = expr
										case *ast.SelectorExpr:
											if baseIdent, ok := expr.X.(*ast.Ident); ok {
												lhsIdent = baseIdent
											}
										}

										// If assigning to the range value variable, report a warning
										// because modifying the copy has no effect on the original slice
										if lhsIdent != nil && lhsIdent.Name == ident.Name {
											sliceExpr := types.ExprString(rangeStmt.X)
											pass.Reportf(lhsIdent.Pos(), "modification to slice element '%s' is discarded; use index to modify directly, e.g., '&&%s[i] = ...'", ident.Name, sliceExpr)
										}
									}
								}
								return true
							})
						}
					}
				}

				// Case 3: Check for passing the address of a slice element, array element, or struct field to a function
				// Example: func foo(s []int) { bar(&s[0]) }
				// Here, '&s[0]' passes a pointer to a copy of the first element, not a pointer to the original element
				if call, ok := n.(*ast.CallExpr); ok {
					// Attempt to get the called function name for a clearer warning message
					funName := "(unknown)"
					if fun, ok := call.Fun.(*ast.Ident); ok {
						funName = fun.Name
					} else if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						funName = sel.Sel.Name
					}

					// Iterate over the function arguments
					for _, arg := range call.Args {
						// Check if the argument is taking the address of something ('&' unary expression)
						if unary, ok := arg.(*ast.UnaryExpr); ok && unary.Op == token.AND {
							var ident *ast.Ident
							var identType types.Type

							// Check if we're taking the address of an identifier or a selector expression
							switch expr := unary.X.(type) {
							case *ast.Ident:
								ident = expr
								identType = pass.TypesInfo.TypeOf(ident)
							case *ast.SelectorExpr:
								if baseIdent, ok := expr.X.(*ast.Ident); ok {
									ident = baseIdent
									identType = pass.TypesInfo.TypeOf(baseIdent)
								}
							}

							// If we found an identifier and could determine its type
							if ident != nil && identType != nil {
								underlyingType := identType.Underlying()
								switch underlyingType := underlyingType.(type) {
								case *types.Slice, *types.Array:
									// Report a warning for taking the address of a slice/array element
									// because it passes a pointer to a copy, not the original element
									pass.Reportf(ident.Pos(), "passing address of slice element '%s' to '%s' will modify a copy; if you want to modify the original, pass a pointer to the element, e.g., '&%s[i]'", ident.Name, funName, ident.Name)
								case *types.Struct:
									// Report a warning for taking the address of a struct field
									// because it passes a pointer to a copy of the struct, not the original
									pass.Reportf(ident.Pos(), "passing address of struct field '%s' to '%s' will modify a copy; if you want to modify the original, pass a pointer to the struct, e.g., '&%s'", ident.Name, funName, ident.Name)
								default:
									// For any other type, also report a warning about passing a copy
									pass.Reportf(ident.Pos(), "passing address of '%s' (type %s) to '%s' is passing a copy; if you want to modify the original, pass a pointer to it, e.g., '&%s[i]'", ident.Name, types.TypeString(underlyingType, nil), funName, ident.Name)
								}
							}
						}
					}
				}

				return true
			})

			return true
		})
	}
	return nil, nil
}

// Helper functions

func isPointer(typ types.Type) bool {
	_, ok := typ.(*types.Pointer)
	return ok
}

func isSliceOrArray(typ types.Type) bool {
	switch typ.(type) {
	case *types.Slice, *types.Array:
		return true
	default:
		return false
	}
}

func getElementType(typ types.Type) types.Type {
	switch t := typ.(type) {
	case *types.Slice:
		return t.Elem()
	case *types.Array:
		return t.Elem()
	default:
		return nil
	}
}
