package graph

import "github.com/graphql-go/graphql"

type graphInputField struct {
	name string
	typ  graphql.Input
}

func graphInput(name string, typ graphql.Input) graphInputField {
	return graphInputField{name: name, typ: typ}
}

func graphInputObject(name string, fields ...graphInputField) *graphql.InputObject {
	fieldMap := make(graphql.InputObjectConfigFieldMap, len(fields))
	for _, field := range fields {
		fieldMap[field.name] = &graphql.InputObjectFieldConfig{Type: field.typ}
	}
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   name,
		Fields: fieldMap,
	})
}
