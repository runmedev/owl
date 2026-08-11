package owl

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func snapshotItems(t *testing.T, store *Store, policy SnapshotPolicy) []SnapshotItem {
	t.Helper()
	output, err := store.Snapshot(context.Background(), SnapshotInput{
		Policy: policy,
		Filter: SnapshotFilter{All: true},
	})
	require.NoError(t, err)
	return output.Envs
}

func applyUpdateLines(ctx context.Context, t *testing.T, store *Store, source Source, lines []string, deleted []string) {
	t.Helper()
	var vars []DotenvVariable
	for _, line := range lines {
		key, value, ok := strings.Cut(line, "=")
		require.True(t, ok, "update line must be KEY=value")
		vars = append(vars, DotenvVariable{Key: key, Value: value, Source: source})
	}
	require.NoError(t, store.ApplyUpdate(ctx, UpdateInput{
		Source: source,
		Dotenv: vars,
		Delete: deleted,
	}))
}

func TestPublicAPIUpdateTimestampsOnlyChangedItems(t *testing.T) {
	t.Parallel()

	timestamps := newTestClock(
		time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 24, 10, 1, 0, 0, time.UTC),
		time.Date(2026, 7, 24, 10, 2, 0, 0, time.UTC),
	)
	store, err := NewStore(
		WithDotenv(".env", strings.NewReader("API_URL=https://one.example.com\nTOKEN=one\n")),
		WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain!\nTOKEN=\"Token\" # Secret!\n")),
		withClock(timestamps.Next),
	)
	require.NoError(t, err)

	initial := snapshotItems(t, store, SnapshotPolicy{Reveal: true})
	initialByName := snapshotItemsByName(initial)
	assert.Equal(t, timestamps.At(0), initialByName["API_URL"].UpdatedAt)
	assert.Equal(t, timestamps.At(0), initialByName["TOKEN"].UpdatedAt)

	applyUpdateLines(context.Background(), t, store, Source{Name: "[update]", Kind: "dotenv"}, []string{"API_URL=https://two.example.com"}, nil)
	afterAPIURLUpdate := snapshotItems(t, store, SnapshotPolicy{Reveal: true})
	afterAPIURLUpdateByName := snapshotItemsByName(afterAPIURLUpdate)
	assert.Equal(t, timestamps.At(1), afterAPIURLUpdateByName["API_URL"].UpdatedAt)
	assert.Equal(t, timestamps.At(0), afterAPIURLUpdateByName["TOKEN"].UpdatedAt)

	applyUpdateLines(context.Background(), t, store, Source{Name: "[update]", Kind: "dotenv"}, []string{"TOKEN=two"}, nil)
	afterTokenUpdate := snapshotItems(t, store, SnapshotPolicy{Reveal: true})
	afterTokenUpdateByName := snapshotItemsByName(afterTokenUpdate)
	assert.Equal(t, timestamps.At(1), afterTokenUpdateByName["API_URL"].UpdatedAt)
	assert.Equal(t, timestamps.At(2), afterTokenUpdateByName["TOKEN"].UpdatedAt)
}

func TestPublicAPIUpdateClearsResolvedRequiredDiagnostics(t *testing.T) {
	t.Parallel()

	timestamps := newTestClock(
		time.Date(2026, 7, 24, 11, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 24, 11, 1, 0, 0, time.UTC),
	)
	store, err := NewStore(
		WithEnvSpec(".env.spec", strings.NewReader("TOKEN=\"Token\" # Secret!\n")),
		withClock(timestamps.Next),
	)
	require.NoError(t, err)

	initial := snapshotItems(t, store, SnapshotPolicy{Reveal: true})
	initialByName := snapshotItemsByName(initial)
	require.Contains(t, initialByName, "TOKEN")
	assert.Contains(t, diagnosticCodes(initialByName["TOKEN"].Diagnostics), "dotenv.unresolved-required")

	ctx := ContextWithExecutionInfo(context.Background(), ExecutionInfo{ExecContext: "direnv"})
	applyUpdateLines(ctx, t, store, Source{Name: "[direnv]", Kind: "direnv"}, []string{"TOKEN=secret"}, nil)

	updated := snapshotItems(t, store, SnapshotPolicy{Reveal: true})
	updatedByName := snapshotItemsByName(updated)
	assert.Equal(t, "secret", updatedByName["TOKEN"].Value)
	assert.Equal(t, "[direnv]", updatedByName["TOKEN"].Source.Name)
	assert.NotContains(t, diagnosticCodes(updatedByName["TOKEN"].Diagnostics), "dotenv.unresolved-required")
	assert.Equal(t, timestamps.At(1), updatedByName["TOKEN"].UpdatedAt)
}

func TestPublicAPIStateEnvelopePreservesOperationTimestamps(t *testing.T) {
	t.Parallel()

	timestamps := newTestClock(
		time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 24, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 7, 24, 12, 2, 0, 0, time.UTC),
	)
	store, err := NewStore(
		WithDotenv(".env", strings.NewReader("API_URL=https://one.example.com\n")),
		WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain!\n")),
		withClock(timestamps.Next),
	)
	require.NoError(t, err)
	applyUpdateLines(context.Background(), t, store, Source{Name: "[update]", Kind: "dotenv"}, []string{"API_URL=https://two.example.com"}, nil)

	envelope, err := store.StateEnvelope(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, envelope.Provenance.Operations)
	assert.Contains(t, operationTimestamps(envelope.Provenance.Operations), timestamps.At(0))
	assert.Contains(t, operationTimestamps(envelope.Provenance.Operations), timestamps.At(1))

	roundTripped, err := NewStore(
		WithStateEnvelope(envelope),
		withClock(timestamps.Next),
	)
	require.NoError(t, err)
	roundTrippedEnvelope, err := roundTripped.StateEnvelope(context.Background())
	require.NoError(t, err)
	assert.Contains(t, operationTimestamps(roundTrippedEnvelope.Provenance.Operations), timestamps.At(0))
	assert.Contains(t, operationTimestamps(roundTrippedEnvelope.Provenance.Operations), timestamps.At(1))

	applyUpdateLines(context.Background(), t, roundTripped, Source{Name: "[update]", Kind: "dotenv"}, []string{"API_URL=https://three.example.com"}, nil)
	snapshot := snapshotItems(t, roundTripped, SnapshotPolicy{Reveal: true})
	assert.Equal(t, timestamps.At(2), snapshotItemsByName(snapshot)["API_URL"].UpdatedAt)
}

type testClock struct {
	timestamps []time.Time
	next       int
}

func newTestClock(timestamps ...time.Time) *testClock {
	return &testClock{timestamps: timestamps}
}

func (c *testClock) Next() time.Time {
	if c.next >= len(c.timestamps) {
		return c.timestamps[len(c.timestamps)-1]
	}
	timestamp := c.timestamps[c.next]
	c.next++
	return timestamp
}

func (c *testClock) At(index int) time.Time {
	return c.timestamps[index]
}

func snapshotItemsByName(items []SnapshotItem) map[string]SnapshotItem {
	result := make(map[string]SnapshotItem, len(items))
	for _, item := range items {
		result[item.Name] = item
	}
	return result
}

func operationTimestamps(operations []OperationMetadata) []time.Time {
	result := make([]time.Time, 0, len(operations))
	for _, operation := range operations {
		result = append(result, operation.Timestamp)
	}
	return result
}

func diagnosticCodes(diagnostics []Diagnostic) []string {
	result := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		result = append(result, diagnostic.Code)
	}
	return result
}
