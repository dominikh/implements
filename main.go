package main

import (
	"code.google.com/p/go.tools/go/types"
	"fmt"
)

type Interface struct {
	Name       string
	Underlying *types.Interface
	Obj        types.Object
}

func getInterfaces(typs []types.Object) []Interface {
	var interfaces []Interface

	for _, obj := range typs {
		// Only types, not variables/constants
		if v, ok := obj.(*types.TypeName); ok {
			// Only interfaces
			if iface, ok := v.Type().Underlying().(*types.Interface); ok {
				interfaces = append(interfaces, Interface{obj.Name(), iface, obj})
			}
		}

	}

	return interfaces
}

func getTypes(paths ...string) []types.Object {
	var typs []types.Object

	imports := make(map[string]*types.Package)
	for _, path := range paths {
		pkg, err := types.GcImport(imports, path)
		if err != nil {
			panic(err)
		}

		scope := pkg.Scope()
		for i := 0; i < scope.NumEntries(); i++ {
			obj := scope.At(i)

			// Only types, not variables/constants
			if _, ok := obj.(*types.TypeName); ok {
				typs = append(typs, obj)
			}

		}
	}
	return typs
}

func main() {
	typs := getTypes("io", "fmt", "net/http")
	interfaces := getInterfaces(typs)

	for _, typ := range typs {
		for _, iface := range interfaces {
			// if typ.Pkg() == iface.Obj.Pkg() {
			//	continue
			// }

			if iface.Underlying.NumMethods() == 0 {
				continue
			}
			if typ.Name() == iface.Name {
				// FIXME this is hackish
				continue
			}

			if fnc, _ := types.MissingMethod(typ.Type(), iface.Underlying); fnc == nil {

				fmt.Printf("%s.%s implements %s.%s\n",
					typ.(*types.TypeName).Pkg().Name(), typ.Name(),
					iface.Obj.Pkg().Name(), iface.Name)
			}
		}
	}
}
