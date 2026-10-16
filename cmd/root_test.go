package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDaemon struct {
	name string
	runs int
	err  error
}

func (f *fakeDaemon) Name() string {
	return f.name
}

func (f *fakeDaemon) Run() error {
	f.runs++
	return f.err
}

func TestCreateRootCommand_OnceFlag(t *testing.T) {
	compaction := &fakeDaemon{name: "compaction"}
	protoParsing := &fakeDaemon{name: "proto parsing"}

	launcher := &Launcher{
		compactionDaemon:   compaction,
		protoParsingDaemon: protoParsing,
	}

	rootCmd := CreateRootCommand(launcher)
	rootCmd.SetArgs([]string{"compaction", "--once"})
	require.NoError(t, rootCmd.Execute())
	assert.Equal(t, 1, compaction.runs)
	assert.Equal(t, 0, protoParsing.runs)

	rootCmd2 := CreateRootCommand(launcher)
	rootCmd2.SetArgs([]string{"proto-parsing", "--once"})
	require.NoError(t, rootCmd2.Execute())
	assert.Equal(t, 1, compaction.runs)
	assert.Equal(t, 1, protoParsing.runs)
}
