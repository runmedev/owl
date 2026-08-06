package seed

import (
	"context"
	"os"
	"sort"
	"strings"

	"github.com/runmedev/owl/internal/projection/dotenv"
	"github.com/runmedev/owl/pkg/owl"
)

type catalogBuildRequest struct {
	options    Options
	direnvKeys map[string]struct{}
}

// Catalog contains candidate values grouped by source role.
type Catalog struct {
	observed []owl.DotenvVariable
	dotenv   []owl.DotenvVariable
}

// All returns every candidate value in seed precedence order.
func (c Catalog) All() []owl.DotenvVariable {
	vars := make([]owl.DotenvVariable, 0, len(c.observed)+len(c.dotenv))
	vars = append(vars, c.observed...)
	vars = append(vars, c.dotenv...)
	return vars
}

// ProcessResolverInput returns observed values for the process resolver.
func (c Catalog) ProcessResolverInput() []owl.DotenvVariable {
	return append([]owl.DotenvVariable{}, c.observed...)
}

// DotenvResolverInput returns dotenv-shaped values in resolver precedence order.
func (c Catalog) DotenvResolverInput() []owl.DotenvVariable {
	return reverseSourceGroups(c.dotenv)
}

func buildCatalog(ctx context.Context, req catalogBuildRequest) (Catalog, []owl.Diagnostic, error) {
	opts := req.options
	observed := observedVariables(opts.Observed)
	dotenvVars, err := dotenvVariables(opts)
	if err != nil {
		return Catalog{}, nil, err
	}
	direnvVars, diagnostics, err := direnvVariables(ctx, direnvVariablesRequest{
		options:  opts,
		keys:     req.direnvKeys,
		observed: observedValueMap(observed),
	})
	if err != nil {
		return Catalog{}, diagnostics, err
	}
	dotenvVars = append(dotenvVars, direnvVars...)
	return Catalog{observed: observed, dotenv: dotenvVars}, diagnostics, nil
}

func observedVariables(sources []ObservedSource) []owl.DotenvVariable {
	var vars []owl.DotenvVariable
	for _, observed := range sources {
		source := observed.Source
		if source.Name == "" && source.Kind == "" {
			source = owl.Source{Name: "[process]", Kind: "process"}
		}
		for _, item := range observed.Environ {
			key, value, ok := strings.Cut(item, "=")
			if !ok || !isDotenvKey(key) {
				continue
			}
			vars = append(vars, owl.DotenvVariable{
				Key:    key,
				Value:  value,
				Source: source,
			})
		}
	}
	sort.SliceStable(vars, func(i, j int) bool {
		if vars[i].Source == vars[j].Source {
			return vars[i].Key < vars[j].Key
		}
		if vars[i].Source.Name == vars[j].Source.Name {
			return vars[i].Source.Kind < vars[j].Source.Kind
		}
		return vars[i].Source.Name < vars[j].Source.Name
	})
	return vars
}

func dotenvVariables(opts Options) ([]owl.DotenvVariable, error) {
	envFiles, err := filesOrDefaults(opts.WorkDir, opts.EnvFiles, ".env")
	if err != nil {
		return nil, err
	}
	var vars []owl.DotenvVariable
	for _, file := range envFiles {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		parsed, err := dotenv.ParseDotenvValues(raw)
		if err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(parsed))
		for key := range parsed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			vars = append(vars, owl.DotenvVariable{
				Key:    key,
				Value:  parsed[key],
				Source: owl.Source{Name: file, Kind: "dotenv"},
			})
		}
	}
	return vars, nil
}

func reverseSourceGroups(vars []owl.DotenvVariable) []owl.DotenvVariable {
	type sourceGroup struct {
		source owl.Source
		vars   []owl.DotenvVariable
	}
	positions := make(map[owl.Source]int)
	groups := make([]sourceGroup, 0)
	for _, variable := range vars {
		source := variable.Source
		index, ok := positions[source]
		if !ok {
			index = len(groups)
			positions[source] = index
			groups = append(groups, sourceGroup{source: source})
		}
		groups[index].vars = append(groups[index].vars, variable)
	}
	reversed := make([]owl.DotenvVariable, 0, len(vars))
	for i := len(groups) - 1; i >= 0; i-- {
		reversed = append(reversed, groups[i].vars...)
	}
	return reversed
}

func observedValueMap(vars []owl.DotenvVariable) map[string]string {
	values := make(map[string]string, len(vars))
	for _, variable := range vars {
		values[variable.Key] = variable.Value
	}
	return values
}
