package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

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

func TestReadSSOCodeFrom(t *testing.T) {
	for _, tt := range []struct {
		name, input, want string
		hidden            bool
	}{
		{"terminal", "test-code\r", "test-code", true},
		{"terminal CRLF", "test-code\r\n", "test-code", true},
		{"terminal backspace", "test-codx\x7fe\r", "test-code", true},
		{"terminal whitespace", "  test-code  \r", "test-code", true},
		{"pipe", "test-code\n", "test-code", false},
		{"pipe EOF", "test-code", "test-code", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			code, err := readSSOCodeFrom(context.Background(), strings.NewReader(tt.input), tt.hidden)
			require.NoError(t, err)
			assert.Equal(t, tt.want, code)
		})
	}
}

func TestReadSSOCodeFromTerminalInterrupt(t *testing.T) {
	for _, input := range []string{"", "test-code\x03"} {
		code, err := readSSOCodeFrom(context.Background(), strings.NewReader(input), true)
		require.ErrorIs(t, err, io.EOF)
		assert.Empty(t, code)
	}
}

type notifyingCodeReader struct {
	io.Reader
	started chan struct{}
}

func (r notifyingCodeReader) Read(p []byte) (int, error) {
	select {
	case r.started <- struct{}{}:
	default:
	}
	return r.Reader.Read(p)
}

func TestReadSSOCodeFromCancellation(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		t.Run(fmt.Sprint(hidden), func(t *testing.T) {
			reader, writer := io.Pipe()
			t.Cleanup(func() {
				_ = reader.Close()
				_ = writer.Close()
			})
			input := notifyingCodeReader{Reader: reader, started: make(chan struct{}, 1)}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := readSSOCodeFrom(ctx, input, hidden)
				done <- err
			}()
			select {
			case <-input.started:
			case <-time.After(time.Second):
				t.Fatal("code input did not start")
			}
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				t.Fatal("code input did not cancel")
			}
		})
	}
}

func TestReadSSOCodeContextPipe(t *testing.T) {
	stubStdin(t, false, "test-code\nnext-line\n")
	var output bytes.Buffer
	code, err := readSSOCodeContext(context.Background(), &output)
	require.NoError(t, err)
	assert.Equal(t, "test-code", code)
	assert.Equal(t, "next-line", readLine())
	assert.Empty(t, output.String())
}

func TestReadSSOCodeContextAlreadyCanceled(t *testing.T) {
	stubStdin(t, true, "unused\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	code, err := readSSOCodeContext(ctx, &output)
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, code)
	assert.Empty(t, output.String())
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
