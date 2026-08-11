package seed

import (
	"context"

	internalseed "github.com/runmedev/owl/internal/seed"
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
	TypeProvider owl.TypeProvider
}

// ObservedSource is an environment-shaped snapshot with caller-provided provenance.
type ObservedSource struct {
	Source  owl.Source
	Environ []string
}

// DirenvPolicy controls whether seed evaluates direnv for .envrc values.
type DirenvPolicy = internalseed.DirenvPolicy

const (
	// DirenvDisabled prevents direnv execution.
	DirenvDisabled = internalseed.DirenvDisabled
	// DirenvEnabledWarn records direnv failures as diagnostics and continues.
	DirenvEnabledWarn = internalseed.DirenvEnabledWarn
	// DirenvEnabledError returns direnv failures as errors.
	DirenvEnabledError = internalseed.DirenvEnabledError
)

// DirenvExportRunner runs direnv export and returns environment key/value pairs.
type DirenvExportRunner = internalseed.DirenvExportRunner

// Result is the seeded store plus source diagnostics used to build it.
type Result struct {
	Store       *owl.Store
	Catalog     Catalog
	Diagnostics []owl.Diagnostic
}

type Catalog = internalseed.Catalog

// NewStore creates an Owl store and resolves values from observed sources.
func NewStore(ctx context.Context, opts Options) (*Result, error) {
	result, err := internalseed.NewStore(ctx, internalOptions(opts))
	if err != nil {
		return nil, err
	}
	if _, err := result.Store.Resolve(ctx, owl.ResolveInput{
		Process: result.Catalog.ProcessResolverInput(),
		Dotenv:  result.Catalog.DotenvResolverInput(),
	}); err != nil {
		return nil, err
	}
	return &Result{
		Store:       result.Store,
		Catalog:     result.Catalog,
		Diagnostics: result.Diagnostics,
	}, nil
}

func NewRawValueStore(opts Options, allowMissingSpec bool) (*owl.Store, error) {
	return internalseed.NewRawValueStore(internalOptions(opts), allowMissingSpec)
}

func BuildCatalog(ctx context.Context, opts Options) (Catalog, []owl.Diagnostic, error) {
	return internalseed.BuildCatalog(ctx, internalOptions(opts))
}

func internalOptions(opts Options) internalseed.Options {
	observed := make([]internalseed.ObservedSource, 0, len(opts.Observed))
	for _, source := range opts.Observed {
		observed = append(observed, internalseed.ObservedSource{
			Source:  source.Source,
			Environ: source.Environ,
		})
	}
	return internalseed.Options{
		EnvFiles:     opts.EnvFiles,
		SpecFiles:    opts.SpecFiles,
		ConfigPath:   opts.ConfigPath,
		Observed:     observed,
		WorkDir:      opts.WorkDir,
		Direnv:       opts.Direnv,
		DirenvRunner: opts.DirenvRunner,
		TypeProvider: opts.TypeProvider,
	}
}
