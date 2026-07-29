package requirements

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/registry"
	"github.com/runmedev/owl/internal/store"
)

func ContractsFromConfig(input model.ConfigInput, source model.Source, types registry.TypeProvider) ([]store.EnvContract, error) {
	if len(input.Needs) == 0 {
		return nil, nil
	}
	if source.Name == "" {
		source = model.Source{Name: "[config]", Kind: "owl-config"}
	}
	if source.Kind == "" {
		source.Kind = "owl-config"
	}
	if types == nil {
		types = registry.NewBuiltInRegistry()
	}

	var (
		contracts []store.EnvContract
		order     uint
		seenKeys  = map[string]model.FieldRef{}
	)
	for i, need := range input.Needs {
		path := fmt.Sprintf("needs[%d]", i)
		typeRef := strings.TrimSpace(string(need.Type))
		if typeRef == "" {
			return nil, fmt.Errorf("%s.type: type is required", path)
		}
		instance := strings.TrimSpace(need.Instance)
		if instance == "" {
			return nil, fmt.Errorf("%s.instance: instance is required", path)
		}
		typeDef, ok, err := types.ResolveTypeRef(typeRef)
		if err != nil {
			return nil, fmt.Errorf("%s.type: %w", path, err)
		}
		if !ok {
			return nil, fmt.Errorf("%s.type: unknown type %q", path, typeRef)
		}

		fieldKeys, err := dotenvFieldKeys(need, typeDef, path)
		if err != nil {
			return nil, err
		}

		var fields []string
		for field := range fieldKeys {
			fields = append(fields, field)
		}
		sort.Slice(fields, func(i, j int) bool {
			left := fieldKeys[fields[i]]
			right := fieldKeys[fields[j]]
			if left == right {
				return fields[i] < fields[j]
			}
			return left < right
		})

		contract := store.EnvContract{
			Source:     source,
			Projection: model.ProjectionDotenv,
		}
		for _, fieldName := range fields {
			fieldDef := typeDef.Fields[fieldName]
			ref := model.FieldRef{TypeID: typeDef.ID, Instance: instance, Field: fieldName}
			key := fieldKeys[fieldName]
			if previous, ok := seenKeys[key]; ok && previous != ref {
				return nil, fmt.Errorf("%s.dotenv.%s: duplicate dotenv key %q already bound to %s", path, fieldName, key, previous.String())
			}
			seenKeys[key] = ref
			order++
			contract.Bindings = append(contract.Bindings, store.EnvBinding{
				FieldRef:    ref,
				Key:         key,
				Projection:  model.ProjectionDotenv,
				Required:    fieldDef.Required,
				Description: fieldDef.Description,
				Source:      source,
				Order:       order,
			})
		}
		contracts = append(contracts, contract)
	}
	return contracts, nil
}

func dotenvFieldKeys(need model.NeedInput, typeDef model.TypeDef, path string) (map[string]string, error) {
	fieldKeys := map[string]string{}
	for fieldName, fieldDef := range typeDef.Fields {
		if fieldDef.Required {
			fieldKeys[fieldName] = inferDotenvKey(typeDef, need.Instance, fieldName, fieldDef)
		}
	}
	if need.Dotenv == nil {
		return fieldKeys, nil
	}
	for i, binding := range need.Dotenv.Fields {
		fieldName := strings.TrimSpace(binding.Field)
		if fieldName == "" {
			return nil, fmt.Errorf("%s.dotenv.fields[%d].field: field is required", path, i)
		}
		if _, ok := typeDef.Fields[fieldName]; !ok {
			return nil, fmt.Errorf("%s.dotenv.fields[%d].field: unknown field %q for type %s", path, i, fieldName, typeDef.ID)
		}
		key := strings.TrimSpace(binding.Key)
		if key == "" {
			return nil, fmt.Errorf("%s.dotenv.fields[%d].key: key is required", path, i)
		}
		fieldKeys[fieldName] = key
	}
	return fieldKeys, nil
}

func inferDotenvKey(typeDef model.TypeDef, instance string, fieldName string, fieldDef model.FieldDef) string {
	if instance == "default" {
		if fieldDef.PreferredDotenvKey != "" {
			return fieldDef.PreferredDotenvKey
		}
		return upperSnake(typeDef.Name + "_" + fieldName)
	}
	return upperSnake(instance + "_" + typeDef.Name + "_" + fieldName)
}

func upperSnake(value string) string {
	var b strings.Builder
	var previousUnderscore bool
	for i, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if i > 0 && unicode.IsUpper(r) && !previousUnderscore {
				_ = b.WriteByte('_')
			}
			_, _ = b.WriteRune(unicode.ToUpper(r))
			previousUnderscore = false
		default:
			if b.Len() > 0 && !previousUnderscore {
				_ = b.WriteByte('_')
				previousUnderscore = true
			}
		}
	}
	return strings.TrimRight(b.String(), "_")
}
