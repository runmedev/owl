package seed

import (
	"bytes"
	"errors"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/runmedev/owl/internal/requirements"
	"github.com/runmedev/owl/pkg/owl"
)

type baseStoreRequest struct {
	options          Options
	allowMissingSpec bool
	loadConfig       bool
	loadValues       bool
}

func newBaseStore(req baseStoreRequest) (*owl.Store, error) {
	opts := req.options
	var storeOpts []owl.StoreOption
	if opts.TypeProvider != nil {
		storeOpts = append(storeOpts, owl.WithTypeProvider(opts.TypeProvider))
	}

	var configPath string
	if req.loadConfig {
		path, err := resolveConfigPath(opts.WorkDir, opts.ConfigPath, false)
		if err != nil {
			return nil, err
		}
		configPath = path
		if configPath != "" {
			input, err := requirements.ReadConfigFile(configPath)
			if err != nil {
				return nil, err
			}
			storeOpts = append(storeOpts, owl.WithConfig(input))
		}
	}

	if configPath != "" {
		if err := validateNoHumanDotenvSpecs(opts.WorkDir, opts.SpecFiles); err != nil {
			return nil, err
		}
	}

	specFiles, err := filesOrDefaults(opts.WorkDir, opts.SpecFiles, ".env.sample", ".env.example", ".env.spec")
	if err != nil {
		return nil, err
	}
	for _, file := range specFiles {
		raw, err := os.ReadFile(file)
		if req.allowMissingSpec && errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if configPath != "" {
			if isGeneratedDotenvSpec(raw) {
				continue
			}
			return nil, errors.New("dotenv spec file exists beside Owl config; move it aside or regenerate it with owl project spec --write")
		}
		storeOpts = append(storeOpts, owl.WithEnvSpec(file, bytes.NewReader(raw)))
	}

	if req.loadValues {
		envFiles, err := filesOrDefaults(opts.WorkDir, opts.EnvFiles, defaultEnvFiles...)
		if err != nil {
			return nil, err
		}
		storeOpts = append(storeOpts, owl.WithDotenv("[process]", strings.NewReader(processEnvDotenv(flattenObservedEnviron(opts.Observed)))))
		for _, file := range envFiles {
			raw, ok, err := readOptionalEnvFile(file)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			storeOpts = append(storeOpts, owl.WithDotenv(file, bytes.NewReader(raw)))
		}
	}

	return owl.NewStore(storeOpts...)
}

func seedInheritedValues(store *owl.Store, catalog Catalog) error {
	explicit, err := explicitSnapshotKeys(store)
	if err != nil {
		return err
	}
	return loadInheritedVariables(store, catalog.All(), explicit)
}

func explicitSnapshotKeys(store *owl.Store) (map[string]struct{}, error) {
	items, err := store.Snapshot(owl.SnapshotPolicy{})
	if err != nil {
		return nil, err
	}
	keys := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Explicit {
			keys[item.Name] = struct{}{}
		}
	}
	return keys, nil
}

func loadInheritedVariables(store *owl.Store, vars []owl.DotenvVariable, explicit map[string]struct{}) error {
	type sourceVars struct {
		source owl.Source
		vars   []owl.DotenvVariable
	}
	positions := make(map[owl.Source]int)
	groups := make([]sourceVars, 0)
	for _, variable := range vars {
		if _, ok := explicit[variable.Key]; ok {
			continue
		}
		source := variable.Source
		index, ok := positions[source]
		if !ok {
			index = len(groups)
			positions[source] = index
			groups = append(groups, sourceVars{source: source})
		}
		groups[index].vars = append(groups[index].vars, variable)
	}
	for _, group := range groups {
		if err := store.LoadDotenv(group.source, group.vars); err != nil {
			return err
		}
	}
	return nil
}

func projectionKeys(store *owl.Store) map[string]struct{} {
	items, err := store.Snapshot(owl.SnapshotPolicy{})
	if err != nil {
		return nil
	}
	keys := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Name != "" {
			keys[item.Name] = struct{}{}
		}
	}
	return keys
}

func flattenObservedEnviron(sources []ObservedSource) []string {
	var envs []string
	for _, source := range sources {
		envs = append(envs, source.Environ...)
	}
	return envs
}

func processEnvDotenv(envs []string) string {
	envs = append([]string{}, envs...)
	sort.Strings(envs)
	var b strings.Builder
	for _, env := range envs {
		key, value, ok := strings.Cut(env, "=")
		if !ok || !isDotenvKey(key) {
			continue
		}
		_, _ = b.WriteString(key)
		_ = b.WriteByte('=')
		_, _ = b.WriteString(strconv.Quote(value))
		_ = b.WriteByte('\n')
	}
	return b.String()
}
