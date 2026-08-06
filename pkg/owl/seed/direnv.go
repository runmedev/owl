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
	"strings"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/pkg/owl"
)

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

type direnvVariablesRequest struct {
	options  Options
	keys     map[string]struct{}
	observed map[string]string
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

func direnvVariables(ctx context.Context, req direnvVariablesRequest) ([]owl.DotenvVariable, []owl.Diagnostic, error) {
	opts := req.options
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
			return runDirenvExportJSON(ctx, dir, req.keys)
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
	var diagnostics []owl.Diagnostic
	for _, key := range keys {
		if observedValue, ok := req.observed[key]; ok && observedValue != values[key] {
			diagnostics = append(diagnostics, direnvMismatchDiagnostic(key))
			continue
		}
		vars = append(vars, owl.DotenvVariable{
			Key:    key,
			Value:  values[key],
			Source: owl.Source{Name: ".envrc", Kind: "direnv"},
		})
	}
	return vars, diagnostics, nil
}

func direnvDiagnostic(code string, err error) owl.Diagnostic {
	return owl.Diagnostic{
		Severity: owl.DiagnosticWarning,
		Code:     code,
		Message:  err.Error(),
		Owner:    model.DiagnosticOwnerParse,
	}
}

func direnvMismatchDiagnostic(key string) owl.Diagnostic {
	return owl.Diagnostic{
		Severity: owl.DiagnosticWarning,
		Code:     "direnv.mismatch",
		Message:  "direnv value differs from observed environment; keeping observed value",
		Key:      key,
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
			if strings.HasPrefix(key, "DIRENV_") {
				continue
			}
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
