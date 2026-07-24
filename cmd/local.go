package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"

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
		ConfigureTypeCommand:     configureLocalFlags,
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

func (c *LocalStoreClient) Type(_ context.Context, req TypeRequest) (*TypeResult, error) {
	if req.SpecPath == "" {
		req.SpecPath = ".env.spec"
	}
	options := c.options
	options.SpecFiles = []string{req.SpecPath}
	store, err := NewLocalStoreClient(options).storeWithOptions(true)
	if err != nil {
		return nil, err
	}

	result, err := store.Type(owl.TypePolicy{All: req.All})
	if err != nil {
		return nil, err
	}

	rendered := renderDotenvSpecTypeProposals(result.Proposals)
	if req.Output != "" {
		if err := os.WriteFile(req.Output, []byte(rendered), 0o600); err != nil {
			return nil, err
		}
	}
	if req.Fix {
		if err := appendDotenvSpecTypeProposals(req.SpecPath, rendered); err != nil {
			return nil, err
		}
	}

	return &TypeResult{
		Proposals: typeProposalsFromItems(result.Proposals),
		Rendered:  rendered,
	}, nil
}

func (c *LocalStoreClient) store() (*owl.Store, error) {
	return c.storeWithOptions(false)
}

func (c *LocalStoreClient) storeWithOptions(allowMissingSpec bool) (*owl.Store, error) {
	var opts []owl.StoreOption

	specFiles, err := filesOrDefaults(c.options.SpecFiles, ".env.example")
	if err != nil {
		return nil, err
	}
	for _, file := range specFiles {
		raw, err := os.ReadFile(file)
		if allowMissingSpec && errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		opts = append(opts, owl.WithEnvSpec(file, bytes.NewReader(raw)))
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
		opts = append(opts, owl.WithDotenv(file, bytes.NewReader(raw)))
	}

	return owl.NewStore(opts...)
}

func renderDotenvSpecTypeProposals(proposals []owl.TypeProposal) string {
	if len(proposals) == 0 {
		return ""
	}
	var b strings.Builder
	for _, proposal := range proposals {
		if proposal.SuggestedType == "" {
			continue
		}
		_, _ = b.WriteString(proposal.Key)
		_, _ = b.WriteString("=")
		_, _ = b.WriteString(strconvQuote(proposal.Description))
		_, _ = b.WriteString(" # ")
		_, _ = b.WriteString(dotenvSpecName(proposal.SuggestedType))
		if proposal.Required {
			_ = b.WriteByte('!')
		}
		_ = b.WriteByte('\n')
	}
	return b.String()
}

func appendDotenvSpecTypeProposals(path string, rendered string) error {
	if rendered == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var b strings.Builder
	_, _ = b.Write(raw)
	if len(raw) > 0 && !strings.HasSuffix(string(raw), "\n") {
		_ = b.WriteByte('\n')
	}
	_, _ = b.WriteString(rendered)
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func dotenvSpecName(typeID owl.TypeID) string {
	switch typeID {
	case owl.TypeCoreSecret:
		return "Secret"
	case owl.TypeCoreURL:
		return "Url"
	case owl.TypeCoreHost:
		return "Host"
	case owl.TypeCorePort:
		return "Port"
	case owl.TypeCorePlain:
		return "Plain"
	default:
		return "Opaque"
	}
}

func strconvQuote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
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

func typeProposalsFromItems(items []owl.TypeProposal) []TypeProposal {
	proposals := make([]TypeProposal, 0, len(items))
	for _, item := range items {
		suggestedType := "-"
		if item.SuggestedType != "" {
			suggestedType = item.SuggestedType.Alias()
		}
		proposals = append(proposals, TypeProposal{
			Key:           item.Key,
			CurrentType:   item.CurrentType.Alias(),
			SuggestedType: suggestedType,
			Confidence:    string(item.Confidence),
			Reason:        item.Reason,
			Description:   item.Description,
			Required:      item.Required,
		})
	}
	return proposals
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
			Explicit:    item.Explicit,
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
