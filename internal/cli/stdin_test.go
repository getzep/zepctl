package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubStdin makes the package read input from the given string, and answer the
// terminal check as asked, for the duration of one test. Tests must never
// reassign os.Stdin: `auth login` leaves a goroutine blocked on a read of it
// that nothing can cancel, and the race detector fails any test that writes
// the variable while that goroutine is alive.
func stubStdin(t *testing.T, isTerminal bool, input string) {
	t.Helper()
	oldReader, oldIsTerminal := stdinReader, stdinIsTerminal
	stdinReader = strings.NewReader(input)
	stdinIsTerminal = func() bool { return isTerminal }
	t.Cleanup(func() {
		stdinReader, stdinIsTerminal = oldReader, oldIsTerminal
	})
}

// deleteTestCommand is a stand-in for the delete commands, which all carry a
// --force flag.
func deleteTestCommand(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{Use: "delete"}
	cmd.Flags().Bool("force", false, "")
	stderr := &bytes.Buffer{}
	cmd.SetErr(stderr)
	return cmd, stderr
}

func TestConfirmYesNo(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"YES\n", true},
		{"  Y  \n", true},
		{"n\n", false},
		{"no\n", false},
		{"\n", false},
		{"", false},
		{"yep\n", false},
	}
	for _, tt := range tests {
		t.Run(strings.TrimSpace(tt.input), func(t *testing.T) {
			stubStdin(t, true, tt.input)
			assert.Equal(t, tt.want, confirmYesNo())
		})
	}
}

func TestReadLineKeepsPipedInputForLaterPrompts(t *testing.T) {
	stubStdin(t, false, "y\nsecret-key\n")

	assert.Equal(t, "y", readLine())
	assert.Equal(t, "secret-key", readLine())
	assert.Empty(t, readLine())
}

func TestReadLineWithoutTrailingNewline(t *testing.T) {
	stubStdin(t, false, "abc")

	assert.Equal(t, "abc", readLine())
}

func TestConfirmDeletion_ForceSkipsPrompt(t *testing.T) {
	stubStdin(t, false, "")
	cmd, stderr := deleteTestCommand(t)
	require.NoError(t, cmd.Flags().Set("force", "true"))

	confirmed, err := confirmDeletion(cmd, "Delete %s?", "thing")
	require.NoError(t, err)
	assert.True(t, confirmed)
	assert.Empty(t, stderr.String())
}

func TestConfirmDeletion_NonTerminalIsAnError(t *testing.T) {
	stubStdin(t, false, "y\n")
	cmd, stderr := deleteTestCommand(t)

	confirmed, err := confirmDeletion(cmd, "Delete %s?", "thing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use --force to delete without confirmation")
	assert.False(t, confirmed)
	assert.Empty(t, stderr.String())
}

func TestConfirmDeletion_PromptsOnStderr(t *testing.T) {
	stubStdin(t, true, "y\n")
	cmd, stderr := deleteTestCommand(t)

	confirmed, err := confirmDeletion(cmd, "Delete %s?", "thing")
	require.NoError(t, err)
	assert.True(t, confirmed)
	assert.Equal(t, "Delete thing? [y/N]: ", stderr.String())
}

func TestConfirmDeletion_DeclinedAnswer(t *testing.T) {
	stubStdin(t, true, "n\n")
	cmd, _ := deleteTestCommand(t)

	confirmed, err := confirmDeletion(cmd, "Delete %s?", "thing")
	require.NoError(t, err)
	assert.False(t, confirmed)
}

func TestReadStdinAll(t *testing.T) {
	stubStdin(t, false, "line one\nline two\n")

	data, err := readStdinAll()
	require.NoError(t, err)
	assert.Equal(t, "line one\nline two\n", string(data))
}
