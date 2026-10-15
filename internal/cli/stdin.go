package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Every read of the process's input goes through these two variables rather
// than os.Stdin directly, and tests replace them rather than os.Stdin. The
// manual `auth login` flow starts a goroutine blocked in fmt.Scanln on
// os.Stdin, and no caller can cancel a blocking read of a file descriptor, so
// a test that reassigns os.Stdin races with that goroutine and the race
// detector fails whichever test happens to run beside it.
//
// Masked input is the exception: term.ReadPassword needs the terminal's own
// descriptor, so it keeps reading os.Stdin.
var (
	stdinReader     io.Reader = os.Stdin
	stdinIsTerminal           = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
)

// readLine reads one line of input and trims its surrounding whitespace. A
// read error yields an empty line, which every prompt treats as no answer.
func readLine() string {
	line, _ := bufio.NewReader(stdinReader).ReadString('\n')
	return strings.TrimSpace(line)
}

// confirmYesNo reports whether a [y/N] prompt was answered yes.
func confirmYesNo() bool {
	response := strings.ToLower(readLine())
	return response == "y" || response == "yes"
}

// confirmDeletion prompts on stderr for a destructive command, unless --force
// was given. Input that is not a terminal cannot answer, so it is an error
// rather than a silent refusal.
func confirmDeletion(cmd *cobra.Command, format string, args ...any) (bool, error) {
	if force, _ := cmd.Flags().GetBool("force"); force {
		return true, nil
	}
	if !stdinIsTerminal() {
		return false, fmt.Errorf("use --force to delete without confirmation")
	}
	fmt.Fprintf(cmd.ErrOrStderr(), format+" [y/N]: ", args...)
	return confirmYesNo(), nil
}

// readStdinAll reads piped input for the --stdin flags.
func readStdinAll() ([]byte, error) {
	return io.ReadAll(stdinReader)
}
