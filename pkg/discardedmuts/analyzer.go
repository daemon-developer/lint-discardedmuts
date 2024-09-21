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
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			// Collect parameter names and their types
			paramNames := make(map[string]types.Type)
			for _, param := range fn.Type.Params.List {
				for _, ident := range param.Names {
					obj := pass.TypesInfo.ObjectOf(ident)
					paramNames[ident.Name] = obj.Type()
				}
			}

			// Traverse function body
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				// Case 1: Check for assignments to value parameters
				if assign, ok := n.(*ast.AssignStmt); ok {
					for _, lhs := range assign.Lhs {
						// Check if lhs is a SelectorExpr (e.g., ts.Name or c.checked)
						var ident *ast.Ident
						switch expr := lhs.(type) {
						case *ast.Ident:
							ident = expr
						case *ast.SelectorExpr:
							if baseIdent, ok := expr.X.(*ast.Ident); ok {
								ident = baseIdent
							}
						}

						if ident != nil {
							typ, exists := paramNames[ident.Name]
							if exists && !isPointer(typ) {
								pass.Reportf(ident.Pos(), "modification to '%s' is discarded because it is passed by value; pass by reference instead", ident.Name)
							}
						}
					}
				}

				// Case 2: Check for slice element modifications
				if rangeStmt, ok := n.(*ast.RangeStmt); ok {
					if ident, ok := rangeStmt.Value.(*ast.Ident); ok {
						typ := pass.TypesInfo.TypeOf(rangeStmt.X)
						if isSliceOrArray(typ) && ident.Name != "_" {
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
				// Case 3: Check for passing the address of a slice element or struct field
				if call, ok := n.(*ast.CallExpr); ok {
					// Attempt to retrieve the function name
					funName := "(unknown)"
					if fun, ok := call.Fun.(*ast.Ident); ok {
						funName = fun.Name
					} else if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						funName = sel.Sel.Name
					}
					for _, arg := range call.Args {
						if unary, ok := arg.(*ast.UnaryExpr); ok && unary.Op == token.AND {
							var ident *ast.Ident
							var identType types.Type

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

							if ident != nil && identType != nil {
								underlyingType := identType.Underlying()
								switch underlyingType := underlyingType.(type) {
								case *types.Slice, *types.Array:
									pass.Reportf(ident.Pos(), "passing address of slice element '%s' to '%s' will modify a copy; if you want to modify the original, pass a pointer to the element, e.g., '&%s[i]'", ident.Name, funName, ident.Name)
								case *types.Struct:
									pass.Reportf(ident.Pos(), "passing address of struct field '%s' to '%s' will modify a copy; if you want to modify the original, pass a pointer to the struct, e.g., '&%s'", ident.Name, funName, ident.Name)
								default:
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
