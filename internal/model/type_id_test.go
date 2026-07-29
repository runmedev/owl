package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTypeIDAcceptsCanonicalRefsAndAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ref string
		id  TypeID
	}{
		{"github.com/runmedev/owl/types/core/plain", TypeCorePlain},
		{"github.com/runmedev/owl/types/core/secret", TypeCoreSecret},
		{"github.com/runmedev/owl/types/universe/redis", TypeUniverseRedis},
		{"core/plain", TypeCorePlain},
		{"universe/redis", TypeUniverseRedis},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			id, err := ParseTypeID(tt.ref)
			require.NoError(t, err)
			assert.Equal(t, tt.id, id)
		})
	}
}

func TestParseTypeIDRejectsLegacyHTTPSRefs(t *testing.T) {
	t.Parallel()

	_, err := ParseTypeID("https://owl.runme.dev/v1/types/core/plain")
	require.Error(t, err)
}

func TestParseTypeIDRejectsUnknownRefs(t *testing.T) {
	t.Parallel()

	_, err := ParseTypeID("github.com/runmedev/owl/types/core/missing")
	require.Error(t, err)

	_, err = ParseTypeID("https://owl.runme.dev/v1/types/core/missing")
	require.Error(t, err)

	_, err = ParseTypeID("core/missing")
	require.Error(t, err)
}
