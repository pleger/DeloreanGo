package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"testing"

	"timepointlib/internal/testx"
)

type fakeNode struct {
	start token.Pos
	end   token.Pos
}

func (n *fakeNode) Pos() token.Pos { return n.start }
func (n *fakeNode) End() token.Pos { return n.end }

func TestIsTimepointImportPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{path: "timepoint", want: true},
		{path: "timepointlib/timepoint", want: true},
		{path: "example.com/x/timepoint", want: true},
		{path: "example.com/x/not-timepoint", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			testx.Equal(t, tc.want, isTimepointImportPath(tc.path))
		})
	}
}

func TestPathBase(t *testing.T) {
	testx.Equal(t, "c", pathBase("a/b/c"))
	testx.Equal(t, "single", pathBase("single"))
}

func TestInnermostScope(t *testing.T) {
	outerNode := &fakeNode{start: 1, end: 100}
	innerNode := &fakeNode{start: 10, end: 90}

	outer := types.NewScope(nil, outerNode.Pos(), outerNode.End(), "outer")
	inner := types.NewScope(outer, innerNode.Pos(), innerNode.End(), "inner")

	scopes := map[ast.Node]*types.Scope{
		outerNode: outer,
		innerNode: inner,
	}

	got := innermostScope(scopes, 20)
	testx.True(t, got == inner, "innermostScope should return the most specific scope")
}

func TestVarsVisibleAt(t *testing.T) {
	pkg := types.NewPackage("example.com/p", "p")
	pkgScope := pkg.Scope()

	global := types.NewVar(1, pkg, "global", types.Typ[types.Int])
	testx.Nil(t, pkgScope.Insert(global))

	fileScope := types.NewScope(pkgScope, 2, 200, "file")
	fnScope := types.NewScope(fileScope, 10, 190, "func")
	blockScope := types.NewScope(fnScope, 20, 180, "block")

	testx.Nil(t, fnScope.Insert(types.NewVar(11, pkg, "param", types.Typ[types.Int])))
	testx.Nil(t, fnScope.Insert(types.NewVar(12, pkg, "shadow", types.Typ[types.Int])))
	testx.Nil(t, blockScope.Insert(types.NewVar(21, pkg, "x", types.Typ[types.Int])))
	testx.Nil(t, blockScope.Insert(types.NewVar(22, pkg, "shadow", types.Typ[types.Int])))
	testx.Nil(t, blockScope.Insert(types.NewVar(170, pkg, "future", types.Typ[types.Int])))

	node := &fakeNode{start: 20, end: 180}
	info := &types.Info{Scopes: map[ast.Node]*types.Scope{node: blockScope}}

	got := varsVisibleAt(info, pkg, 100)
	want := []string{"param", "x", "shadow"}
	testx.True(t, reflect.DeepEqual(got, want), "varsVisibleAt should exclude globals/future vars and honor shadowing")
}

func TestInstrumentFileInjectsWithVariables(t *testing.T) {
	pkg := types.NewPackage("example.com/p", "p")
	pkgScope := pkg.Scope()

	fileScope := types.NewScope(pkgScope, 1, 200, "file")
	fnScope := types.NewScope(fileScope, 10, 190, "func")
	blockScope := types.NewScope(fnScope, 20, 180, "block")
	testx.Nil(t, blockScope.Insert(types.NewVar(30, pkg, "x", types.Typ[types.Int])))

	tpPkg := types.NewPackage("timepointlib/timepoint", "timepoint")
	pkgIdent := &ast.Ident{Name: "timepoint", NamePos: 40}
	createSel := &ast.SelectorExpr{X: pkgIdent, Sel: &ast.Ident{Name: "Create", NamePos: 50}}
	call := &ast.CallExpr{Fun: createSel, Lparen: 60, Rparen: 61}
	block := &ast.BlockStmt{Lbrace: 20, List: []ast.Stmt{&ast.ExprStmt{X: call}}, Rbrace: 180}
	fn := &ast.FuncDecl{Name: ast.NewIdent("f"), Type: &ast.FuncType{Func: 10, Params: &ast.FieldList{}}, Body: block}
	file := &ast.File{Name: ast.NewIdent("p"), Decls: []ast.Decl{fn}}

	info := &types.Info{
		Uses: map[*ast.Ident]types.Object{
			pkgIdent: types.NewPkgName(pkgIdent.Pos(), pkg, "timepoint", tpPkg),
		},
		Scopes: map[ast.Node]*types.Scope{
			block: blockScope,
		},
	}

	changed := instrumentFile(file, info, pkg, map[string]struct{}{"timepoint": {}})
	testx.True(t, changed, "instrumentFile should modify a Create call without WithVariables")
	testx.Equal(t, 1, len(call.Args))

	withVars, ok := call.Args[0].(*ast.CallExpr)
	testx.True(t, ok, "first argument must be a call expression")
	sel, ok := withVars.Fun.(*ast.SelectorExpr)
	testx.True(t, ok && sel.Sel.Name == "WithVariables", "first argument must call WithVariables")
	testx.Equal(t, 1, len(withVars.Args))
}
