package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Format represents an output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatWide  Format = "wide"
)

// GetFormat returns the configured output format.
func GetFormat() Format {
	f := viper.GetString("output")
	switch f {
	case "json":
		return FormatJSON
	case "yaml":
		return FormatYAML
	case "wide":
		return FormatWide
	default:
		return FormatTable
	}
}

// Print outputs data in the configured format.
func Print(data any) error {
	return Fprint(os.Stdout, data)
}

// Fprint outputs data to the given writer in the configured format.
func Fprint(w io.Writer, data any) error {
	switch GetFormat() {
	case FormatJSON:
		return printJSON(w, data)
	case FormatYAML:
		return printYAML(w, data)
	case FormatTable, FormatWide:
		// Table/Wide format should be handled by the caller with NewTable.
		// Fall through to JSON for generic Print calls.
		return printJSON(w, data)
	}
	return printJSON(w, data)
}

func printJSON(w io.Writer, data any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func printYAML(w io.Writer, data any) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}

// Table provides a simple table writer.
type Table struct {
	w       *tabwriter.Writer
	headers []string
}

// NewTable creates a new table with the given headers.
func NewTable(headers ...string) *Table {
	t := &Table{
		w:       tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0),
		headers: headers,
	}
	return t
}

// WriteHeader writes the table header.
func (t *Table) WriteHeader() {
	for i, h := range t.headers {
		if i > 0 {
			fmt.Fprint(t.w, "\t")
		}
		fmt.Fprint(t.w, h)
	}
	fmt.Fprintln(t.w)
}

// WriteRow writes a row to the table.
func (t *Table) WriteRow(values ...string) {
	for i, v := range values {
		if i > 0 {
			fmt.Fprint(t.w, "\t")
		}
		fmt.Fprint(t.w, v)
	}
	fmt.Fprintln(t.w)
}

// Flush flushes the table output.
func (t *Table) Flush() error {
	return t.w.Flush()
}

// IsQuiet returns true if quiet mode is enabled.
func IsQuiet() bool {
	return viper.GetBool("quiet")
}

// Info prints an informational message (suppressed in quiet mode).
func Info(format string, args ...any) {
	if !IsQuiet() {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

// Warn prints a warning message.
func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", args...)
}

// Error prints an error message.
func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}
