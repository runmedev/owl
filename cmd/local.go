package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/runmedev/owl/internal/requirements"
	"github.com/runmedev/owl/pkg/owl"
)

type LocalStoreOptions struct {
	EnvFiles   []string
	SpecFiles  []string
	ConfigPath string
}

type LocalStoreClient struct {
	options LocalStoreOptions
}

func NewLocalCommands() []*cobra.Command {
	var options LocalStoreOptions

	configureLocalFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringArrayVar(&options.EnvFiles, "env-file", nil, "Env file to load")
		cmd.Flags().StringArrayVar(&options.SpecFiles, "spec-file", nil, "Env spec file to load")
		cmd.Flags().StringVar(&options.ConfigPath, "config", "", "Owl config file to load")
	}
	configureTypeFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringArrayVar(&options.EnvFiles, "env-file", nil, "Env file to load")
		cmd.Flags().StringArrayVar(&options.SpecFiles, "spec-file", nil, "Env spec file to load")
	}

	opts := StoreCommandOptions{
		ClientFactory: func(cmd *cobra.Command) (StoreClient, error) {
			return NewLocalStoreClient(options), nil
		},
		ConfigureSnapshotCommand: configureLocalFlags,
		ConfigureSourceCommand:   configureLocalFlags,
		ConfigureCheckCommand:    configureLocalFlags,
		ConfigureTypeCommand:     configureTypeFlags,
		InsecureAllowed:          func() bool { return true },
	}

	commands := NewStoreCommands(opts)
	commands = append(commands, newProjectCommand(opts))
	return commands
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
	store, err := NewLocalStoreClient(options).storeWithOptions(true, false)
	if err != nil {
		return nil, err
	}

	result, err := store.Type(owl.TypePolicy{All: req.All})
	if err != nil {
		return nil, err
	}

	proposals := renderDotenvSpecTypeProposals(result.Proposals)
	rendered := proposals
	if req.Output != "" {
		materialized, err := materializeDotenvSpecTypeProposals(req.SpecPath, proposals)
		if err != nil {
			return nil, err
		}
		rendered = materialized
		if req.Output != "-" {
			if err := os.WriteFile(req.Output, []byte(materialized), 0o600); err != nil {
				return nil, err
			}
		}
	}
	if req.Fix {
		if err := appendDotenvSpecTypeProposals(req.SpecPath, proposals); err != nil {
			return nil, err
		}
	}

	return &TypeResult{
		Proposals: typeProposalsFromItems(result.Proposals),
		Rendered:  rendered,
	}, nil
}

func (c *LocalStoreClient) store() (*owl.Store, error) {
	return c.storeWithOptions(false, true)
}

func (c *LocalStoreClient) storeWithOptions(allowMissingSpec bool, loadConfig bool) (*owl.Store, error) {
	var opts []owl.StoreOption

	var configPath string
	if loadConfig {
		path, err := resolveConfigPath(c.options.ConfigPath, false)
		if err != nil {
			return nil, err
		}
		configPath = path
		if configPath != "" {
			input, err := readConfigFile(configPath)
			if err != nil {
				return nil, err
			}
			opts = append(opts, owl.WithConfig(input))
		}
	}

	if configPath != "" {
		if err := validateNoHumanDotenvSpecs(c.options.SpecFiles); err != nil {
			return nil, err
		}
	}

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
		if configPath != "" {
			if isGeneratedDotenvSpec(raw) {
				continue
			}
			return nil, errors.New("dotenv spec file exists beside Owl config; move it aside or regenerate it with owl project spec --write")
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

func validateNoHumanDotenvSpecs(specFiles []string) error {
	files := specFiles
	if len(files) == 0 {
		files = []string{".env.spec", ".env.example"}
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !isGeneratedDotenvSpec(raw) {
			return errors.New("dotenv spec file exists beside Owl config; move it aside or regenerate it with owl project spec --write")
		}
	}
	return nil
}

func (c *LocalStoreClient) ProjectSpec(_ context.Context, req ProjectSpecRequest) (*ProjectSpecResult, error) {
	configPath, err := resolveConfigPath(req.ConfigPath, true)
	if err != nil {
		return nil, err
	}
	input, err := readConfigFile(configPath)
	if err != nil {
		return nil, err
	}
	store, err := owl.NewStore(owl.WithConfig(input))
	if err != nil {
		return nil, err
	}
	rendered, err := store.DotenvSpec()
	if err != nil {
		return nil, err
	}
	output := req.Output
	if req.Write {
		output = ".env.spec"
	}
	if output != "" && output != "-" {
		if err := writeGeneratedDotenvSpec(output, rendered); err != nil {
			return nil, err
		}
	}
	return &ProjectSpecResult{Rendered: rendered}, nil
}

func resolveConfigPath(explicit string, required bool) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", err
		}
		return explicit, nil
	}
	var found []string
	for _, candidate := range []string{"owl.toml", "owl.yaml", "owl.yml", "owl.json"} {
		if _, err := os.Stat(candidate); err == nil {
			found = append(found, candidate)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	if len(found) > 1 {
		return "", errors.New("multiple Owl config files found; pass --config <path>")
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if required {
		return "", errors.New("owl config not found; pass --config <path> or create owl.toml")
	}
	return "", nil
}

func readConfigFile(path string) (owl.ConfigInput, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return owl.ConfigInput{}, err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		var input owl.ConfigInput
		if err := json.Unmarshal(raw, &input); err != nil {
			return owl.ConfigInput{}, err
		}
		return input, nil
	case ".yaml", ".yml":
		var input owl.ConfigInput
		if err := yaml.Unmarshal(raw, &input); err != nil {
			return owl.ConfigInput{}, err
		}
		return input, nil
	case ".toml":
		return parseTomlConfig(raw)
	default:
		return owl.ConfigInput{}, errors.New("unsupported Owl config extension; use .toml, .yaml, .yml, or .json")
	}
}

type tomlConfig struct {
	Needs map[string]map[string]tomlNeed `toml:"needs"`
}

type tomlNeed struct {
	Type   string            `toml:"type"`
	Dotenv map[string]string `toml:"dotenv"`
}

func parseTomlConfig(raw []byte) (owl.ConfigInput, error) {
	var cfg tomlConfig
	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return owl.ConfigInput{}, err
	}
	var kinds []string
	for kind := range cfg.Needs {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	var input owl.ConfigInput
	for _, kind := range kinds {
		var instances []string
		for instance := range cfg.Needs[kind] {
			instances = append(instances, instance)
		}
		sort.Strings(instances)
		for _, instance := range instances {
			need := cfg.Needs[kind][instance]
			inputNeed := owl.NeedInput{
				ID:       kind + "." + instance,
				Type:     owl.TypeID(need.Type),
				Instance: instance,
			}
			if len(need.Dotenv) > 0 {
				var fields []string
				for field := range need.Dotenv {
					fields = append(fields, field)
				}
				sort.Strings(fields)
				inputNeed.Dotenv = &owl.DotenvProjection{}
				for _, field := range fields {
					inputNeed.Dotenv.Fields = append(inputNeed.Dotenv.Fields, owl.DotenvFieldBinding{
						Field: field,
						Key:   need.Dotenv[field],
					})
				}
			}
			input.Needs = append(input.Needs, inputNeed)
		}
	}
	return input, nil
}

func writeGeneratedDotenvSpec(path string, rendered string) error {
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && !isGeneratedDotenvSpec(raw) {
		return errors.New("dotenv spec file already exists and is not Owl-generated; move it aside or use --output <path>")
	}
	return os.WriteFile(path, []byte(rendered), 0o600)
}

func isGeneratedDotenvSpec(raw []byte) bool {
	return strings.HasPrefix(string(raw), requirements.GeneratedDotenvSpecHeader)
}

func renderDotenvSpecTypeProposals(proposals []owl.TypeProposal) string {
	if len(proposals) == 0 {
		return ""
	}
	type renderedProposal struct {
		left     string
		typeName string
		required bool
	}
	rendered := make([]renderedProposal, 0, len(proposals))
	maxLeftWidth := 0
	for _, proposal := range proposals {
		if proposal.SuggestedType == "" {
			continue
		}
		left := proposal.Key + "=" + strconvQuote(proposal.Description)
		rendered = append(rendered, renderedProposal{
			left:     left,
			typeName: dotenvSpecName(proposal.SuggestedType),
			required: proposal.Required,
		})
		if len(left) > maxLeftWidth {
			maxLeftWidth = len(left)
		}
	}
	if len(rendered) == 0 {
		return ""
	}
	var b strings.Builder
	for _, proposal := range rendered {
		_, _ = b.WriteString(proposal.left)
		_, _ = b.WriteString(strings.Repeat(" ", maxLeftWidth-len(proposal.left)+1))
		_, _ = b.WriteString("# ")
		_, _ = b.WriteString(proposal.typeName)
		if proposal.required {
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
	materialized, err := materializeDotenvSpecTypeProposals(path, rendered)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(materialized), 0o600)
}

func materializeDotenvSpecTypeProposals(path string, rendered string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	var b strings.Builder
	_, _ = b.Write(raw)
	if len(raw) > 0 {
		switch {
		case strings.HasSuffix(string(raw), "\n\n"):
		case strings.HasSuffix(string(raw), "\n"):
			_ = b.WriteByte('\n')
		default:
			_, _ = b.WriteString("\n\n")
		}
	}
	_, _ = b.WriteString(rendered)
	return b.String(), nil
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
