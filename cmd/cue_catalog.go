package cmd

import (
	"fmt"
	"os"

	"github.com/runmedev/owl/pkg/owl"
)

const cueRootEnv = "OWL_CUE_ROOT"

var lookupEnv = os.LookupEnv

func commandTypeProvider() (owl.TypeProvider, error) {
	root, configured := lookupEnv(cueRootEnv)
	if !configured {
		return owl.NewBuiltInTypeProvider(), nil
	}
	if root == "" {
		return nil, fmt.Errorf("%s is set but empty", cueRootEnv)
	}
	types, err := owl.NewTypeProviderFromDirectory(root)
	if err != nil {
		return nil, fmt.Errorf("load CUE catalog from %s=%q: %w", cueRootEnv, root, err)
	}
	return types, nil
}
