package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type StoreClient interface {
	Snapshot(context.Context, SnapshotRequest) (*SnapshotResult, error)
	Source(context.Context, SourceRequest) (*SourceResult, error)
	Check(context.Context, CheckRequest) (*CheckResult, error)
	GraphQLSchema(context.Context, GraphQLSchemaRequest) (*GraphQLSchemaResult, error)
}

type StoreCommandOptions struct {
	ClientFactory            func(*cobra.Command) (StoreClient, error)
	ConfigureSnapshotCommand func(*cobra.Command)
	ConfigureSourceCommand   func(*cobra.Command)
	ConfigureCheckCommand    func(*cobra.Command)
	Hidden                   bool
	InsecureAllowed          func() bool
}

type SnapshotRequest struct {
	Limit  int
	Reveal bool
	All    bool
}

type SnapshotResult struct {
	Envs []SnapshotEnv
}

type SnapshotEnv struct {
	Name        string
	Value       string
	Description string
	Type        string
	Field       string
	Source      string
	Status      string
	Diagnostics []string
}

type SourceRequest struct {
	Export   bool
	Insecure bool
}

type SourceResult struct {
	Envs []string
}

type CheckRequest struct{}

type CheckResult struct {
	OK          bool
	Diagnostics []string
}

type GraphQLSchemaRequest struct{}

type GraphQLSchemaResult struct {
	Schema string
}

func NewStoreCommand(opts StoreCommandOptions) *cobra.Command {
	if opts.InsecureAllowed == nil {
		opts.InsecureAllowed = func() bool { return false }
	}

	cmd := cobra.Command{
		Hidden: opts.Hidden,
		Use:    "store",
		Short:  "Owl store",
		Long:   "Owl Store",
	}

	cmd.AddCommand(NewStoreCommands(opts)...)

	return &cmd
}

func NewStoreCommands(opts StoreCommandOptions) []*cobra.Command {
	if opts.InsecureAllowed == nil {
		opts.InsecureAllowed = func() bool { return false }
	}

	return []*cobra.Command{
		newSnapshotCommand(opts),
		newSourceCommand(opts),
		newCheckCommand(opts),
		newGraphQLSchemaCommand(opts),
	}
}

func newSnapshotCommand(opts StoreCommandOptions) *cobra.Command {
	var req SnapshotRequest

	cmd := cobra.Command{
		Hidden: opts.Hidden,
		Use:    "snapshot",
		Short:  "Takes a snapshot of the smart env store",
		Long:   "Inspects environment variables and returns a snapshot of the smart env store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if req.Reveal && !opts.InsecureAllowed() {
				return errors.New("must be run in insecure mode to prevent misuse; enable by adding --insecure flag")
			}

			client, err := opts.client(cmd)
			if err != nil {
				return err
			}

			result, err := client.Snapshot(cmd.Context(), req)
			if err != nil {
				return err
			}

			return errors.Wrap(renderSnapshot(cmd.OutOrStdout(), result, req), "failed to render")
		},
	}

	cmd.Flags().IntVar(&req.Limit, "limit", 50, "Limit the number of lines")
	cmd.Flags().BoolVarP(&req.All, "all", "A", false, "Show all lines")
	cmd.Flags().BoolVarP(&req.Reveal, "reveal", "r", false, "Reveal hidden values")
	if opts.ConfigureSnapshotCommand != nil {
		opts.ConfigureSnapshotCommand(&cmd)
	}

	return &cmd
}

func newSourceCommand(opts StoreCommandOptions) *cobra.Command {
	var req SourceRequest

	cmd := cobra.Command{
		Use:   "source",
		Short: "Source environment variables",
		Long:  "Source environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !req.Insecure {
				return errors.New("must be run in insecure mode to prevent misuse; enable by adding --insecure flag")
			}

			client, err := opts.client(cmd)
			if err != nil {
				return err
			}

			result, err := client.Source(cmd.Context(), req)
			if err != nil {
				return err
			}

			return renderSource(cmd.OutOrStdout(), result, req)
		},
	}

	cmd.Flags().BoolVarP(&req.Export, "export", "", false, "export variables")
	cmd.Flags().BoolVar(&req.Insecure, "insecure", false, "Explicitly allow delicate operations to prevent misuse")
	if opts.ConfigureSourceCommand != nil {
		opts.ConfigureSourceCommand(&cmd)
	}

	return &cmd
}

func newCheckCommand(opts StoreCommandOptions) *cobra.Command {
	cmd := cobra.Command{
		Hidden: opts.Hidden,
		Use:    "check",
		Short:  "Validates smart store",
		Long:   "Validates smart store, exiting with success or displaying errors.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.client(cmd)
			if err != nil {
				return err
			}

			result, err := client.Check(cmd.Context(), CheckRequest{})
			if err != nil {
				return err
			}

			if len(result.Diagnostics) == 0 {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "Success")
				return err
			}
			for _, diagnostic := range result.Diagnostics {
				if _, err = fmt.Fprintln(cmd.OutOrStdout(), diagnostic); err != nil {
					return err
				}
			}
			if !result.OK {
				return errors.New("owl store check failed")
			}
			return nil
		},
	}
	if opts.ConfigureCheckCommand != nil {
		opts.ConfigureCheckCommand(&cmd)
	}

	return &cmd
}

func newGraphQLSchemaCommand(opts StoreCommandOptions) *cobra.Command {
	cmd := cobra.Command{
		Hidden: true,
		Use:    "graphql-schema",
		Short:  "Prints the hidden Owl GraphQL schema",
		Long:   "Prints the hidden Owl GraphQL schema introspection JSON.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.client(cmd)
			if err != nil {
				return err
			}

			result, err := client.GraphQLSchema(cmd.Context(), GraphQLSchemaRequest{})
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), result.Schema)
			return err
		},
	}

	return &cmd
}

func (opts StoreCommandOptions) client(cmd *cobra.Command) (StoreClient, error) {
	if opts.ClientFactory == nil {
		return nil, errors.New("store client factory is required")
	}
	return opts.ClientFactory(cmd)
}

func renderSnapshot(w io.Writer, result *SnapshotResult, req SnapshotRequest) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tVALUE\tTYPE\tSOURCE\tSTATUS\tDESCRIPTION"); err != nil {
		return err
	}

	lines := req.Limit
	for i, env := range result.Envs {
		if i >= lines && !req.All {
			break
		}

		strippedVal := strings.ReplaceAll(strings.ReplaceAll(env.Value, "\n", " "), "\r", "")
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			env.Name,
			strippedVal,
			env.Type,
			env.Source,
			env.Status,
			env.Description,
		); err != nil {
			return err
		}
	}

	return tw.Flush()
}

func renderSource(w io.Writer, result *SourceResult, req SourceRequest) error {
	for _, kv := range result.Envs {
		parts := strings.Split(kv, "=")
		if len(parts) < 2 {
			return errors.Errorf("invalid key-value pair: %s", kv)
		}

		envVar := fmt.Sprintf("%s=%q", parts[0], strings.Join(parts[1:], "="))
		if req.Export {
			envVar = fmt.Sprintf("export %s", envVar)
		}

		if _, err := fmt.Fprintf(w, "%s\n", envVar); err != nil {
			return err
		}
	}

	return nil
}
