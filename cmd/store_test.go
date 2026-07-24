package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreSnapshotRendersVisibilityColumn(t *testing.T) {
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
	assert.Contains(t, out.String(), "VISIBILITY")
	assert.NotContains(t, out.String(), "STATUS")
	assert.NotContains(t, out.String(), "EXPOSURE")
	assert.Contains(t, out.String(), "masked")
}

func TestStoreSnapshotRendersExplicitItemsByDefault(t *testing.T) {
	t.Parallel()

	client := &fakeStoreClient{
		snapshot: &SnapshotResult{Envs: []SnapshotEnv{
			{Name: "ZZZ_SYSTEM", Value: "[hidden]", Type: "core/opaque", Source: "(system)", Visibility: "hidden"},
			{Name: "API_KEY", Value: "[masked]", Type: "core/secret", Source: ".env", Explicit: true, Visibility: "masked"},
			{Name: "AAA_SYSTEM", Value: "[hidden]", Type: "core/opaque", Source: "(system)", Visibility: "hidden"},
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
			{Name: "ZZZ_SYSTEM", Value: "[hidden]", Type: "core/opaque", Source: "(system)", Visibility: "hidden"},
			{Name: "API_KEY", Value: "[masked]", Type: "core/secret", Source: ".env", Explicit: true, Visibility: "masked"},
			{Name: "AAA_SYSTEM", Value: "[hidden]", Type: "core/opaque", Source: "(system)", Visibility: "hidden"},
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
		InsecureAllowed: func() bool { return false },
	})
	cmd.SetArgs([]string{"snapshot", "--reveal"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be run in insecure mode")
	assert.False(t, client.snapshotCalled)
}

func TestStoreSnapshotRevealPassesRequestWhenInsecureAllowed(t *testing.T) {
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
		InsecureAllowed: func() bool { return true },
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"snapshot", "--reveal", "--all"})

	require.NoError(t, cmd.Execute())
	assert.True(t, client.snapshotReq.Reveal)
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

type fakeStoreClient struct {
	snapshot       *SnapshotResult
	source         *SourceResult
	check          *CheckResult
	typeResult     *TypeResult
	snapshotReq    SnapshotRequest
	sourceReq      SourceRequest
	typeReq        TypeRequest
	snapshotCalled bool
	sourceCalled   bool
	checkCalled    bool
	typeCalled     bool
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

func (c *fakeStoreClient) Check(context.Context, CheckRequest) (*CheckResult, error) {
	c.checkCalled = true
	return c.check, nil
}

func (c *fakeStoreClient) Type(_ context.Context, req TypeRequest) (*TypeResult, error) {
	c.typeReq = req
	c.typeCalled = true
	return c.typeResult, nil
}
