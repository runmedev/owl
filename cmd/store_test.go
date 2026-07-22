package cmd

import (
	"bytes"
	"context"
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
	snapshotReq    SnapshotRequest
	sourceReq      SourceRequest
	snapshotCalled bool
	sourceCalled   bool
	checkCalled    bool
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
