package cmd

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/runmedev/owl/pkg/owl"
	"github.com/runmedev/owl/pkg/owl/seed"
)

type LocalStoreOptions struct {
	EnvFiles     []string
	SpecFiles    []string
	ConfigPath   string
	ProcessEnv   []string
	Direnv       seed.DirenvPolicy
	DirenvDir    string
	DirenvRunner seed.DirenvExportRunner
	TypeProvider owl.TypeProvider
}

type LocalStoreClient struct {
	options               LocalStoreOptions
	lastStore             *owl.Store
	lastSourceDiagnostics []owl.Diagnostic
}

var processEnviron = os.Environ

func NewLocalCommands() []*cobra.Command {
	options := LocalStoreOptions{Direnv: seed.DirenvEnabledWarn}

	configureLocalFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringArrayVar(&options.EnvFiles, "env-file", nil, "Env file to load")
		cmd.Flags().StringArrayVar(&options.SpecFiles, "spec-file", nil, "Env spec file to load")
		cmd.Flags().StringVar(&options.ConfigPath, "config", "", "Owl config file to load")
		cmd.Flags().Var(&options.Direnv, "direnv", "Direnv policy: disabled, enabled_warn, enabled_error")
	}
	configureTypeFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringArrayVar(&options.EnvFiles, "env-file", nil, "Env file to load")
		cmd.Flags().StringArrayVar(&options.SpecFiles, "spec-file", nil, "Env spec file to load")
	}

	opts := StoreCommandOptions{
		ClientFactory: func(cmd *cobra.Command) (StoreClient, error) {
			types, err := commandTypeProvider()
			if err != nil {
				return nil, err
			}
			clientOptions := options
			clientOptions.TypeProvider = types
			return NewLocalStoreClient(clientOptions), nil
		},
		ConfigureSnapshotCommand:   configureLocalFlags,
		ConfigureSourceCommand:     configureLocalFlags,
		ConfigureCheckCommand:      configureLocalFlags,
		ConfigureTypeCommand:       configureTypeFlags,
		ConfigureResolveCommand:    configureLocalFlags,
		InsecureModeEnabled:        func() bool { return true },
		DefineSnapshotInsecureFlag: true,
	}

	commands := NewStoreCommands(opts)
	commands = append(commands, newProjectCommand(opts))
	return commands
}

func NewLocalStoreClient(options LocalStoreOptions) *LocalStoreClient {
	return &LocalStoreClient{options: options}
}

func (c *LocalStoreClient) Snapshot(ctx context.Context, req SnapshotRequest) (*SnapshotResult, error) {
	store, err := c.resolvedStore(ctx, req.Interactive)
	if err != nil {
		return nil, err
	}

	snapshot, err := store.Snapshot(ctx, owl.SnapshotInput{
		Policy: owl.SnapshotPolicy{Reveal: req.Reveal && req.Insecure},
		Filter: owl.SnapshotFilter{All: req.All, Limit: req.Limit},
	})
	if err != nil {
		return nil, err
	}

	return &SnapshotResult{Envs: snapshotEnvsFromItems(snapshot.Envs)}, nil
}

func (c *LocalStoreClient) Source(ctx context.Context, req SourceRequest) (*SourceResult, error) {
	store, err := c.resolvedStore(ctx, req.Interactive)
	if err != nil {
		return nil, err
	}

	source, err := store.Source(ctx, owl.SourceInput{
		Policy: owl.DotenvPolicy{Insecure: req.Insecure},
	})
	if err != nil {
		return nil, err
	}

	return &SourceResult{Envs: source.Envs}, nil
}

func (c *LocalStoreClient) Check(ctx context.Context, req CheckRequest) (*CheckResult, error) {
	store, err := c.resolvedStore(ctx, req.Interactive)
	if err != nil {
		return nil, err
	}

	check, err := store.Check(ctx, owl.CheckInput{})
	if err != nil {
		return nil, err
	}
	return &CheckResult{
		OK:          check.OK,
		Diagnostics: append(diagnosticStrings(c.lastSourceDiagnostics, req.Details), diagnosticStrings(check.Diagnostics, req.Details)...),
		Checked:     check.Checked,
	}, nil
}

func (c *LocalStoreClient) Type(ctx context.Context, req TypeRequest) (*TypeResult, error) {
	if req.SpecPath == "" {
		req.SpecPath = ".env.spec"
	}
	options := c.options
	options.SpecFiles = []string{req.SpecPath}
	store, err := seed.NewRawValueStore(seedOptions(options), true)
	if err != nil {
		return nil, err
	}

	result, err := store.Type(ctx, owl.TypeInput{Policy: owl.TypePolicy{All: req.All}})
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

func (c *LocalStoreClient) Resolve(ctx context.Context, req ResolveRequest) (*ResolveResult, error) {
	store, sources, err := c.baseStoreWithSources(ctx)
	if err != nil {
		return nil, err
	}
	c.lastStore = store
	result, err := store.Resolve(ctx, owl.ResolveInput{
		Process: sources.ProcessResolverInput(),
		Dotenv:  sources.DotenvResolverInput(),
		Policy:  owl.ResolvePolicy{AllowInteraction: req.Interactive},
	})
	if err != nil {
		return nil, err
	}
	return resolveResultFromOwl(result), nil
}

func (c *LocalStoreClient) ApplyPromptAnswers(ctx context.Context, answers []PromptAnswer) (*ResolveResult, error) {
	if c.lastStore == nil {
		store, err := c.baseStore(ctx)
		if err != nil {
			return nil, err
		}
		c.lastStore = store
	}
	owlAnswers := make([]owl.PromptAnswer, 0, len(answers))
	for _, answer := range answers {
		owlAnswers = append(owlAnswers, owl.PromptAnswer{
			NeedID: owl.UnresolvedNeedID(answer.NeedID),
			Value:  answer.Value,
		})
	}
	result, err := c.lastStore.ApplyPromptAnswers(ctx, owlAnswers)
	if err != nil {
		return nil, err
	}
	return resolveResultFromOwl(result), nil
}

func (c *LocalStoreClient) baseStore(ctx context.Context) (*owl.Store, error) {
	store, _, err := c.baseStoreWithSources(ctx)
	return store, err
}

func (c *LocalStoreClient) baseStoreWithSources(ctx context.Context) (*owl.Store, seed.Catalog, error) {
	result, err := seed.NewStore(ctx, seedOptions(c.options))
	if err != nil {
		return nil, seed.Catalog{}, err
	}
	c.lastSourceDiagnostics = result.Diagnostics
	return result.Store, result.Catalog, nil
}

func (c *LocalStoreClient) resolvedStore(ctx context.Context, interactive bool) (*owl.Store, error) {
	if interactive && c.lastStore != nil {
		return c.lastStore, nil
	}
	store, sources, err := c.baseStoreWithSources(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := store.Resolve(ctx, owl.ResolveInput{
		Process: sources.ProcessResolverInput(),
		Dotenv:  sources.DotenvResolverInput(),
		Policy:  owl.ResolvePolicy{AllowInteraction: interactive},
	}); err != nil {
		return nil, err
	}
	return store, nil
}

func seedOptions(options LocalStoreOptions) seed.Options {
	return seed.Options{
		EnvFiles:     options.EnvFiles,
		SpecFiles:    options.SpecFiles,
		ConfigPath:   options.ConfigPath,
		Observed:     []seed.ObservedSource{{Source: owl.Source{Name: "[process]", Kind: "process"}, Environ: processEnvForOptions(options)}},
		WorkDir:      options.DirenvDir,
		Direnv:       options.Direnv,
		DirenvRunner: options.DirenvRunner,
		TypeProvider: options.TypeProvider,
	}
}

func processEnvForOptions(options LocalStoreOptions) []string {
	environ := options.ProcessEnv
	if environ == nil {
		environ = processEnviron()
	}
	filtered := make([]string, 0, len(environ))
	for _, item := range environ {
		key, _, _ := strings.Cut(item, "=")
		if key != cueRootEnv {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (c *LocalStoreClient) ProjectSpec(ctx context.Context, req ProjectSpecRequest) (*ProjectSpecResult, error) {
	configPath, err := resolveConfigPath(req.ConfigPath, true)
	if err != nil {
		return nil, err
	}
	types := c.options.TypeProvider
	if types == nil {
		types = owl.NewBuiltInTypeProvider()
	}
	storeOptions := []owl.StoreOption{owl.WithTypeProvider(types), owl.WithConfigFile(configPath)}
	store, err := owl.NewStore(storeOptions...)
	if err != nil {
		return nil, err
	}
	rendered, err := store.ProjectSpec(ctx, owl.ProjectSpecInput{
		Load: owl.LoadInput{},
	})
	if err != nil {
		return nil, err
	}
	outputText := rendered.Rendered
	output := req.Output
	if req.Write {
		output = ".env.spec"
	}
	if output != "" && output != "-" {
		if err := writeGeneratedDotenvSpec(output, outputText); err != nil {
			return nil, err
		}
	}
	return &ProjectSpecResult{Rendered: outputText}, nil
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
	return strings.HasPrefix(string(raw), owl.GeneratedDotenvSpecHeaderPrefix)
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
		diagnostics := diagnosticStrings(item.Diagnostics, false)
		status := visibility
		if len(item.Diagnostics) > 0 {
			status = item.Diagnostics[0].Code
		}
		invalid := false
		for _, diagnostic := range item.Diagnostics {
			if diagnostic.Severity == owl.DiagnosticError {
				invalid = true
				break
			}
		}
		envs = append(envs, SnapshotEnv{
			Name:        item.Name,
			Value:       item.Value,
			Description: item.Description,
			Type:        item.Type.Alias(),
			Field:       item.Field.String(),
			Source:      item.Source.Name,
			Explicit:    item.Explicit,
			Visibility:  status,
			Diagnostics: diagnostics,
			Invalid:     invalid,
		})
	}
	return envs
}

func diagnosticStrings(diagnostics []owl.Diagnostic, details bool) []string {
	result := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		result = append(result, diagnosticString(diagnostic, details))
	}
	return result
}

func resolveResultFromOwl(result owl.ResolveResult) *ResolveResult {
	attempts := make([]ResolverAttempt, 0, len(result.Attempts))
	for _, attempt := range result.Attempts {
		attempts = append(attempts, ResolverAttempt{
			ResolverID:    string(attempt.ResolverID),
			ProjectionKey: string(attempt.ProjectionKey),
			Outcome:       string(attempt.Outcome),
			Message:       attempt.Message,
		})
	}
	actions := make([]ResolverAction, 0, len(result.Actions))
	for _, action := range result.Actions {
		item := ResolverAction{Type: string(action.Type)}
		if action.Prompt != nil {
			item.Prompt = &PromptAction{
				NeedID:        string(action.Prompt.NeedID),
				ProjectionKey: string(action.Prompt.ProjectionKey),
				Label:         action.Prompt.Label,
				Description:   action.Prompt.Description,
				Sensitive:     action.Prompt.Sensitivity == owl.SensitivitySensitive,
				Required:      action.Prompt.Required,
				AllowEmpty:    action.Prompt.AllowEmpty,
			}
		}
		actions = append(actions, item)
	}
	return &ResolveResult{Attempts: attempts, Actions: actions}
}

func diagnosticString(diagnostic owl.Diagnostic, details bool) string {
	line := ""
	if diagnostic.Key != "" {
		line = string(diagnostic.Severity) + " " + diagnostic.Code + " " + diagnostic.Key + ": " + diagnostic.Message
	} else if diagnostic.FieldRef.TypeID != "" {
		line = string(diagnostic.Severity) + " " + diagnostic.Code + " " + diagnostic.FieldRef.String() + ": " + diagnostic.Message
	} else {
		line = string(diagnostic.Severity) + " " + diagnostic.Code + ": " + diagnostic.Message
	}
	if details {
		for _, detail := range diagnostic.Details {
			line += "\n  cue: " + detail
		}
	}
	return line
}
