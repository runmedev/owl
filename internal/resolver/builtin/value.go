package builtin

import (
	"context"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/resolver"
)

const (
	ResolverIDProcess model.ResolverID = "core/process"
	ResolverIDDotenv  model.ResolverID = "core/dotenv"
)

type Catalog struct {
	Source model.Source
	Values map[model.ProjectionKey]string
}

type ValueResolver struct {
	id          model.ResolverID
	name        string
	description string
	catalogs    []Catalog
}

func ProcessResolver(catalogs ...Catalog) ValueResolver {
	return ValueResolver{
		id:          ResolverIDProcess,
		name:        "Process",
		description: "looks up values from supplied process environment sources",
		catalogs:    catalogs,
	}
}

func DotenvResolver(catalogs ...Catalog) ValueResolver {
	return ValueResolver{
		id:          ResolverIDDotenv,
		name:        "Dotenv",
		description: "looks up values from supplied dotenv sources",
		catalogs:    catalogs,
	}
}

func (r ValueResolver) Describe(context.Context) (resolver.Descriptor, error) {
	return resolver.Descriptor{
		ID:           r.id,
		Name:         r.name,
		Description:  r.description,
		Capabilities: []resolver.Capability{resolver.CapabilityValueLookup},
	}, nil
}

func (r ValueResolver) Resolve(_ context.Context, req resolver.Request) (resolver.Result, error) {
	if req.Need.ProjectionKey == "" {
		return resolver.Result{Outcome: model.ResolverAttemptNotApplicable}, nil
	}
	for _, catalog := range r.catalogs {
		value, ok := catalog.Values[req.Need.ProjectionKey]
		if !ok {
			continue
		}
		source := catalog.Source
		if source.Name == "" && source.Kind == "" {
			source = req.SourceHint
		}
		return resolver.Result{
			Outcome: model.ResolverAttemptResolved,
			Proposed: &resolver.ProposedValue{
				Value:       value,
				Source:      source,
				Sensitivity: req.Need.Sensitivity,
				Exposure:    req.Need.Exposure,
			},
		}, nil
	}
	return resolver.Result{Outcome: model.ResolverAttemptNotFound}, nil
}
