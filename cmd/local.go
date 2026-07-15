package cmd

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/runmedev/owl/pkg/owl"
)

type LocalStoreOptions struct {
	EnvFiles  []string
	SpecFiles []string
}

type LocalStoreClient struct {
	options LocalStoreOptions
}

func NewLocalCommands() []*cobra.Command {
	var options LocalStoreOptions

	configureLocalFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringArrayVar(&options.EnvFiles, "env-file", nil, "Env file to load")
		cmd.Flags().StringArrayVar(&options.SpecFiles, "spec-file", nil, "Env spec file to load")
	}

	return NewStoreCommands(StoreCommandOptions{
		ClientFactory: func(cmd *cobra.Command) (StoreClient, error) {
			return NewLocalStoreClient(options), nil
		},
		ConfigureSnapshotCommand: configureLocalFlags,
		ConfigureSourceCommand:   configureLocalFlags,
		ConfigureCheckCommand:    configureLocalFlags,
		InsecureAllowed:          func() bool { return true },
	})
}

func NewLocalStoreClient(options LocalStoreOptions) *LocalStoreClient {
	return &LocalStoreClient{options: options}
}

func (c *LocalStoreClient) Snapshot(context.Context, SnapshotRequest) (*SnapshotResult, error) {
	store, err := c.store()
	if err != nil {
		return nil, err
	}

	items, err := store.Snapshot()
	if err != nil {
		return nil, err
	}

	return &SnapshotResult{Envs: snapshotEnvsFromItems(items)}, nil
}

func (c *LocalStoreClient) Source(context.Context, SourceRequest) (*SourceResult, error) {
	store, err := c.store()
	if err != nil {
		return nil, err
	}

	envs, err := store.InsecureValues()
	if err != nil {
		return nil, err
	}

	return &SourceResult{Envs: envs}, nil
}

func (c *LocalStoreClient) Check(context.Context, CheckRequest) (*CheckResult, error) {
	store, err := c.store()
	if err != nil {
		return nil, err
	}

	if _, err := store.Snapshot(); err != nil {
		return nil, err
	}

	return &CheckResult{Message: "Success"}, nil
}

func (c *LocalStoreClient) store() (*owl.Store, error) {
	var opts []owl.StoreOption

	specFiles, err := filesOrDefaults(c.options.SpecFiles, ".env.example")
	if err != nil {
		return nil, err
	}
	for _, file := range specFiles {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		opts = append(opts, owl.WithSpecFile(file, raw))
	}

	envFiles, err := filesOrDefaults(c.options.EnvFiles, ".env")
	if err != nil {
		return nil, err
	}
	for _, file := range envFiles {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		opts = append(opts, owl.WithEnvFile(file, raw))
	}

	return owl.NewStore(opts...)
}

func filesOrDefaults(files []string, defaults ...string) ([]string, error) {
	if len(files) > 0 {
		return files, nil
	}

	var existing []string
	for _, file := range defaults {
		if _, err := os.Stat(file); err == nil {
			existing = append(existing, file)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	return existing, nil
}

func snapshotEnvsFromItems(items owl.SetVarItems) []SnapshotEnv {
	envs := make([]SnapshotEnv, 0, len(items))
	for _, item := range items {
		env := SnapshotEnv{}
		if item.Var != nil {
			env.Name = item.Var.Key
			env.Origin = item.Var.Origin
			if item.Var.Updated != nil {
				env.UpdateTime = item.Var.Updated.Format(time.RFC3339)
			}
		}
		if item.Value != nil {
			env.OriginalValue = item.Value.Original
			env.ResolvedValue = item.Value.Resolved
			env.Status = item.Value.Status
			if env.Status == "" {
				env.Status = "UNSPECIFIED"
			}
		}
		if item.Spec != nil {
			env.Description = item.Spec.Description
			env.Spec = item.Spec.Name
		}
		envs = append(envs, env)
	}
	return envs
}
