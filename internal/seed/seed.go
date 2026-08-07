package seed

import (
	"context"

	"github.com/runmedev/owl/internal/registry"
	"github.com/runmedev/owl/pkg/owl"
)

// Options configures store seeding from observed environment sources.
type Options struct {
	EnvFiles     []string
	SpecFiles    []string
	ConfigPath   string
	Observed     []ObservedSource
	WorkDir      string
	Direnv       DirenvPolicy
	DirenvRunner DirenvExportRunner
	TypeProvider registry.TypeProvider
}

// ObservedSource is an environment-shaped snapshot with caller-provided provenance.
type ObservedSource struct {
	Source  owl.Source
	Environ []string
}

// Result is the seeded store plus catalog and source diagnostics used to build it.
type Result struct {
	Store       *owl.Store
	Catalog     Catalog
	Diagnostics []owl.Diagnostic
}

// NewStore creates an Owl store and seeds inherited values from observed sources.
func NewStore(ctx context.Context, opts Options) (*Result, error) {
	store, err := newBaseStore(baseStoreRequest{
		options:          opts,
		allowMissingSpec: false,
		loadConfig:       true,
		loadValues:       false,
	})
	if err != nil {
		return nil, err
	}
	catalog, diagnostics, err := buildCatalog(ctx, catalogBuildRequest{
		options:    opts,
		direnvKeys: projectionKeys(store),
	})
	if err != nil {
		return nil, err
	}
	if err := seedInheritedValues(store, catalog); err != nil {
		return nil, err
	}
	return &Result{
		Store:       store,
		Catalog:     catalog,
		Diagnostics: diagnostics,
	}, nil
}

// NewRawValueStore creates a store with raw observed and dotenv values loaded directly.
func NewRawValueStore(opts Options, allowMissingSpec bool) (*owl.Store, error) {
	return newBaseStore(baseStoreRequest{
		options:          opts,
		allowMissingSpec: allowMissingSpec,
		loadConfig:       false,
		loadValues:       true,
	})
}

// BuildCatalog discovers candidate values without creating or mutating a store.
func BuildCatalog(ctx context.Context, opts Options) (Catalog, []owl.Diagnostic, error) {
	return buildCatalog(ctx, catalogBuildRequest{options: opts})
}
