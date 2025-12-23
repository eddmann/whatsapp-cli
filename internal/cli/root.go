package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"

	// Global flags
	formatFlag   string
	fieldsFlag   string
	noHeaderFlag bool
	storeDir     string
	timeout      time.Duration
	verbose      bool

	// Cached resolved format
	resolvedFormat Format
)

var rootCmd = &cobra.Command{
	Use:           "whatsapp",
	Short:         "WhatsApp from your terminal. Pipe it, script it, automate it.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	cobra.OnInitialize(initConfig, resolveFormatOnce)

	rootCmd.PersistentFlags().StringVarP(&formatFlag, "format", "f", "", "Output format: json, jsonl, csv, tsv, human (default: json, or $WHATSAPP_FORMAT)")
	rootCmd.PersistentFlags().StringVar(&fieldsFlag, "fields", "", "Comma-separated list of fields to include in output")
	rootCmd.PersistentFlags().BoolVar(&noHeaderFlag, "no-header", false, "Skip header row in CSV/TSV output")
	rootCmd.PersistentFlags().StringVar(&storeDir, "store", "", "Store directory (default: ~/.config/whatsapp-cli)")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Command timeout")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolP("version", "V", false, "Show version")

	rootCmd.SetVersionTemplate(fmt.Sprintf("whatsapp-cli %s\n", version))
	rootCmd.Version = version
}

func initConfig() {
	if storeDir != "" {
		SetStoreDir(storeDir)
	}
}

// resolveFormatOnce caches the output format at startup
func resolveFormatOnce() {
	// Priority: flag > env var > default (json)
	f := formatFlag
	if f == "" {
		f = os.Getenv("WHATSAPP_FORMAT")
	}
	if f == "" {
		f = "json"
	}

	resolvedFormat = Format(strings.ToLower(f))
	if !resolvedFormat.IsValid() {
		fmt.Fprintf(os.Stderr, "warning: invalid format %q, using json\n", f)
		resolvedFormat = FormatJSON
	}
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// GetFormat returns the cached output format
func GetFormat() Format {
	return resolvedFormat
}

// GetFields returns the list of fields to include in output
func GetFields() []string {
	if fieldsFlag == "" {
		return nil
	}
	fields := strings.Split(fieldsFlag, ",")
	for i, f := range fields {
		fields[i] = strings.TrimSpace(f)
	}
	return fields
}

// NoHeader returns whether to skip header row in CSV/TSV output
func NoHeader() bool {
	return noHeaderFlag
}

// GetOutputOptions returns the current output options
func GetOutputOptions() OutputOptions {
	return OutputOptions{
		Format:   GetFormat(),
		Fields:   GetFields(),
		NoHeader: NoHeader(),
	}
}

// Output helper for commands - returns error for proper handling
func Output(data any) error {
	return output(data, GetOutputOptions())
}

// OutputResult outputs structured data for machine formats, or a human message for human format
func OutputResult(data any, humanMsg string) error {
	if GetFormat() == FormatHuman {
		fmt.Println(humanMsg)
		return nil
	}
	return Output(data)
}

// OutputWarning prints warning to stderr (for non-fatal issues)
func OutputWarning(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if IsJSON() {
		fmt.Fprintf(os.Stderr, `{"warning":%q}`+"\n", msg)
	} else {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}
}

// IsJSON returns whether JSON output is enabled (json or jsonl)
func IsJSON() bool {
	f := GetFormat()
	return f == FormatJSON || f == FormatJSONL
}

// IsVerbose returns whether verbose mode is enabled
func IsVerbose() bool {
	return verbose
}
