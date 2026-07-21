package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/runmedev/owl/pkg/owl"
)

func NewDebugCommand() *cobra.Command {
	cmd := cobra.Command{
		Hidden: true,
		Use:    "debug",
		Short:  "Debug Owl internals",
		Long:   "Debug Owl internals.",
	}

	cmd.AddCommand(newDebugGraphQLSchemaCommand())
	return &cmd
}

func newDebugGraphQLSchemaCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "graphql-schema",
		Short: "Prints the internal Owl GraphQL schema",
		Long:  "Prints the internal Owl GraphQL schema introspection JSON.",
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, err := owl.GraphQLSchema()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), schema)
			return err
		},
	}

	return &cmd
}
