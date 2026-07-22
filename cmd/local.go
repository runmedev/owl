package cmd

import (
	"context"
	"errors"
	"os"

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

func (c *LocalStoreClient) Snapshot(_ context.Context, req SnapshotRequest) (*SnapshotResult, error) {
	store, err := c.store()
	if err != nil {
		return nil, err
	}

	items, err := store.Snapshot(owl.SnapshotPolicy{Reveal: req.Reveal})
	if err != nil {
		return nil, err
	}

	return &SnapshotResult{Envs: snapshotEnvsFromItems(items)}, nil
}

func (c *LocalStoreClient) Source(_ context.Context, req SourceRequest) (*SourceResult, error) {
	store, err := c.store()
	if err != nil {
		return nil, err
	}

	envs, err := store.Dotenv(owl.DotenvPolicy{Insecure: req.Insecure})
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

	check := store.Check()
	return &CheckResult{
		OK:          check.OK,
		Diagnostics: diagnosticStrings(check.Diagnostics),
	}, nil
}

func (c *LocalStoreClient) store() (*owl.Store, error) {
	var opts []owl.StoreOption
	var opened []*os.File
	closeOpened := func() error {
		var closeErr error
		for _, f := range opened {
			if err := f.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		return closeErr
	}

	specFiles, err := filesOrDefaults(c.options.SpecFiles, ".env.example")
	if err != nil {
		return nil, err
	}
	for _, file := range specFiles {
		f, err := os.Open(file)
		if err != nil {
			_ = closeOpened()
			return nil, err
		}
		opened = append(opened, f)
		opts = append(opts, owl.WithEnvSpec(file, f))
	}

	envFiles, err := filesOrDefaults(c.options.EnvFiles, ".env")
	if err != nil {
		_ = closeOpened()
		return nil, err
	}
	for _, file := range envFiles {
		f, err := os.Open(file)
		if err != nil {
			_ = closeOpened()
			return nil, err
		}
		opened = append(opened, f)
		opts = append(opts, owl.WithDotenv(file, f))
	}

	store, err := owl.NewStore(opts...)
	if closeErr := closeOpened(); err == nil && closeErr != nil {
		return nil, closeErr
	}
	return store, err
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

func snapshotEnvsFromItems(items []owl.SnapshotItem) []SnapshotEnv {
	envs := make([]SnapshotEnv, 0, len(items))
	for _, item := range items {
		visibility := string(item.Visibility)
		if visibility == "" {
			visibility = "UNSPECIFIED"
		}
		envs = append(envs, SnapshotEnv{
			Name:        item.Name,
			Value:       item.Value,
			Description: item.Description,
			Type:        item.Type.Alias(),
			Field:       item.Field.String(),
			Source:      item.Source.Name,
			Visibility:  visibility,
			Diagnostics: diagnosticStrings(item.Diagnostics),
		})
	}
	return envs
}

func diagnosticStrings(diagnostics []owl.Diagnostic) []string {
	result := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		result = append(result, diagnosticString(diagnostic))
	}
	return result
}

func diagnosticString(diagnostic owl.Diagnostic) string {
	if diagnostic.Key != "" {
		return string(diagnostic.Severity) + " " + diagnostic.Code + " " + diagnostic.Key + ": " + diagnostic.Message
	}
	if diagnostic.FieldRef.TypeID != "" {
		return string(diagnostic.Severity) + " " + diagnostic.Code + " " + diagnostic.FieldRef.String() + ": " + diagnostic.Message
	}
	return string(diagnostic.Severity) + " " + diagnostic.Code + ": " + diagnostic.Message
}
