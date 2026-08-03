package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolverAttemptSerializesWithoutValuePayload(t *testing.T) {
	t.Parallel()

	attempt := ResolverAttempt{
		ID:            "attempt-1",
		ResolverID:    "core/dotenv",
		FieldRef:      FieldRef{TypeID: TypeCoreSecret, Instance: "default", Field: "api.key"},
		ProjectionKey: "API_KEY",
		Outcome:       ResolverAttemptNotFound,
		Message:       "dotenv value was not present",
		Source:        Source{Name: ".env", Kind: "dotenv"},
	}

	raw, err := json.Marshal(attempt)
	require.NoError(t, err)

	assert.Contains(t, string(raw), "core/dotenv")
	assert.Contains(t, string(raw), "API_KEY")
	assert.NotContains(t, string(raw), "secret-value")
	assert.NotContains(t, string(raw), "Proposed")
	assert.NotContains(t, string(raw), "Value")
}
