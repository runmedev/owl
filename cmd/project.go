package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newProjectCommand(opts StoreCommandOptions) *cobra.Command {
	cmd := cobra.Command{
		Use:   "project",
		Short: "Project Owl config into adapter artifacts",
		Long:  "Project Owl config into adapter artifacts.",
	}
	cmd.AddCommand(newProjectSpecCommand(opts))
	return &cmd
}

func newProjectSpecCommand(opts StoreCommandOptions) *cobra.Command {
	var req ProjectSpecRequest

	cmd := cobra.Command{
		Use:   "spec",
		Short: "Project Owl config into dotenv spec",
		Long:  "Project Owl config into a generated dotenv spec.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if req.Write && req.Output != "" {
				return errors.New("use either --write or --output, not both")
			}
			client, err := opts.client(cmd)
			if err != nil {
				return err
			}
			result, err := client.ProjectSpec(cmd.Context(), req)
			if err != nil {
				return err
			}
			if req.Write || (req.Output != "" && req.Output != "-") {
				return nil
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), result.Rendered)
			return err
		},
	}

	cmd.Flags().StringVar(&req.ConfigPath, "config", "", "Owl config file to load")
	cmd.Flags().StringVar(&req.Output, "output", "", "Write generated dotenv spec to a file, or '-' for stdout")
	cmd.Flags().BoolVar(&req.Write, "write", false, "Write generated dotenv spec to .env.spec")
	if opts.ConfigureProjectCommand != nil {
		opts.ConfigureProjectCommand(&cmd)
	}

	return &cmd
}
