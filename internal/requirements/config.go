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

type ConfigCompiler struct {
	types  registry.TypeProvider
	source model.Source

	order    uint
	seenKeys map[string]model.FieldRef
}

func NewConfigCompiler(types registry.TypeProvider, source model.Source) *ConfigCompiler {
	if source.Name == "" {
		source = model.Source{Name: "[config]", Kind: "owl-config"}
	}
	if source.Kind == "" {
		source.Kind = "owl-config"
	}
	if types == nil {
		types = registry.NewBuiltInRegistry()
	}
	return &ConfigCompiler{
		types:    types,
		source:   source,
		seenKeys: map[string]model.FieldRef{},
	}
}

func ContractsFromConfig(input model.ConfigInput, source model.Source, types registry.TypeProvider) ([]store.EnvContract, error) {
	return NewConfigCompiler(types, source).Compile(input)
}

func (c *ConfigCompiler) Compile(input model.ConfigInput) ([]store.EnvContract, error) {
	if len(input.Needs) == 0 {
		return nil, nil
	}

	contracts := make([]store.EnvContract, 0, len(input.Needs))
	for i, need := range input.Needs {
		contract, err := c.compileNeed(fmt.Sprintf("needs[%d]", i), need)
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, contract)
	}
	return contracts, nil
}

func (c *ConfigCompiler) compileNeed(path string, need model.NeedInput) (store.EnvContract, error) {
	typeRef := strings.TrimSpace(string(need.Type))
	if typeRef == "" {
		return store.EnvContract{}, fmt.Errorf("%s.type: type is required", path)
	}
	instance := strings.TrimSpace(need.Instance)
	if instance == "" {
		return store.EnvContract{}, fmt.Errorf("%s.instance: instance is required", path)
	}
	typeDef, ok, err := c.types.ResolveTypeRef(typeRef)
	if err != nil {
		return store.EnvContract{}, fmt.Errorf("%s.type: %w", path, err)
	}
	if !ok {
		return store.EnvContract{}, fmt.Errorf("%s.type: unknown type %q", path, typeRef)
	}

	fieldKeys, err := c.dotenvFieldKeys(path, need, typeDef)
	if err != nil {
		return store.EnvContract{}, err
	}

	fields := sortedFieldNames(fieldKeys)
	contract := store.EnvContract{
		Source:     c.source,
		Projection: model.ProjectionDotenv,
	}
	for _, fieldName := range fields {
		binding, err := c.compileBinding(path, typeDef, instance, fieldName, fieldKeys[fieldName])
		if err != nil {
			return store.EnvContract{}, err
		}
		contract.Bindings = append(contract.Bindings, binding)
	}
	return contract, nil
}

func (c *ConfigCompiler) compileBinding(path string, typeDef model.TypeDef, instance string, fieldName string, key string) (store.EnvBinding, error) {
	fieldDef := typeDef.Fields[fieldName]
	ref := model.FieldRef{TypeID: typeDef.ID, Instance: instance, Field: fieldName}
	if previous, ok := c.seenKeys[key]; ok && previous != ref {
		return store.EnvBinding{}, fmt.Errorf("%s.dotenv.%s: duplicate dotenv key %q already bound to %s", path, fieldName, key, previous.String())
	}
	c.seenKeys[key] = ref
	c.order++
	return store.EnvBinding{
		FieldRef:    ref,
		Key:         key,
		Projection:  model.ProjectionDotenv,
		Required:    fieldDef.Required,
		Description: fieldDef.Description,
		Source:      c.source,
		Order:       c.order,
	}, nil
}

func (c *ConfigCompiler) dotenvFieldKeys(path string, need model.NeedInput, typeDef model.TypeDef) (map[string]string, error) {
	fieldKeys := map[string]string{}
	for fieldName, fieldDef := range typeDef.Fields {
		if fieldDef.Required {
			fieldKeys[fieldName] = c.inferDotenvKey(typeDef, need.Instance, fieldName, fieldDef)
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

func (c *ConfigCompiler) inferDotenvKey(typeDef model.TypeDef, instance string, fieldName string, fieldDef model.FieldDef) string {
	if instance == "default" {
		if fieldDef.PreferredDotenvKey != "" {
			return fieldDef.PreferredDotenvKey
		}
		return upperSnake(typeDef.Name + "_" + fieldName)
	}
	return upperSnake(instance + "_" + typeDef.Name + "_" + fieldName)
}

func sortedFieldNames(fieldKeys map[string]string) []string {
	fields := make([]string, 0, len(fieldKeys))
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
	return fields
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
