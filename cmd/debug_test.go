package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebugGraphQLSchemaCommand(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"debug", "graphql-schema"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), `"__schema"`)
	assert.Contains(t, out.String(), `"LoadInput"`)
	assert.Contains(t, out.String(), `"EnvContractInput"`)
}

func TestDebugCommandIsHidden(t *testing.T) {
	t.Parallel()

	var debugFound bool
	for _, cmd := range NewRootCommand().Commands() {
		if cmd.Name() != "debug" {
			continue
		}
		debugFound = true
		assert.True(t, cmd.Hidden)

		var schemaFound bool
		for _, sub := range cmd.Commands() {
			if sub.Name() != "graphql-schema" {
				continue
			}
			schemaFound = true
			assert.False(t, sub.Hidden)
		}
		assert.True(t, schemaFound)
	}
	assert.True(t, debugFound)
}

func TestDebugHelpShowsSubcommands(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"debug", "--help"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "graphql-schema")
}
