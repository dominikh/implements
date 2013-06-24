package main

import (
	"code.google.com/p/go.tools/go/types"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

var (
	universe string
	own      string
)

func init() {
	flag.StringVar(&universe, "universe", "std", "Comma-separated list of Which packages to scan for interfaces. Defaults to std.")
	flag.StringVar(&own, "own", "", "Comma-separated list of packages whose types to check for implemented interfaces.")

	flag.Parse()
}

type Interface struct {
	Name       string
	Underlying *types.Interface
	Obj        types.Object
}

type Type struct {
	Object   types.Object
	TypeName *types.TypeName
	Pointer  *types.Pointer
}

// getInterfaces extracts all the interfaces from the objects we
// parsed.
func getInterfaces(typs []Type) []Interface {
	var interfaces []Interface

	for _, typ := range typs {
		// Only types, not variables/constants
		// Only interfaces
		if iface, ok := typ.TypeName.Type().Underlying().(*types.Interface); ok {
			interfaces = append(interfaces, Interface{typ.Object.Name(), iface, typ.Object})
		}
	}

	return interfaces
}

func parseFile(fset *token.FileSet, fileName string) (f *ast.File, err error) {
	astFile, err := parser.ParseFile(fset, fileName, nil, 0)
	if err != nil {
		return f, fmt.Errorf("could not parse: %s", err)
	}

	return astFile, nil
}

type Context struct {
	allImports map[string]*types.Package
	context    types.Context
}

func NewContext() *Context {
	ctx := &Context{
		allImports: make(map[string]*types.Package),
	}

	ctx.context = types.Context{
		Import: ctx.importer,
	}

	return ctx
}

func (ctx *Context) importer(imports map[string]*types.Package, path string) (pkg *types.Package, err error) {
	// types.Importer does not seem to be designed for recursive
	// parsing like we're doing here. Specifically, each nested import
	// will maintain its own imports map. This will lead to duplicate
	// imports and in turn packages, which will lead to funny errors
	// such as "cannot pass argument ip (variable of type net.IP) to
	// variable of type net.IP"
	//
	// To work around this, we keep a global imports map, allImports,
	// to which we add all nested imports, and which we use as the
	// cache, instead of imports.
	//
	// Since all nested imports will also use this importer, there
	// should be no way to end up with duplicate imports.

	// We first try to use GcImport directly. This has the downside of
	// using possibly out-of-date packages, but it has the upside of
	// not having to parse most of the Go standard library.

	buildPkg, buildErr := build.Import(path, ".", 0)
	// If we found no build dir, assume we're dealing with installed
	// but no source. If we found a build dir, only use GcImport if
	// it's in GOROOT. This way we always use up-to-date code for
	// normal packages but avoid parsing the standard library.
	if (buildErr == nil && buildPkg.Goroot) || buildErr != nil {
		pkg, err = types.GcImport(ctx.allImports, path)
		if err == nil {
			// We don't use imports, but per API we have to add the package.
			imports[pkg.Path()] = pkg
			ctx.allImports[pkg.Path()] = pkg
			return pkg, nil
		}
	}

	// See if we already imported this package
	if pkg = ctx.allImports[path]; pkg != nil && pkg.Complete() {
		return pkg, nil
	}

	// allImports failed, try to use go/build
	if buildErr != nil {
		return nil, buildErr
	}

	// TODO check if the .a file is up to date and use it instead
	fmt.Println("we have to build", path, "ourselves")
	fileSet := token.NewFileSet()

	isGoFile := func(d os.FileInfo) bool {
		allFiles := make([]string, 0, len(buildPkg.GoFiles)+len(buildPkg.CgoFiles))
		allFiles = append(allFiles, buildPkg.GoFiles...)
		allFiles = append(allFiles, buildPkg.CgoFiles...)

		for _, file := range allFiles {
			if file == d.Name() {
				return true
			}
		}
		return false
	}
	pkgs, err := parser.ParseDir(fileSet, buildPkg.Dir, isGoFile, 0)
	if err != nil {
		return nil, err
	}

	delete(pkgs, "documentation")
	var astPkg *ast.Package
	var name string
	for name, astPkg = range pkgs {
		// Use the first non-main package, or the only package we
		// found.
		//
		// NOTE(dh) I can't think of a reason why there should be
		// multiple packages in a single directory, but ParseDir
		// accommodates for that possibility.
		if len(pkgs) == 1 || name != "main" {
			break
		}
	}

	if astPkg == nil {
		return nil, fmt.Errorf("can't find import: %s", name)
	}

	var ff []*ast.File
	for _, f := range astPkg.Files {
		ff = append(ff, f)
	}

	context := types.Context{
		Import: ctx.importer,
	}

	pkg, err = context.Check(name, fileSet, ff...)
	if err != nil {
		return pkg, err
	}
	if !pkg.Complete() {
		pkg = types.NewPackage(pkg.Pos(), pkg.Path(), pkg.Name(), pkg.Scope(), pkg.Imports(), true)
	}

	imports[path] = pkg
	ctx.allImports[path] = pkg
	return pkg, nil
}

func (ctx *Context) getTypes(paths ...string) []Type {
	var typs []Type

	for _, path := range paths {
		buildPkg, err := build.Import(path, ".", 0)
		if err != nil {
			// FIXME better error handling
			fmt.Printf("Couldn't import %s: %s\n", path, err)
			continue
		}
		fset := token.NewFileSet()
		var astFiles []*ast.File
		var pkg *types.Package
		if buildPkg.Goroot {
			// TODO what if the compiled package in GoRoot is
			// outdated?
			fmt.Println("GcImport", path)
			pkg, err = types.GcImport(ctx.allImports, path)
			if err != nil {
				// TODO better error handling
				fmt.Printf("Couldn't import %s: %s\n", path, err)
				continue
			}
		} else {
			fmt.Println("Manual parsing", path)
			if len(buildPkg.GoFiles) == 0 {
				// TODO proper error handling
				fmt.Printf("Couldn't parse %s: No go files\n", path)
				continue
			}
			for _, file := range buildPkg.GoFiles {
				astFile, err := parseFile(fset, filepath.Join(buildPkg.Dir, file))
				if err != nil {
					// TODO proper error handling
					fmt.Printf("Couldn't parse %s: %s", err)
					continue
				}
				astFiles = append(astFiles, astFile)
			}

			pkg, err = check(ctx, astFiles[0].Name.Name, fset, astFiles)
			if err != nil {
				// FIXME better error handling
				fmt.Printf("Couldn't parse %s: %s\n", path, err)
				continue
			}
			fmt.Println(pkg.Name(), pkg.Complete())
		}

		scope := pkg.Scope()
		for i := 0; i < scope.NumEntries(); i++ {
			obj := scope.At(i)

			// Only types, not variables/constants
			if typ, ok := obj.(*types.TypeName); ok {
				typs = append(typs, Type{
					Object:   obj,
					TypeName: typ,
					Pointer:  types.NewPointer(typ.Type()),
				})
			}

		}
	}
	return typs
}

func check(ctx *Context, name string, fset *token.FileSet, astFiles []*ast.File) (pkg *types.Package, err error) {
	return ctx.context.Check(name, fset, astFiles...)
}

func main() {
	if own == "" {
		flag.Usage()
		os.Exit(1)
	}

	ctx := NewContext()
	stdlib := ctx.getTypes(matchPackages(universe)...)
	toCheck := ctx.getTypes(matchPackages(own)...)

	interfaces := getInterfaces(stdlib)

	for _, typ := range toCheck {
		var implements []string
		var implementsPointer []string
		for _, iface := range interfaces {
			if iface.Underlying.NumMethods() == 0 {
				// Everything implements empty interfaces, skip those
				continue
			}

			if typ.Object.Pkg() == iface.Obj.Pkg() && typ.Object.Name() == iface.Name {
				// An interface will always implement itself, so skip those
				continue
			}

			if fnc, _ := types.MissingMethod(typ.Object.Type(), iface.Underlying); fnc == nil {
				s := fmt.Sprintf("%s.%s",
					iface.Obj.Pkg().Name(), iface.Name)
				implements = append(implements, s)
			}

			// TODO DRY
			if _, ok := typ.TypeName.Type().Underlying().(*types.Interface); !ok {
				if fnc, _ := types.MissingMethod(typ.Pointer.Underlying(), iface.Underlying); fnc == nil {
					s := fmt.Sprintf("%s.%s",
						iface.Obj.Pkg().Name(), iface.Name)
					implementsPointer = append(implementsPointer, s)
				}
			}
		}

		if len(implements) > 0 {
			fmt.Printf("%s.%s implements...\n", typ.TypeName.Pkg().Name(), typ.Object.Name())
			for _, impl := range implements {
				fmt.Printf("\t%s\n", impl)
			}
		}
		// TODO DRY
		if len(implementsPointer) > 0 {
			fmt.Printf("*%s.%s implements...\n", typ.TypeName.Pkg().Name(), typ.Object.Name())
			for _, impl := range implementsPointer {
				fmt.Printf("\t%s\n", impl)
			}
		}
	}
}
