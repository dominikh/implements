package main // import "honnef.co/go/implements"

import (
	"flag"
	"fmt"
	"go/types"
	"os"
	"strings"

	"golang.org/x/tools/go/loader"

	"github.com/kisielk/gotool"
)

var (
	interfacesFrom string
	typesFrom      string
	reverse        bool
	printHelp      bool
)

func init() {
	flag.StringVar(&interfacesFrom, "interfaces", "std", "Comma-separated list of which packages to scan for interfaces. Defaults to std.")
	flag.StringVar(&typesFrom, "types", "", "Comma-separated list of packages whose types to check for implemented interfaces. Required.")
	flag.BoolVar(&reverse, "reverse", false, "Print 'implemented by' as opposed to 'implements' relations.")
	flag.BoolVar(&printHelp, "help", false, "Print a help text and exit.")

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

type Context struct {
	conf *loader.Config
}

func NewContext() *Context {
	return &Context{}
}

func (ctx *Context) getTypes(paths ...string) ([]Type, error) {
	conf := &loader.Config{}
	for _, path := range paths {
		conf.Import(path)
	}
	lprog, err := conf.Load()
	if err != nil {
		return nil, err
	}
	var typs []Type

	for _, pkg := range lprog.InitialPackages() {
		scope := pkg.Pkg.Scope()
		for _, n := range scope.Names() {
			obj := scope.Lookup(n)

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
	return typs, nil
}

func doesImplement(typ types.Type, iface *types.Interface) bool {
	fnc, _ := types.MissingMethod(typ, iface, true)
	return fnc == nil
}

func listImplementedInterfaces(universe, toCheck []Type) {
	interfaces := getInterfaces(universe)

	for _, typ := range toCheck {
		var implements []Interface
		var implementsPointer []Interface
		for _, iface := range interfaces {
			if iface.Underlying.NumMethods() == 0 {
				// Everything implements empty interfaces, skip those
				continue
			}

			if typ.Object.Pkg() == iface.Obj.Pkg() && typ.Object.Name() == iface.Name {
				// An interface will always implement itself, so skip those
				continue
			}

			if doesImplement(typ.Object.Type(), iface.Underlying) {
				implements = append(implements, iface)
			}

			if _, ok := typ.TypeName.Type().Underlying().(*types.Interface); !ok {
				if doesImplement(typ.Pointer.Underlying(), iface.Underlying) {
					implementsPointer = append(implementsPointer, iface)
				}
			}
		}

		if len(implements) > 0 {
			fmt.Printf("%s.%s implements...\n", typ.TypeName.Pkg().Path(), typ.Object.Name())
			for _, iface := range implements {
				fmt.Printf("\t%s.%s\n", iface.Obj.Pkg().Path(), iface.Name)
			}
		}
		// TODO DRY
		if len(implementsPointer) > 0 {
			fmt.Printf("*%s.%s implements...\n", typ.TypeName.Pkg().Path(), typ.Object.Name())
			for _, iface := range implementsPointer {
				fmt.Printf("\t%s.%s\n", iface.Obj.Pkg().Path(), iface.Name)
			}
		}
	}
}

func listImplementers(universe, toCheck []Type) {
	interfaces := getInterfaces(universe)

	for _, iface := range interfaces {
		if iface.Underlying.NumMethods() == 0 {
			// Everything implements empty interfaces, skip those
			continue
		}

		var implementedBy []string
		for _, typ := range toCheck {
			if typ.Object.Pkg() == iface.Obj.Pkg() && typ.Object.Name() == iface.Name {
				// An interface will always implement itself, so skip those
				continue
			}

			if doesImplement(typ.Object.Type(), iface.Underlying) {
				implementedBy = append(implementedBy, fmt.Sprintf("%s.%s", typ.TypeName.Pkg().Path(), typ.Object.Name()))
			}

			if _, ok := typ.TypeName.Type().Underlying().(*types.Interface); !ok {
				if doesImplement(typ.Pointer.Underlying(), iface.Underlying) {
					implementedBy = append(implementedBy, fmt.Sprintf("*%s.%s", typ.TypeName.Pkg().Path(), typ.Object.Name()))
				}
			}
		}

		if len(implementedBy) > 0 {
			fmt.Printf("%s.%s is implemented by...\n", iface.Obj.Pkg().Path(), iface.Name)
			for _, s := range implementedBy {
				fmt.Printf("\t%s\n", s)
			}
		}
	}
}

func main() {
	if printHelp {
		flag.Usage()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr,
			`implements is a tool that will tell you which types implement which
interfaces, or alternatively by which types interfaces are
implemented.

You use it by specifying a set of packages to scan for interfaces and
another set of packages to scan for types. The two sets can but don't
have to overlap.

When specifying packages, "std" will stand for all of the standard
library. Also, the "..." pattern as understood by the go tool is
supported as well.

By default, implements will iterate all types and list the interfaces
they implement. By supplying the -reverse flag, however, it will
iterate all interfaces and list the types that implement the
interfaces.

Example: For all interfaces in the fmt package you want to know the
types in the standard library that implement them:

    implements -interfaces fmt -types std -reverse

Another example: For all types in your own package you want to know
which interfaces from the standard library they implement:

    implements -interfaces std -types my/own/package`)

		os.Exit(0)
	}

	if typesFrom == "" {
		flag.Usage()
		os.Exit(1)
	}

	ctx := NewContext()
	universe, err := ctx.getTypes(gotool.ImportPaths(strings.Split(interfacesFrom, ","))...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	toCheck, err := ctx.getTypes(gotool.ImportPaths(strings.Split(typesFrom, ","))...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if reverse {
		listImplementers(universe, toCheck)
	} else {
		listImplementedInterfaces(universe, toCheck)
	}
}
