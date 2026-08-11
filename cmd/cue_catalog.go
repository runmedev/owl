package cmd

import (
	"fmt"
	"os"

	"github.com/runmedev/owl/pkg/owl"
)

const cueRootEnv = "OWL_CUE_ROOT"

var lookupEnv = os.LookupEnv

func commandTypeProvider() (owl.TypeProvider, error) {
	root, _ := lookupEnv(cueRootEnv)
	types, err := owl.TypeProviderFromCatalogInput(owl.TypeCatalogInput{Root: root})
	if err != nil {
		return nil, fmt.Errorf("load CUE catalog from %s=%q: %w", cueRootEnv, root, err)
	}
	return types, nil
}
