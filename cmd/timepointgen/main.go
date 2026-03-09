package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "timepointgen:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("timepointgen", flag.ContinueOnError)
	fs.SetOutput(stderr)

	write := fs.Bool("w", false, "write changes back to files")
	verbose := fs.Bool("v", false, "print unchanged files")
	if err := fs.Parse(args); err != nil {
		return err
	}

	roots := fs.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}

	goFiles, err := discoverGoFiles(roots)
	if err != nil {
		return err
	}
	if len(goFiles) == 0 {
		return errors.New("no Go files found")
	}

	dirs := groupByDir(goFiles)
	dirNames := make([]string, 0, len(dirs))
	for dir := range dirs {
		dirNames = append(dirNames, dir)
	}
	sort.Strings(dirNames)

	totalChanges := 0
	for _, dir := range dirNames {
		changes, err := processDir(dir, dirs[dir], *write, *verbose, stdout)
		if err != nil {
			return err
		}
		totalChanges += changes
	}

	if totalChanges == 0 {
		fmt.Fprintln(stdout, "timepointgen: no changes")
	}
	return nil
}

func discoverGoFiles(roots []string) ([]string, error) {
	var files []string
	seen := map[string]struct{}{}

	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			return nil, err
		}

		if !info.IsDir() {
			if strings.HasSuffix(root, ".go") && !strings.HasSuffix(root, "_test.go") {
				abs, err := filepath.Abs(root)
				if err != nil {
					return nil, err
				}
				if _, ok := seen[abs]; !ok {
					seen[abs] = struct{}{}
					files = append(files, abs)
				}
			}
			continue
		}

		err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if path == root {
					return nil
				}
				name := d.Name()
				if name == ".git" || name == "vendor" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, "_test.go") || !strings.HasSuffix(path, ".go") {
				return nil
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			if _, ok := seen[abs]; ok {
				return nil
			}
			seen[abs] = struct{}{}
			files = append(files, abs)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Strings(files)
	return files, nil
}

func groupByDir(files []string) map[string][]string {
	out := make(map[string][]string)
	for _, file := range files {
		dir := filepath.Dir(file)
		out[dir] = append(out[dir], file)
	}
	for dir := range out {
		sort.Strings(out[dir])
	}
	return out
}

type parsedFile struct {
	path string
	ast  *ast.File
}

func processDir(dir string, files []string, write, verbose bool, stdout *os.File) (int, error) {
	fset := token.NewFileSet()
	packageFiles := map[string][]parsedFile{}

	for _, path := range files {
		fileAst, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", path, err)
		}
		packageFiles[fileAst.Name.Name] = append(packageFiles[fileAst.Name.Name], parsedFile{path: path, ast: fileAst})
	}

	totalChanges := 0
	packageNames := make([]string, 0, len(packageFiles))
	for pkgName := range packageFiles {
		packageNames = append(packageNames, pkgName)
	}
	sort.Strings(packageNames)

	for _, pkgName := range packageNames {
		pf := packageFiles[pkgName]
		changes, err := processPackage(fset, pf, write, verbose, stdout)
		if err != nil {
			return 0, fmt.Errorf("process %s (%s): %w", dir, pkgName, err)
		}
		totalChanges += changes
	}

	return totalChanges, nil
}

func processPackage(fset *token.FileSet, files []parsedFile, write, verbose bool, stdout *os.File) (int, error) {
	astFiles := make([]*ast.File, 0, len(files))
	for _, file := range files {
		astFiles = append(astFiles, file.ast)
	}

	info := &types.Info{
		Uses:   make(map[*ast.Ident]types.Object),
		Scopes: make(map[ast.Node]*types.Scope),
	}

	config := &types.Config{Importer: importer.Default()}
	pkg, err := config.Check("", fset, astFiles, info)
	if err != nil {
		return 0, fmt.Errorf("type-check failed: %w", err)
	}

	changes := 0
	for _, file := range files {
		timepointAliases := importsForTimepoint(file.ast)
		if len(timepointAliases) == 0 {
			if verbose {
				fmt.Fprintln(stdout, file.path, "(unchanged)")
			}
			continue
		}

		changed := instrumentFile(file.ast, info, pkg, timepointAliases)
		if !changed {
			if verbose {
				fmt.Fprintln(stdout, file.path, "(unchanged)")
			}
			continue
		}

		rendered, err := renderFile(fset, file.ast)
		if err != nil {
			return 0, fmt.Errorf("render %s: %w", file.path, err)
		}

		if write {
			if err := os.WriteFile(file.path, rendered, 0644); err != nil {
				return 0, fmt.Errorf("write %s: %w", file.path, err)
			}
		}
		fmt.Fprintln(stdout, file.path)
		changes++
	}

	return changes, nil
}

func renderFile(fset *token.FileSet, file *ast.File) ([]byte, error) {
	var buf bytes.Buffer
	cfg := &printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := cfg.Fprint(&buf, fset, file); err != nil {
		return nil, err
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), nil
	}
	return formatted, nil
}

func importsForTimepoint(file *ast.File) map[string]struct{} {
	aliases := map[string]struct{}{}
	for _, imp := range file.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if !isTimepointImportPath(importPath) {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "." || imp.Name.Name == "_" {
				continue
			}
			aliases[imp.Name.Name] = struct{}{}
			continue
		}
		aliases[pathBase(importPath)] = struct{}{}
	}
	return aliases
}

func isTimepointImportPath(path string) bool {
	return path == "timepoint" || strings.HasSuffix(path, "/timepoint")
}

func pathBase(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return path
	}
	return path[idx+1:]
}

func instrumentFile(file *ast.File, info *types.Info, pkg *types.Package, aliases map[string]struct{}) bool {
	changed := false

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil || selector.Sel.Name != "Create" {
			return true
		}
		pkgIdent, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if _, ok := aliases[pkgIdent.Name]; !ok {
			return true
		}

		obj := info.Uses[pkgIdent]
		pkgObj, ok := obj.(*types.PkgName)
		if !ok || pkgObj.Imported() == nil || !isTimepointImportPath(pkgObj.Imported().Path()) {
			return true
		}

		if hasWithVariablesArg(call, pkgIdent.Name, info) {
			return true
		}

		vars := varsVisibleAt(info, pkg, call.Pos())
		if len(vars) == 0 {
			return true
		}

		withVarsArg := buildWithVariablesArg(pkgIdent.Name, vars)
		call.Args = append([]ast.Expr{withVarsArg}, call.Args...)
		changed = true
		return true
	})

	return changed
}

func hasWithVariablesArg(call *ast.CallExpr, alias string, info *types.Info) bool {
	for _, arg := range call.Args {
		argCall, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := argCall.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil || sel.Sel.Name != "WithVariables" {
			continue
		}
		xid, ok := sel.X.(*ast.Ident)
		if !ok || xid.Name != alias {
			continue
		}
		obj := info.Uses[xid]
		if _, ok := obj.(*types.PkgName); ok {
			return true
		}
	}
	return false
}

type scopedVar struct {
	name string
	pos  token.Pos
}

func varsVisibleAt(info *types.Info, pkg *types.Package, pos token.Pos) []string {
	scope := innermostScope(info.Scopes, pos)
	if scope == nil {
		return nil
	}

	seen := map[string]struct{}{}
	vars := make([]scopedVar, 0, 16)

	for s := scope; s != nil; s = s.Parent() {
		names := s.Names()
		sort.Strings(names)
		for _, name := range names {
			if name == "_" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			obj := s.Lookup(name)
			if obj == nil {
				continue
			}
			v, ok := obj.(*types.Var)
			if !ok {
				continue
			}
			if v.Pkg() == pkg && obj.Parent() == pkg.Scope() {
				// Global variables are intentionally excluded; this instrumentation focuses on in-scope locals.
				continue
			}
			if obj.Pos() == token.NoPos || obj.Pos() > pos {
				continue
			}

			seen[name] = struct{}{}
			vars = append(vars, scopedVar{name: name, pos: obj.Pos()})
		}
	}

	sort.Slice(vars, func(i, j int) bool {
		if vars[i].pos == vars[j].pos {
			return vars[i].name < vars[j].name
		}
		return vars[i].pos < vars[j].pos
	})

	out := make([]string, 0, len(vars))
	for _, v := range vars {
		out = append(out, v.name)
	}
	return out
}

func innermostScope(scopes map[ast.Node]*types.Scope, pos token.Pos) *types.Scope {
	var best *types.Scope
	for _, scope := range scopes {
		if scope == nil {
			continue
		}
		if !(scope.Pos() <= pos && pos < scope.End()) {
			continue
		}
		if best == nil {
			best = scope
			continue
		}
		if scope.Pos() >= best.Pos() && scope.End() <= best.End() {
			best = scope
		}
	}
	return best
}

func buildWithVariablesArg(alias string, vars []string) ast.Expr {
	args := make([]ast.Expr, 0, len(vars))
	for _, name := range vars {
		args = append(args, &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(alias),
				Sel: ast.NewIdent("AnyVar"),
			},
			Args: []ast.Expr{
				&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(name)},
				&ast.UnaryExpr{Op: token.AND, X: ast.NewIdent(name)},
			},
		})
	}

	return &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(alias),
			Sel: ast.NewIdent("WithVariables"),
		},
		Args: args,
	}
}
