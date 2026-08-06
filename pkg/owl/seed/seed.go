package seed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/projection/dotenv"
	"github.com/runmedev/owl/internal/requirements"
	"github.com/runmedev/owl/pkg/owl"
)

// Options configures store seeding from observed environment sources.
type Options struct {
	EnvFiles     []string
	SpecFiles    []string
	ConfigPath   string
	Observed     []ObservedSource
	WorkDir      string
	Direnv       DirenvPolicy
	DirenvRunner DirenvExportRunner
}

// ObservedSource is an environment-shaped snapshot with caller-provided provenance.
type ObservedSource struct {
	Source  owl.Source
	Environ []string
}

// DirenvPolicy controls whether seed evaluates direnv for .envrc values.
type DirenvPolicy string

const (
	// DirenvDisabled prevents direnv execution.
	DirenvDisabled DirenvPolicy = "disabled"
	// DirenvEnabledWarn records direnv failures as diagnostics and continues.
	DirenvEnabledWarn DirenvPolicy = "enabled_warn"
	// DirenvEnabledError returns direnv failures as errors.
	DirenvEnabledError DirenvPolicy = "enabled_error"
)

// DirenvExportRunner runs direnv export and returns environment key/value pairs.
type DirenvExportRunner func(context.Context, string) (map[string]string, error)

// Result is the seeded store plus catalog and source diagnostics used to build it.
type Result struct {
	Store       *owl.Store
	Catalog     Catalog
	Diagnostics []owl.Diagnostic
}

// Catalog contains candidate values grouped by source role.
type Catalog struct {
	observed []owl.DotenvVariable
	dotenv   []owl.DotenvVariable
}

// All returns every candidate value in seed precedence order.
func (c Catalog) All() []owl.DotenvVariable {
	vars := make([]owl.DotenvVariable, 0, len(c.observed)+len(c.dotenv))
	vars = append(vars, c.observed...)
	vars = append(vars, c.dotenv...)
	return vars
}

// ProcessResolverInput returns observed values for the process resolver.
func (c Catalog) ProcessResolverInput() []owl.DotenvVariable {
	return append([]owl.DotenvVariable{}, c.observed...)
}

// DotenvResolverInput returns dotenv-shaped values in resolver precedence order.
func (c Catalog) DotenvResolverInput() []owl.DotenvVariable {
	return reverseSourceGroups(c.dotenv)
}

// NewStore creates an Owl store and seeds inherited values from observed sources.
func NewStore(ctx context.Context, opts Options) (*Result, error) {
	store, err := newBaseStore(opts, false, true, false)
	if err != nil {
		return nil, err
	}
	catalog, diagnostics, err := buildCatalog(ctx, opts, projectionKeys(store))
	if err != nil {
		return nil, err
	}
	if err := seedInheritedValues(store, catalog); err != nil {
		return nil, err
	}
	return &Result{
		Store:       store,
		Catalog:     catalog,
		Diagnostics: diagnostics,
	}, nil
}

// NewValueStore creates a store with raw observed and dotenv values loaded directly.
func NewValueStore(opts Options, allowMissingSpec bool) (*owl.Store, error) {
	return newBaseStore(opts, allowMissingSpec, false, true)
}

// BuildCatalog discovers candidate values without creating or mutating a store.
func BuildCatalog(ctx context.Context, opts Options) (Catalog, []owl.Diagnostic, error) {
	return buildCatalog(ctx, opts, nil)
}

func buildCatalog(ctx context.Context, opts Options, direnvKeys map[string]struct{}) (Catalog, []owl.Diagnostic, error) {
	observed := observedVariables(opts.Observed)
	dotenvVars, err := dotenvVariables(opts)
	if err != nil {
		return Catalog{}, nil, err
	}
	direnvVars, diagnostics, err := direnvVariables(ctx, opts, direnvKeys)
	if err != nil {
		return Catalog{}, diagnostics, err
	}
	dotenvVars = append(dotenvVars, direnvVars...)
	return Catalog{observed: observed, dotenv: dotenvVars}, diagnostics, nil
}

func (p *DirenvPolicy) String() string {
	if p == nil || *p == "" {
		return string(DirenvDisabled)
	}
	return string(*p)
}

func (p *DirenvPolicy) Set(value string) error {
	policy := DirenvPolicy(value)
	switch policy {
	case DirenvDisabled, DirenvEnabledWarn, DirenvEnabledError:
		*p = policy
		return nil
	default:
		return fmt.Errorf("invalid direnv policy %q", value)
	}
}

func (p *DirenvPolicy) Type() string {
	return "direnv-policy"
}

func newBaseStore(opts Options, allowMissingSpec bool, loadConfig bool, loadValues bool) (*owl.Store, error) {
	var storeOpts []owl.StoreOption

	var configPath string
	if loadConfig {
		path, err := resolveConfigPath(opts.WorkDir, opts.ConfigPath, false)
		if err != nil {
			return nil, err
		}
		configPath = path
		if configPath != "" {
			input, err := requirements.ReadConfigFile(configPath)
			if err != nil {
				return nil, err
			}
			storeOpts = append(storeOpts, owl.WithConfig(input))
		}
	}

	if configPath != "" {
		if err := validateNoHumanDotenvSpecs(opts.WorkDir, opts.SpecFiles); err != nil {
			return nil, err
		}
	}

	specFiles, err := filesOrDefaults(opts.WorkDir, opts.SpecFiles, ".env.sample", ".env.example", ".env.spec")
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
		storeOpts = append(storeOpts, owl.WithEnvSpec(file, bytes.NewReader(raw)))
	}

	if loadValues {
		envFiles, err := filesOrDefaults(opts.WorkDir, opts.EnvFiles, ".env")
		if err != nil {
			return nil, err
		}
		storeOpts = append(storeOpts, owl.WithDotenv("[process]", strings.NewReader(processEnvDotenv(flattenObservedEnviron(opts.Observed)))))
		for _, file := range envFiles {
			raw, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			storeOpts = append(storeOpts, owl.WithDotenv(file, bytes.NewReader(raw)))
		}
	}

	return owl.NewStore(storeOpts...)
}

func seedInheritedValues(store *owl.Store, catalog Catalog) error {
	explicit, err := explicitSnapshotKeys(store)
	if err != nil {
		return err
	}
	return loadInheritedVariables(store, catalog.All(), explicit)
}

func explicitSnapshotKeys(store *owl.Store) (map[string]struct{}, error) {
	items, err := store.Snapshot(owl.SnapshotPolicy{})
	if err != nil {
		return nil, err
	}
	keys := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Explicit {
			keys[item.Name] = struct{}{}
		}
	}
	return keys, nil
}

func loadInheritedVariables(store *owl.Store, vars []owl.DotenvVariable, explicit map[string]struct{}) error {
	type sourceVars struct {
		source owl.Source
		vars   []owl.DotenvVariable
	}
	positions := make(map[owl.Source]int)
	groups := make([]sourceVars, 0)
	for _, variable := range vars {
		if _, ok := explicit[variable.Key]; ok {
			continue
		}
		source := variable.Source
		index, ok := positions[source]
		if !ok {
			index = len(groups)
			positions[source] = index
			groups = append(groups, sourceVars{source: source})
		}
		groups[index].vars = append(groups[index].vars, variable)
	}
	for _, group := range groups {
		if err := store.LoadDotenv(group.source, group.vars); err != nil {
			return err
		}
	}
	return nil
}

func observedVariables(sources []ObservedSource) []owl.DotenvVariable {
	var vars []owl.DotenvVariable
	for _, observed := range sources {
		source := observed.Source
		if source.Name == "" && source.Kind == "" {
			source = owl.Source{Name: "[process]", Kind: "process"}
		}
		for _, item := range observed.Environ {
			key, value, ok := strings.Cut(item, "=")
			if !ok || !isDotenvKey(key) {
				continue
			}
			vars = append(vars, owl.DotenvVariable{
				Key:    key,
				Value:  value,
				Source: source,
			})
		}
	}
	sort.SliceStable(vars, func(i, j int) bool {
		if vars[i].Source == vars[j].Source {
			return vars[i].Key < vars[j].Key
		}
		if vars[i].Source.Name == vars[j].Source.Name {
			return vars[i].Source.Kind < vars[j].Source.Kind
		}
		return vars[i].Source.Name < vars[j].Source.Name
	})
	return vars
}

func dotenvVariables(opts Options) ([]owl.DotenvVariable, error) {
	envFiles, err := filesOrDefaults(opts.WorkDir, opts.EnvFiles, ".env")
	if err != nil {
		return nil, err
	}
	var vars []owl.DotenvVariable
	for _, file := range envFiles {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		parsed, err := dotenv.ParseDotenvValues(raw)
		if err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(parsed))
		for key := range parsed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			vars = append(vars, owl.DotenvVariable{
				Key:    key,
				Value:  parsed[key],
				Source: owl.Source{Name: file, Kind: "dotenv"},
			})
		}
	}
	return vars, nil
}

func projectionKeys(store *owl.Store) map[string]struct{} {
	items, err := store.Snapshot(owl.SnapshotPolicy{})
	if err != nil {
		return nil
	}
	keys := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Name != "" {
			keys[item.Name] = struct{}{}
		}
	}
	return keys
}

func direnvVariables(ctx context.Context, opts Options, direnvKeys map[string]struct{}) ([]owl.DotenvVariable, []owl.Diagnostic, error) {
	policy := opts.Direnv
	if policy == "" {
		policy = DirenvDisabled
	}
	if policy == DirenvDisabled {
		return nil, nil, nil
	}
	dir := opts.WorkDir
	if dir == "" {
		dir = "."
	}
	if _, err := os.Stat(filepath.Join(dir, ".envrc")); errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	} else if err != nil {
		return nil, []owl.Diagnostic{direnvDiagnostic("direnv.stat", err)}, nil
	}
	runner := opts.DirenvRunner
	if runner == nil {
		runner = func(ctx context.Context, dir string) (map[string]string, error) {
			return runDirenvExportJSON(ctx, dir, direnvKeys)
		}
	}
	values, err := runner(ctx, dir)
	if err != nil {
		diagnostic := direnvDiagnostic("direnv.export", err)
		if policy == DirenvEnabledError {
			return nil, []owl.Diagnostic{diagnostic}, err
		}
		return nil, []owl.Diagnostic{diagnostic}, nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if isDotenvKey(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var vars []owl.DotenvVariable
	for _, key := range keys {
		vars = append(vars, owl.DotenvVariable{
			Key:    key,
			Value:  values[key],
			Source: owl.Source{Name: ".envrc", Kind: "direnv"},
		})
	}
	return vars, nil, nil
}

func direnvDiagnostic(code string, err error) owl.Diagnostic {
	return owl.Diagnostic{
		Severity: owl.DiagnosticWarning,
		Code:     code,
		Message:  err.Error(),
		Owner:    model.DiagnosticOwnerParse,
	}
}

// RunDirenvExportJSON runs direnv export json in dir and returns non-null values.
func RunDirenvExportJSON(ctx context.Context, dir string) (map[string]string, error) {
	return runDirenvExportJSON(ctx, dir, nil)
}

func runDirenvExportJSON(ctx context.Context, dir string, unsetKeys map[string]struct{}) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "direnv", "export", "json")
	cmd.Dir = dir
	cmd.Env = direnvCommandEnv(unsetKeys)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(msg)
	}
	if msg := strings.TrimSpace(stderr.String()); strings.Contains(msg, "direnv: error") {
		return nil, errors.New(msg)
	}
	if msg := strings.TrimSpace(string(raw)); strings.Contains(msg, "direnv: error") {
		return nil, errors.New(msg)
	}
	raw, err = extractJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var decoded map[string]*string
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	values := make(map[string]string, len(decoded))
	for key, value := range decoded {
		if value == nil {
			continue
		}
		values[key] = *value
	}
	return values, nil
}

func direnvCommandEnv(unsetKeys map[string]struct{}) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if ok {
			if _, unset := unsetKeys[key]; unset {
				continue
			}
		}
		env = append(env, item)
	}
	return append(env, "DIRENV_LOG_FORMAT=")
}

func extractJSONObject(raw []byte) ([]byte, error) {
	start := bytes.IndexByte(raw, '{')
	end := bytes.LastIndexByte(raw, '}')
	if start < 0 || end < start {
		return nil, errors.New("direnv export json did not include a JSON object")
	}
	return raw[start : end+1], nil
}

func reverseSourceGroups(vars []owl.DotenvVariable) []owl.DotenvVariable {
	type sourceGroup struct {
		source owl.Source
		vars   []owl.DotenvVariable
	}
	positions := make(map[owl.Source]int)
	groups := make([]sourceGroup, 0)
	for _, variable := range vars {
		source := variable.Source
		index, ok := positions[source]
		if !ok {
			index = len(groups)
			positions[source] = index
			groups = append(groups, sourceGroup{source: source})
		}
		groups[index].vars = append(groups[index].vars, variable)
	}
	reversed := make([]owl.DotenvVariable, 0, len(vars))
	for i := len(groups) - 1; i >= 0; i-- {
		reversed = append(reversed, groups[i].vars...)
	}
	return reversed
}

func flattenObservedEnviron(sources []ObservedSource) []string {
	var envs []string
	for _, source := range sources {
		envs = append(envs, source.Environ...)
	}
	return envs
}

func processEnvDotenv(envs []string) string {
	envs = append([]string{}, envs...)
	sort.Strings(envs)
	var b strings.Builder
	for _, env := range envs {
		key, value, ok := strings.Cut(env, "=")
		if !ok || !isDotenvKey(key) {
			continue
		}
		_, _ = b.WriteString(key)
		_ = b.WriteByte('=')
		_, _ = b.WriteString(strconv.Quote(value))
		_ = b.WriteByte('\n')
	}
	return b.String()
}

func isDotenvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func resolveConfigPath(workDir string, explicit string, required bool) (string, error) {
	if explicit != "" {
		path := pathInWorkDir(workDir, explicit)
		if _, err := os.Stat(path); err != nil {
			return "", err
		}
		return path, nil
	}
	var found []string
	for _, candidate := range []string{"owl.toml", "owl.yaml", "owl.yml", "owl.json"} {
		path := pathInWorkDir(workDir, candidate)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
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

func validateNoHumanDotenvSpecs(workDir string, specFiles []string) error {
	files := specFiles
	if len(files) == 0 {
		files = []string{".env.sample", ".env.example", ".env.spec"}
	}
	for _, file := range files {
		raw, err := os.ReadFile(pathInWorkDir(workDir, file))
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

func isGeneratedDotenvSpec(raw []byte) bool {
	return strings.HasPrefix(string(raw), requirements.GeneratedDotenvSpecHeaderPrefix)
}

func filesOrDefaults(workDir string, files []string, defaults ...string) ([]string, error) {
	if len(files) > 0 {
		resolved := make([]string, 0, len(files))
		for _, file := range files {
			resolved = append(resolved, pathInWorkDir(workDir, file))
		}
		return resolved, nil
	}

	var existing []string
	for _, file := range defaults {
		path := pathInWorkDir(workDir, file)
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	return existing, nil
}

func pathInWorkDir(workDir string, path string) string {
	if workDir == "" || workDir == "." || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workDir, path)
}
