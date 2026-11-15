package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// Build metadata. These are populated via -X ldflags at release time (see the
// builds section of .goreleaser.yml and the build-cli target in the Makefile).
// When they are left empty, for example on a plain `go install`, buildInfo falls
// back to the values embedded by the Go toolchain via runtime/debug.ReadBuildInfo.
var (
	version = ""
	commit  = ""
	date    = ""
)

type versionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// buildInfo returns the resolved version metadata, preferring ldflags values and
// falling back to the module version and VCS stamps recorded in the binary.
func buildInfo() versionInfo {
	info := versionInfo{
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		if info.Version == "" && bi.Main.Version != "" {
			info.Version = bi.Main.Version
		}
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.Date == "" {
					info.Date = setting.Value
				}
			}
		}
	}

	if info.Version == "" {
		info.Version = "dev"
	}
	if info.Commit == "" {
		info.Commit = "unknown"
	}
	if info.Date == "" {
		info.Date = "unknown"
	}

	return info
}

// NewVersionCmd creates the `pbuf version` command.
func NewVersionCmd() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Print the pbuf CLI version, commit, build date, and Go runtime details",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			outputFormat, err := parseVersionFormat(cmd)
			if err != nil {
				return err
			}
			return renderVersion(cmd.OutOrStdout(), buildInfo(), outputFormat)
		},
	}

	versionCmd.Flags().StringP("output", "o", "text", "output format: text|json")
	return versionCmd
}

func parseVersionFormat(cmd *cobra.Command) (string, error) {
	raw, err := cmd.Flags().GetString("output")
	if err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "text", "":
		return "text", nil
	case "json":
		return "json", nil
	default:
		return "", fmt.Errorf("invalid --output value %q (must be text or json)", raw)
	}
}

func renderVersion(w io.Writer, info versionInfo, outputFormat string) error {
	if outputFormat == "json" {
		marshalled, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal version: %w", err)
		}
		_, err = fmt.Fprintln(w, string(marshalled))
		return err
	}

	_, err := fmt.Fprintf(
		w,
		"pbuf %s\ncommit: %s\nbuilt: %s\ngo: %s\nplatform: %s\n",
		info.Version,
		info.Commit,
		info.Date,
		info.GoVersion,
		info.Platform,
	)
	return err
}
