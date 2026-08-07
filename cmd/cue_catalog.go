package cmd

import (
	"fmt"
	"os"

	"github.com/runmedev/owl/internal/registry"
)

const cueRootEnv = "OWL_CUE_ROOT"

var lookupEnv = os.LookupEnv

func commandTypeProvider() (registry.TypeProvider, error) {
	root, configured := lookupEnv(cueRootEnv)
	if !configured {
		return registry.NewBuiltInRegistry(), nil
	}
	if root == "" {
		return nil, fmt.Errorf("%s is set but empty", cueRootEnv)
	}
	types, err := registry.NewBuiltInRegistryFromDirectory(root)
	if err != nil {
		return nil, fmt.Errorf("load CUE catalog from %s=%q: %w", cueRootEnv, root, err)
	}
	return types, nil
}
