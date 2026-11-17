package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/modules"
	"github.com/pbufio/pbuf-registry/internal/cli/registry"
	"github.com/pbufio/pbuf-registry/internal/mcp"
	"github.com/pbufio/pbuf-registry/internal/utils"
	"github.com/spf13/cobra"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
)

func NewDriftCmd(config *model.Config, client v1.DriftServiceClient, registryClients ...v1.RegistryClient) *cobra.Command {
	var regClient v1.RegistryClient
	if len(registryClients) > 0 {
		regClient = registryClients[0]
	}

	driftCmd := &cobra.Command{
		Use:   "drift",
		Short: "Drift",
		Long:  "Drift is a command to manage drift detection events",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	driftCmd.AddCommand(newListDriftEventsCmd(client))
	driftCmd.AddCommand(newGetModuleDriftEventsCmd(client))
	driftCmd.AddCommand(newGetModuleDependencyDriftStatusCmd(client))
	driftCmd.AddCommand(newAcknowledgeDriftEventCmd(client))
	driftCmd.AddCommand(newDriftDiffCmd(config, regClient))

	return driftCmd
}

func newListDriftEventsCmd(client v1.DriftServiceClient) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List drift events",
		Long:  "List is a command to list all drift events",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			unacknowledgedOnly, err := cmd.Flags().GetBool("unacknowledged-only")
			if err != nil {
				return err
			}

			resp, err := client.ListDriftEvents(cmd.Context(), &v1.ListDriftEventsRequest{
				UnacknowledgedOnly: unacknowledgedOnly,
			})
			if err != nil {
				return err
			}

			return printJSON(cmd.OutOrStdout(), resp)
		},
	}

	listCmd.Flags().Bool("unacknowledged-only", true, "only return unacknowledged events")
	return listCmd
}

func newGetModuleDriftEventsCmd(client v1.DriftServiceClient) *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "module [module_name]",
		Short: "Get module drift events",
		Long:  "Module is a command to get drift events for a specific module",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			moduleName := args[0]
			tagName, err := cmd.Flags().GetString("tag")
			if err != nil {
				return err
			}

			req := &v1.GetModuleDriftEventsRequest{
				ModuleName: moduleName,
			}
			if tagName != "" {
				req.TagName = &tagName
			}

			resp, err := client.GetModuleDriftEvents(cmd.Context(), req)
			if err != nil {
				return err
			}

			return printJSON(cmd.OutOrStdout(), resp)
		},
	}

	getCmd.Flags().String("tag", "", "filter drift events by tag name")
	return getCmd
}

func newGetModuleDependencyDriftStatusCmd(client v1.DriftServiceClient) *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "dependencies [module_name]",
		Short: "Get dependency drift status",
		Long:  "Dependencies is a command to get dependency drift status for a specific module",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			moduleName := args[0]
			tagName, err := cmd.Flags().GetString("tag")
			if err != nil {
				return err
			}

			req := &v1.GetModuleDependencyDriftStatusRequest{
				ModuleName: moduleName,
			}
			if tagName != "" {
				req.TagName = &tagName
			}

			resp, err := client.GetModuleDependencyDriftStatus(cmd.Context(), req)
			if err != nil {
				return err
			}

			return printDependencyDriftStatuses(cmd.OutOrStdout(), moduleName, tagName, resp.GetStatuses())
		},
	}

	getCmd.Flags().String("tag", "", "module tag to evaluate (latest stable when omitted)")
	return getCmd
}

func printDependencyDriftStatuses(w io.Writer, moduleName, tagName string, statuses []*v1.DependencyDriftStatus) error {
	if _, err := fmt.Fprintf(w, "Dependency drift status for %s", moduleName); err != nil {
		return err
	}
	if tagName != "" {
		if _, err := fmt.Fprintf(w, " (tag: %s)", tagName); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if len(statuses) == 0 {
		_, err := fmt.Fprintln(w, "No dependency drift detected.")
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "DEPENDENCY\tCURRENT\tTARGET\tSEVERITY\tRECOMMENDATION"); err != nil {
		return err
	}

	for _, status := range statuses {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			status.GetDependencyName(),
			status.GetCurrentTag(),
			status.GetTargetTag(),
			status.GetSeverity().String(),
			status.GetRecommendation().String(),
		); err != nil {
			return err
		}
	}

	return tw.Flush()
}

func newAcknowledgeDriftEventCmd(client v1.DriftServiceClient) *cobra.Command {
	ackCmd := &cobra.Command{
		Use:   "acknowledge [event_id]",
		Short: "Acknowledge a drift event",
		Long:  "Acknowledge is a command to mark a drift event as acknowledged",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			eventID := args[0]
			acknowledgedBy, err := cmd.Flags().GetString("by")
			if err != nil {
				return err
			}

			resp, err := client.AcknowledgeDriftEvent(cmd.Context(), &v1.AcknowledgeDriftEventRequest{
				EventId:        eventID,
				AcknowledgedBy: acknowledgedBy,
			})
			if err != nil {
				return err
			}

			return printJSON(cmd.OutOrStdout(), resp)
		},
	}

	ackCmd.Flags().String("by", "", "identifier of the user acknowledging the event")
	return ackCmd
}

// LocalDriftDiffItem represents a single schema change detected between a registry tag and local .proto files.
type LocalDriftDiffItem = utils.ProtoDiffItem

// LocalDriftDiffReport represents the full result of `pbuf drift diff`.
type LocalDriftDiffReport struct {
	ModuleName         string               `json:"module_name"`
	BaseTag            string               `json:"base_tag"`
	HasBreakingChanges bool                 `json:"has_breaking_changes"`
	Items              []LocalDriftDiffItem `json:"items"`
}

func runLocalDriftDiff(
	ctx context.Context,
	regClient v1.RegistryClient,
	fallbackCfg *model.Config,
	directory, moduleName, baseTag string,
) (LocalDriftDiffReport, int, bool, error) {
	if regClient == nil {
		return LocalDriftDiffReport{}, 0, false, fmt.Errorf("registry client is not configured")
	}

	rootDir := strings.TrimSpace(directory)
	if rootDir == "" {
		rootDir = "."
	}
	if info, err := os.Stat(rootDir); err != nil {
		return LocalDriftDiffReport{}, 0, false, fmt.Errorf("failed to access directory %s: %w", rootDir, err)
	} else if !info.IsDir() {
		return LocalDriftDiffReport{}, 0, false, fmt.Errorf("%s is not a directory", rootDir)
	}

	cfg := fallbackCfg
	cfgPath := filepath.Join(rootDir, model.PbufConfigFilename)
	if rawCfg, err := os.ReadFile(cfgPath); err == nil {
		parsed, parseErr := modules.NewConfig(rawCfg)
		if parseErr != nil {
			return LocalDriftDiffReport{}, 0, false, fmt.Errorf("failed to parse %s: %w", cfgPath, parseErr)
		}
		cfg = parsed
	} else if !os.IsNotExist(err) {
		return LocalDriftDiffReport{}, 0, false, fmt.Errorf("failed to read %s: %w", cfgPath, err)
	} else if filepath.Clean(rootDir) != "." {
		cfg = nil
	}

	moduleName = strings.TrimSpace(moduleName)
	if moduleName == "" && cfg != nil {
		moduleName = strings.TrimSpace(cfg.Name)
	}
	if moduleName == "" {
		return LocalDriftDiffReport{}, 0, false, fmt.Errorf("module name is required (set 'name' in pbuf.yaml or pass as argument)")
	}

	exportPaths := resolveLocalProtoPaths(rootDir, cfg)

	localFiles, err := registry.CollectProtoFilesInRoot(rootDir, exportPaths)
	if err != nil {
		return LocalDriftDiffReport{}, 0, false, fmt.Errorf("failed to collect local proto files: %w", err)
	}

	baseTag = strings.TrimSpace(baseTag)
	if baseTag == "" {
		mod, getErr := regClient.GetModule(ctx, &v1.GetModuleRequest{Name: moduleName})
		if getErr != nil {
			return LocalDriftDiffReport{}, 0, false, fmt.Errorf("failed to fetch module %s from registry: %w", moduleName, getErr)
		}
		if len(mod.GetTags()) == 0 {
			return LocalDriftDiffReport{
				ModuleName: moduleName,
				Items:      []LocalDriftDiffItem{},
			}, len(localFiles), true, nil
		}
		baseTag = mod.GetTags()[0]
	}

	pulled, err := regClient.PullModule(ctx, &v1.PullModuleRequest{
		Name: moduleName,
		Tag:  baseTag,
	})
	if err != nil {
		return LocalDriftDiffReport{}, 0, false, fmt.Errorf("failed to pull baseline %s@%s: %w", moduleName, baseTag, err)
	}

	items, err := computeLocalDriftDiff(pulled.GetProtofiles(), localFiles)
	if err != nil {
		return LocalDriftDiffReport{}, 0, false, err
	}
	if items == nil {
		items = []LocalDriftDiffItem{}
	}

	return LocalDriftDiffReport{
		ModuleName:         moduleName,
		BaseTag:            baseTag,
		HasBreakingChanges: shouldFailDriftDiff(items, 3),
		Items:              items,
	}, len(localFiles), false, nil
}

func resolveLocalProtoPaths(rootDir string, cfg *model.Config) []string {
	if cfg != nil && len(cfg.Export.Paths) > 0 {
		return cfg.Export.Paths
	}
	if rootDir == "" {
		rootDir = "."
	}
	var paths []string
	for _, dir := range []string{"api", "proto"} {
		if info, err := os.Stat(filepath.Join(rootDir, dir)); err == nil && info.IsDir() {
			paths = append(paths, dir)
		}
	}
	if len(paths) == 0 {
		return []string{"api"}
	}
	return paths
}

func registerLocalMCPTools(srv *mcp.Server, fallbackCfg *model.Config, regClient v1.RegistryClient) {
	srv.RegisterTool(
		"diff_local_protos",
		"Compare local .proto files against a published registry tag to detect wire-breaking and schema drift changes before pushing.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"directory":   map[string]any{"type": "string", "description": "Local project directory containing pbuf.yaml or .proto files (defaults to '.')."},
				"module_name": map[string]any{"type": "string", "description": "Module name in the registry (defaults to 'name' in pbuf.yaml)."},
				"against":     map[string]any{"type": "string", "description": "Baseline registry tag to compare against (defaults to latest published tag)."},
			},
		},
		func(ctx context.Context, rawArgs json.RawMessage) (string, error) {
			var args struct {
				Directory  string `json:"directory"`
				ModuleName string `json:"module_name"`
				Against    string `json:"against"`
			}
			if len(rawArgs) > 0 && string(rawArgs) != "null" {
				if err := json.Unmarshal(rawArgs, &args); err != nil {
					return "", &mcp.InvalidArgumentError{Message: err.Error()}
				}
			}
			report, _, _, err := runLocalDriftDiff(ctx, regClient, fallbackCfg, args.Directory, args.ModuleName, args.Against)
			if err != nil {
				return "", err
			}
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
	)
}

func newDriftDiffCmd(config *model.Config, regClient v1.RegistryClient) *cobra.Command {
	diffCmd := &cobra.Command{
		Use:     "diff [module_name]",
		Aliases: []string{"check"},
		Short:   "Compare local .proto files against a published registry tag",
		Long:    "Compare local .proto files in export.paths against the latest (or specified) registry tag and detect breaking changes before pushing",
		Args:    cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			argModule := ""
			if len(args) > 0 {
				argModule = args[0]
			}

			baseTag, err := cmd.Flags().GetString("against")
			if err != nil {
				return err
			}
			if baseTag == "" {
				baseTag, err = cmd.Flags().GetString("tag")
				if err != nil {
					return err
				}
			}

			format, err := cmd.Flags().GetString("format")
			if err != nil {
				return err
			}

			failOn, err := cmd.Flags().GetString("fail-on")
			if err != nil {
				return err
			}
			failThreshold, err := parseFailOnThreshold(failOn)
			if err != nil {
				return err
			}

			report, localCount, noPriorTags, err := runLocalDriftDiff(cmd.Context(), regClient, config, ".", argModule, baseTag)
			if err != nil {
				return err
			}
			if noPriorTags {
				if strings.EqualFold(format, "json") {
					return renderDriftDiffReport(cmd.OutOrStdout(), report, format)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No prior published tags found for %s; skipping diff (%d local file(s)).\n", report.ModuleName, localCount)
				return nil
			}

			if err := renderDriftDiffReport(cmd.OutOrStdout(), report, format); err != nil {
				return err
			}

			if shouldFailDriftDiff(report.Items, failThreshold) {
				return fmt.Errorf("schema drift check failed: detected changes meeting or exceeding %s severity", strings.ToUpper(failOn))
			}

			return nil
		},
	}

	diffCmd.Flags().String("against", "", "baseline tag in registry to compare against (defaults to latest published tag)")
	diffCmd.Flags().String("tag", "", "alias for --against")
	diffCmd.Flags().String("format", "table", "output format: table, markdown, or json")
	diffCmd.Flags().String("fail-on", "critical", "exit non-zero when changes meet severity: critical, warning, info, or none")

	return diffCmd
}

func computeLocalDriftDiff(baseFiles, localFiles []*v1.ProtoFile) ([]LocalDriftDiffItem, error) {
	baseParsedList, err := utils.ParseProtoFilesContents(baseFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to parse baseline proto files: %w", err)
	}
	localParsedList, err := utils.ParseProtoFilesContents(localFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to parse local proto files: %w", err)
	}

	baseContentMap := make(map[string]string, len(baseFiles))
	for _, f := range baseFiles {
		baseContentMap[f.GetFilename()] = f.GetContent()
	}
	localContentMap := make(map[string]string, len(localFiles))
	for _, f := range localFiles {
		localContentMap[f.GetFilename()] = f.GetContent()
	}

	baseParsed := make(map[string]*unordered.Proto, len(baseParsedList))
	for _, p := range baseParsedList {
		baseParsed[p.Filename] = p.Proto
	}
	localParsed := make(map[string]*unordered.Proto, len(localParsedList))
	for _, p := range localParsedList {
		localParsed[p.Filename] = p.Proto
	}

	var items []LocalDriftDiffItem

	// Check deleted and modified files
	baseNames := make([]string, 0, len(baseContentMap))
	for name := range baseContentMap {
		baseNames = append(baseNames, name)
	}
	sort.Strings(baseNames)

	for _, filename := range baseNames {
		localContent, exists := localContentMap[filename]
		if !exists {
			items = append(items, LocalDriftDiffItem{
				Filename: filename,
				Severity: "CRITICAL",
				Change:   "FILE_DELETED",
				Details:  "Proto file was deleted",
			})
			continue
		}
		if localContent == baseContentMap[filename] {
			continue
		}
		items = append(items, utils.DiffParsedProtoFile(filename, baseParsed[filename], localParsed[filename])...)
	}

	// Check added files
	localNames := make([]string, 0, len(localContentMap))
	for name := range localContentMap {
		localNames = append(localNames, name)
	}
	sort.Strings(localNames)

	for _, filename := range localNames {
		if _, exists := baseContentMap[filename]; !exists {
			items = append(items, LocalDriftDiffItem{
				Filename: filename,
				Severity: "INFO",
				Change:   "FILE_ADDED",
				Details:  "New proto file added",
			})
		}
	}

	return items, nil
}

func renderDriftDiffReport(w io.Writer, report LocalDriftDiffReport, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return printJSON(w, report)
	case "markdown", "md":
		if _, err := fmt.Fprintf(w, "### Schema Drift Diff: `%s` (against `%s`)\n\n", report.ModuleName, report.BaseTag); err != nil {
			return err
		}
		if len(report.Items) == 0 {
			_, err := fmt.Fprintln(w, "No schema changes detected.")
			return err
		}
		if _, err := fmt.Fprintln(w, "| Severity | File | Change | Details |\n| :--- | :--- | :--- | :--- |"); err != nil {
			return err
		}
		for _, item := range report.Items {
			if _, err := fmt.Fprintf(w, "| **%s** | `%s` | `%s` | %s |\n", item.Severity, item.Filename, item.Change, item.Details); err != nil {
				return err
			}
		}
		return nil
	default:
		if _, err := fmt.Fprintf(w, "Schema drift diff for %s (against %s)\n", report.ModuleName, report.BaseTag); err != nil {
			return err
		}
		if len(report.Items) == 0 {
			_, err := fmt.Fprintln(w, "No schema changes detected.")
			return err
		}
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "SEVERITY\tFILE\tCHANGE\tDETAILS"); err != nil {
			return err
		}
		for _, item := range report.Items {
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.Severity, item.Filename, item.Change, item.Details); err != nil {
				return err
			}
		}
		return tw.Flush()
	}
}

func shouldFailDriftDiff(items []LocalDriftDiffItem, threshold int) bool {
	if threshold == 0 {
		return false
	}
	for _, item := range items {
		if r, _ := parseFailOnThreshold(item.Severity); r >= threshold {
			return true
		}
	}
	return false
}

func parseFailOnThreshold(s string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return 3, nil
	case "warning", "warn":
		return 2, nil
	case "info":
		return 1, nil
	case "none", "":
		return 0, nil
	default:
		return 0, fmt.Errorf("invalid --fail-on value %q (must be critical, warning, info, or none)", s)
	}
}
