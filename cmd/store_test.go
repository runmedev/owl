package cmd

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreSnapshotRendersStatusColumn(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		snapshot: &SnapshotResult{Envs: []SnapshotEnv{
			{
				Name:        "API_KEY",
				Value:       "[masked]",
				Type:        "core/secret",
				Source:      ".env",
				Visibility:  "masked",
				Description: "API key",
			},
		}},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"snapshot", "--all"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "STATUS")
	assert.NotContains(t, out.String(), "VISIBILITY")
	assert.NotContains(t, out.String(), "EXPOSURE")
	assert.Contains(t, out.String(), "masked")
}

func TestStoreSnapshotRendersDiagnosticStatus(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		snapshot: &SnapshotResult{Envs: []SnapshotEnv{
			{
				Name:        "REDIS_HOST",
				Value:       "not a host",
				Type:        "universe/redis",
				Source:      ".env",
				Visibility:  "type.invalid-host: host value must be a hostname or IP address",
				Description: "Redis host",
				Invalid:     true,
			},
		}},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"snapshot", "--all"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "type.invalid-host")
	assert.NotContains(t, stdout.String(), "\tliteral\t")
	assert.Contains(t, stderr.String(), "run `owl check`")
}

func TestStoreSnapshotRendersExplicitItemsByDefault(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		snapshot: &SnapshotResult{Envs: []SnapshotEnv{
			{Name: "ZZZ_SYSTEM", Value: "[hidden]", Type: "core/opaque", Source: "[process]", Visibility: "hidden"},
			{Name: "API_KEY", Value: "[masked]", Type: "core/secret", Source: ".env", Explicit: true, Visibility: "masked"},
			{Name: "AAA_SYSTEM", Value: "[hidden]", Type: "core/opaque", Source: "[process]", Visibility: "hidden"},
			{Name: "API_URL", Value: "https://api.example.com", Type: "core/plain", Source: ".env", Explicit: true, Visibility: "literal"},
		}},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"snapshot"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "API_KEY")
	assert.Contains(t, out.String(), "API_URL")
	assert.NotContains(t, out.String(), "AAA_SYSTEM")
	assert.NotContains(t, out.String(), "ZZZ_SYSTEM")
	assert.Less(t, strings.Index(out.String(), "API_KEY"), strings.Index(out.String(), "API_URL"))
}

func TestStoreSnapshotAllRendersInheritedAfterExplicit(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		snapshot: &SnapshotResult{Envs: []SnapshotEnv{
			{Name: "ZZZ_SYSTEM", Value: "[hidden]", Type: "core/opaque", Source: "[process]", Visibility: "hidden"},
			{Name: "API_KEY", Value: "[masked]", Type: "core/secret", Source: ".env", Explicit: true, Visibility: "masked"},
			{Name: "AAA_SYSTEM", Value: "[hidden]", Type: "core/opaque", Source: "[process]", Visibility: "hidden"},
			{Name: "API_URL", Value: "https://api.example.com", Type: "core/plain", Source: ".env", Explicit: true, Visibility: "literal"},
		}},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"snapshot", "--all"})

	require.NoError(t, cmd.Execute())
	rendered := out.String()
	assert.Less(t, strings.Index(rendered, "API_KEY"), strings.Index(rendered, "API_URL"))
	assert.Less(t, strings.Index(rendered, "API_URL"), strings.Index(rendered, "AAA_SYSTEM"))
	assert.Less(t, strings.Index(rendered, "AAA_SYSTEM"), strings.Index(rendered, "ZZZ_SYSTEM"))
}

func TestStoreCheckRendersSuccessSummaryAndPassesDetailsFlag(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{check: &CheckResult{OK: true, Checked: 3}}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"check", "--details"})

	require.NoError(t, cmd.Execute())
	assert.True(t, client.checkReq.Details)
	assert.Equal(t, "ok: 3 variables checked, 0 errors, 0 warnings\n", out.String())
}

func TestStoreSnapshotRevealRequiresInsecurePermission(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		snapshot: &SnapshotResult{Envs: []SnapshotEnv{
			{Name: "API_KEY", Value: "secret", Type: "core/secret", Source: ".env", Visibility: "literal"},
		}},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
		InsecureModeEnabled:        func() bool { return false },
		DefineSnapshotInsecureFlag: true,
	})
	cmd.SetArgs([]string{"snapshot", "--reveal", "--insecure"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be run in insecure mode")
	assert.False(t, client.snapshotCalled)
}

func TestStoreSnapshotRevealRequiresInsecureFlag(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		snapshot: &SnapshotResult{Envs: []SnapshotEnv{
			{Name: "API_KEY", Value: "secret", Type: "core/secret", Source: ".env", Visibility: "literal"},
		}},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
		InsecureModeEnabled:        func() bool { return true },
		DefineSnapshotInsecureFlag: true,
	})
	cmd.SetArgs([]string{"snapshot", "--reveal"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be run in insecure mode")
	assert.False(t, client.snapshotCalled)
}

func TestStoreSnapshotInsecureWithoutRevealPassesSafeRequest(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		snapshot: &SnapshotResult{Envs: []SnapshotEnv{
			{Name: "API_KEY", Value: "[masked]", Type: "core/secret", Source: ".env", Visibility: "masked"},
		}},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
		InsecureModeEnabled:        func() bool { return true },
		DefineSnapshotInsecureFlag: true,
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"snapshot", "--insecure", "--all"})

	require.NoError(t, cmd.Execute())
	assert.False(t, client.snapshotReq.Reveal)
	assert.True(t, client.snapshotReq.Insecure)
	assert.Contains(t, out.String(), "[masked]")
}

func TestStoreSnapshotRevealPassesRequestWhenInsecureModeEnabled(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		snapshot: &SnapshotResult{Envs: []SnapshotEnv{
			{Name: "API_KEY", Value: "secret", Type: "core/secret", Source: ".env", Visibility: "literal"},
		}},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
		InsecureModeEnabled:        func() bool { return true },
		DefineSnapshotInsecureFlag: true,
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"snapshot", "--reveal", "--insecure", "--all"})

	require.NoError(t, cmd.Execute())
	assert.True(t, client.snapshotReq.Reveal)
	assert.True(t, client.snapshotReq.Insecure)
	assert.Contains(t, out.String(), "secret")
	assert.Contains(t, out.String(), "literal")
}

func TestStoreTypeRendersTable(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		typeResult: &TypeResult{Proposals: []TypeProposal{
			{
				Key:           "API_KEY",
				CurrentType:   "core/opaque",
				SuggestedType: "core/secret",
				Confidence:    "heuristic",
				Reason:        "key name suggests sensitive value",
			},
		}},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"type", "--spec", ".env.spec"})

	require.NoError(t, cmd.Execute())
	assert.True(t, client.typeCalled)
	assert.Equal(t, ".env.spec", client.typeReq.SpecPath)
	assert.Contains(t, out.String(), "SUGGESTED")
	assert.Contains(t, out.String(), "API_KEY")
	assert.Contains(t, out.String(), "core/secret")
}

func TestStoreTypeRendersDotenvSpec(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		typeResult: &TypeResult{Rendered: "API_KEY=\"Api Key\" # Secret\n"},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"type", "--format", "dotenv-spec"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, "API_KEY=\"Api Key\" # Secret\n", out.String())
}

func TestStoreTypeOutputDashRendersChangedSpec(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		typeResult: &TypeResult{Rendered: "API_URL=\"API URL\" # Plain\nAPI_KEY=\"Api Key\" # Secret\n"},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"type", "--output", "-"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, "API_URL=\"API URL\" # Plain\nAPI_KEY=\"Api Key\" # Secret\n", out.String())
}

func TestStoreTypeRendersMissingSuggestionAsDash(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		typeResult: &TypeResult{Proposals: []TypeProposal{
			{
				Key:           "TARGET_PLATFORM",
				CurrentType:   "core/opaque",
				SuggestedType: "",
				Confidence:    "none",
				Reason:        "no primitive type heuristic matched",
			},
		}},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"type", "--all"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "TARGET_PLATFORM")
	assert.Contains(t, out.String(), "-")
	assert.Contains(t, out.String(), "none")
}

func TestStoreSourceRequiresInsecureFlag(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{source: &SourceResult{Envs: []string{"API_KEY=secret"}}}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
	})
	cmd.SetArgs([]string{"source"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be run in insecure mode")
	assert.False(t, client.sourceCalled)
}

func TestStoreResolveRendersAttemptsAndPromptActions(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		resolve: &ResolveResult{
			Attempts: []ResolverAttempt{{
				ResolverID:    "core/dotenv",
				ProjectionKey: "API_KEY",
				Outcome:       "not_found",
			}},
			Actions: []ResolverAction{{
				Type: "prompt",
				Prompt: &PromptAction{
					NeedID:        "need:API_KEY",
					ProjectionKey: "API_KEY",
					Label:         "API key",
				},
			}},
		},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"resolve"})

	require.NoError(t, cmd.Execute())
	assert.True(t, client.resolveCalled)
	assert.False(t, client.resolveReq.Prompt)
	assert.Contains(t, out.String(), "API_KEY")
	assert.Contains(t, out.String(), "not_found")
	assert.Contains(t, out.String(), "prompt")
}

func TestStoreResolvePromptSubmitsAnswers(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		resolve: &ResolveResult{
			Actions: []ResolverAction{{
				Type: "prompt",
				Prompt: &PromptAction{
					NeedID:        "need:API_KEY",
					ProjectionKey: "API_KEY",
					Label:         "API key",
				},
			}},
		},
		applyPromptAnswers: &ResolveResult{
			Attempts: []ResolverAttempt{{ResolverID: "core/prompt", ProjectionKey: "API_KEY", Outcome: "resolved"}},
		},
	}
	cmd := NewStoreCommand(StoreCommandOptions{
		ClientFactory: func(*cobra.Command) (StoreClient, error) {
			return client, nil
		},
		PromptInput: func(_ io.Reader, _ io.Writer, prompt PromptAction) (string, error) {
			assert.Equal(t, "need:API_KEY", prompt.NeedID)
			return "secret", nil
		},
	})

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"resolve", "--prompt"})

	require.NoError(t, cmd.Execute())
	assert.True(t, client.resolveReq.Prompt)
	require.Len(t, client.promptAnswers, 1)
	assert.Equal(t, "need:API_KEY", client.promptAnswers[0].NeedID)
	assert.Equal(t, "secret", client.promptAnswers[0].Value)
	assert.Empty(t, stderr.String())
	assert.Equal(t, "resolved 1 prompted values\n", out.String())
}

type fakeStoreClient struct {
	snapshot           *SnapshotResult
	source             *SourceResult
	check              *CheckResult
	typeResult         *TypeResult
	resolve            *ResolveResult
	applyPromptAnswers *ResolveResult
	projectResult      *ProjectSpecResult
	snapshotReq        SnapshotRequest
	sourceReq          SourceRequest
	checkReq           CheckRequest
	typeReq            TypeRequest
	resolveReq         ResolveRequest
	projectReq         ProjectSpecRequest
	promptAnswers      []PromptAnswer
	snapshotCalled     bool
	sourceCalled       bool
	checkCalled        bool
	typeCalled         bool
	resolveCalled      bool
	projectCalled      bool
}

func (c *fakeStoreClient) Snapshot(_ context.Context, req SnapshotRequest) (*SnapshotResult, error) {
	c.snapshotReq = req
	c.snapshotCalled = true
	return c.snapshot, nil
}

func (c *fakeStoreClient) Source(_ context.Context, req SourceRequest) (*SourceResult, error) {
	c.sourceReq = req
	c.sourceCalled = true
	return c.source, nil
}

func (c *fakeStoreClient) Check(_ context.Context, req CheckRequest) (*CheckResult, error) {
	c.checkReq = req
	c.checkCalled = true
	return c.check, nil
}

func (c *fakeStoreClient) Type(_ context.Context, req TypeRequest) (*TypeResult, error) {
	c.typeReq = req
	c.typeCalled = true
	return c.typeResult, nil
}

func (c *fakeStoreClient) Resolve(_ context.Context, req ResolveRequest) (*ResolveResult, error) {
	c.resolveReq = req
	c.resolveCalled = true
	return c.resolve, nil
}

func (c *fakeStoreClient) ApplyPromptAnswers(_ context.Context, answers []PromptAnswer) (*ResolveResult, error) {
	c.promptAnswers = append([]PromptAnswer{}, answers...)
	return c.applyPromptAnswers, nil
}

func (c *fakeStoreClient) ProjectSpec(_ context.Context, req ProjectSpecRequest) (*ProjectSpecResult, error) {
	c.projectReq = req
	c.projectCalled = true
	return c.projectResult, nil
}
