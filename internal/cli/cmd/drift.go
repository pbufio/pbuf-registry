package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/registry"
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

			return printJSON(resp)
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

			return printJSON(resp)
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

			return printJSON(resp)
		},
	}

	ackCmd.Flags().String("by", "", "identifier of the user acknowledging the event")
	return ackCmd
}

// LocalDriftDiffItem represents a single schema change detected between a registry tag and local .proto files.
type LocalDriftDiffItem struct {
	Filename string `json:"filename"`
	Severity string `json:"severity"`
	Change   string `json:"change"`
	Details  string `json:"details"`
}

// LocalDriftDiffReport represents the full result of `pbuf drift diff`.
type LocalDriftDiffReport struct {
	ModuleName string               `json:"module_name"`
	BaseTag    string               `json:"base_tag"`
	Items      []LocalDriftDiffItem `json:"items"`
}

func newDriftDiffCmd(config *model.Config, regClient v1.RegistryClient) *cobra.Command {
	diffCmd := &cobra.Command{
		Use:     "diff [module_name]",
		Aliases: []string{"check"},
		Short:   "Compare local .proto files against a published registry tag",
		Long:    "Compare local .proto files in export.paths against the latest (or specified) registry tag and detect breaking changes before pushing",
		Args:    cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if regClient == nil {
				return fmt.Errorf("registry client is not configured")
			}

			moduleName := ""
			if config != nil {
				moduleName = config.Name
			}
			if len(args) > 0 && args[0] != "" {
				moduleName = args[0]
			}
			if moduleName == "" {
				return fmt.Errorf("module name is required (set 'name' in pbuf.yaml or pass as argument)")
			}

			exportPaths := []string{"api", "proto"}
			if config != nil && len(config.Export.Paths) > 0 {
				exportPaths = config.Export.Paths
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

			localFiles, err := registry.CollectProtoFilesInDirs(exportPaths)
			if err != nil {
				return fmt.Errorf("failed to collect local proto files: %w", err)
			}

			if baseTag == "" {
				mod, getErr := regClient.GetModule(cmd.Context(), &v1.GetModuleRequest{Name: moduleName})
				if getErr != nil {
					return fmt.Errorf("failed to fetch module %s from registry: %w", moduleName, getErr)
				}
				if len(mod.GetTags()) == 0 {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No prior published tags found for %s; skipping diff (%d local file(s)).\n", moduleName, len(localFiles))
					return nil
				}
				baseTag = mod.GetTags()[0]
			}

			pulled, err := regClient.PullModule(cmd.Context(), &v1.PullModuleRequest{
				Name: moduleName,
				Tag:  baseTag,
			})
			if err != nil {
				return fmt.Errorf("failed to pull baseline %s@%s: %w", moduleName, baseTag, err)
			}

			items, err := computeLocalDriftDiff(pulled.GetProtofiles(), localFiles)
			if err != nil {
				return err
			}

			report := LocalDriftDiffReport{
				ModuleName: moduleName,
				BaseTag:    baseTag,
				Items:      items,
			}

			if err := renderDriftDiffReport(cmd.OutOrStdout(), report, format); err != nil {
				return err
			}

			if shouldFailDriftDiff(items, failOn) {
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
		fileItems := diffParsedProtoFile(filename, baseParsed[filename], localParsed[filename])
		items = append(items, fileItems...)
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

func diffParsedProtoFile(filename string, prev, curr *unordered.Proto) []LocalDriftDiffItem {
	if prev == nil || curr == nil || prev.ProtoBody == nil || curr.ProtoBody == nil {
		return []LocalDriftDiffItem{{
			Filename: filename,
			Severity: "WARNING",
			Change:   "FILE_MODIFIED",
			Details:  "Proto file modified",
		}}
	}

	var items []LocalDriftDiffItem
	prevBody := prev.ProtoBody
	currBody := curr.ProtoBody

	prevPkg := firstPackageName(prevBody)
	currPkg := firstPackageName(currBody)
	if prevPkg != currPkg {
		items = append(items, LocalDriftDiffItem{
			Filename: filename,
			Severity: "CRITICAL",
			Change:   "PACKAGE_CHANGED",
			Details:  fmt.Sprintf("package changed from %q to %q", prevPkg, currPkg),
		})
	}

	prevMsgs := collectMessageNames(prevBody.Messages, "")
	currMsgs := collectMessageNames(currBody.Messages, "")
	for _, msg := range sortedMissingKeys(prevMsgs, currMsgs) {
		items = append(items, LocalDriftDiffItem{
			Filename: filename,
			Severity: "CRITICAL",
			Change:   "MESSAGE_REMOVED",
			Details:  fmt.Sprintf("message %s was removed", msg),
		})
	}

	prevSvcs := make(map[string]bool)
	for _, s := range prevBody.Services {
		prevSvcs[s.ServiceName] = true
	}
	currSvcs := make(map[string]bool)
	for _, s := range currBody.Services {
		currSvcs[s.ServiceName] = true
	}
	for _, svc := range sortedMissingKeys(prevSvcs, currSvcs) {
		items = append(items, LocalDriftDiffItem{
			Filename: filename,
			Severity: "CRITICAL",
			Change:   "SERVICE_REMOVED",
			Details:  fmt.Sprintf("service %s was removed", svc),
		})
	}

	prevEnums := make(map[string]bool)
	for _, e := range prevBody.Enums {
		prevEnums[e.EnumName] = true
	}
	currEnums := make(map[string]bool)
	for _, e := range currBody.Enums {
		currEnums[e.EnumName] = true
	}
	for _, enumName := range sortedMissingKeys(prevEnums, currEnums) {
		items = append(items, LocalDriftDiffItem{
			Filename: filename,
			Severity: "CRITICAL",
			Change:   "ENUM_REMOVED",
			Details:  fmt.Sprintf("enum %s was removed", enumName),
		})
	}

	prevFields := collectFieldsByTagNumber(prevBody.Messages, "")
	currFields := collectFieldsByTagNumber(currBody.Messages, "")
	fieldKeys := make([]string, 0, len(prevFields))
	for k := range prevFields {
		fieldKeys = append(fieldKeys, k)
	}
	sort.Strings(fieldKeys)

	for _, fk := range fieldKeys {
		pf := prevFields[fk]
		cf, exists := currFields[fk]
		if !exists {
			items = append(items, LocalDriftDiffItem{
				Filename: filename,
				Severity: "CRITICAL",
				Change:   "FIELD_REMOVED",
				Details:  fmt.Sprintf("field %s (%s %s) was removed", fk, pf.Type, pf.Name),
			})
			continue
		}
		if pf.Type != cf.Type {
			items = append(items, LocalDriftDiffItem{
				Filename: filename,
				Severity: "CRITICAL",
				Change:   "FIELD_TYPE_CHANGED",
				Details:  fmt.Sprintf("field %s (%s) type changed from %s to %s", fk, pf.Name, pf.Type, cf.Type),
			})
		} else if pf.Name != cf.Name {
			items = append(items, LocalDriftDiffItem{
				Filename: filename,
				Severity: "WARNING",
				Change:   "FIELD_RENAMED",
				Details:  fmt.Sprintf("field %s renamed from %s to %s", fk, pf.Name, cf.Name),
			})
		}
	}

	prevRPCs := collectRPCDetails(prevBody.Services)
	currRPCs := collectRPCDetails(currBody.Services)
	rpcKeys := make([]string, 0, len(prevRPCs))
	for k := range prevRPCs {
		rpcKeys = append(rpcKeys, k)
	}
	sort.Strings(rpcKeys)

	for _, rk := range rpcKeys {
		pr := prevRPCs[rk]
		cr, exists := currRPCs[rk]
		if !exists {
			items = append(items, LocalDriftDiffItem{
				Filename: filename,
				Severity: "CRITICAL",
				Change:   "RPC_REMOVED",
				Details:  fmt.Sprintf("rpc %s was removed", rk),
			})
			continue
		}
		if pr != cr {
			items = append(items, LocalDriftDiffItem{
				Filename: filename,
				Severity: "CRITICAL",
				Change:   "RPC_SIGNATURE_CHANGED",
				Details:  fmt.Sprintf("rpc %s signature changed from (%s)->(%s) to (%s)->(%s)", rk, pr.Req, pr.Resp, cr.Req, cr.Resp),
			})
		}
	}

	if len(items) == 0 {
		items = append(items, LocalDriftDiffItem{
			Filename: filename,
			Severity: "INFO",
			Change:   "FILE_MODIFIED",
			Details:  "Backward-compatible schema update",
		})
	}

	return items
}

type cliFieldInfo struct {
	Name string
	Type string
}

type cliRPCInfo struct {
	Req        string
	Resp       string
	ReqStream  bool
	RespStream bool
}

func firstPackageName(body *unordered.ProtoBody) string {
	if body != nil && len(body.Packages) > 0 {
		return body.Packages[0].Name
	}
	return ""
}

func collectMessageNames(messages []*unordered.Message, prefix string) map[string]bool {
	res := make(map[string]bool)
	for _, m := range messages {
		name := m.MessageName
		if prefix != "" {
			name = prefix + "." + m.MessageName
		}
		res[name] = true
		if m.MessageBody != nil {
			for k, v := range collectMessageNames(m.MessageBody.Messages, name) {
				res[k] = v
			}
		}
	}
	return res
}

func collectFieldsByTagNumber(messages []*unordered.Message, prefix string) map[string]cliFieldInfo {
	res := make(map[string]cliFieldInfo)
	for _, m := range messages {
		msgName := m.MessageName
		if prefix != "" {
			msgName = prefix + "." + m.MessageName
		}
		if m.MessageBody == nil {
			continue
		}
		for _, f := range m.MessageBody.Fields {
			if _, err := strconv.ParseInt(f.FieldNumber, 10, 32); err == nil {
				res[msgName+"#"+f.FieldNumber] = cliFieldInfo{Name: f.FieldName, Type: f.Type}
			}
		}
		for _, mf := range m.MessageBody.Maps {
			if _, err := strconv.ParseInt(mf.FieldNumber, 10, 32); err == nil {
				res[msgName+"#"+mf.FieldNumber] = cliFieldInfo{Name: mf.MapName, Type: "map<" + mf.KeyType + "," + mf.Type + ">"}
			}
		}
		for _, oo := range m.MessageBody.Oneofs {
			for _, oof := range oo.OneofFields {
				if _, err := strconv.ParseInt(oof.FieldNumber, 10, 32); err == nil {
					res[msgName+"#"+oof.FieldNumber] = cliFieldInfo{Name: oof.FieldName, Type: oof.Type}
				}
			}
		}
		for k, v := range collectFieldsByTagNumber(m.MessageBody.Messages, msgName) {
			res[k] = v
		}
	}
	return res
}

func collectRPCDetails(services []*unordered.Service) map[string]cliRPCInfo {
	res := make(map[string]cliRPCInfo)
	for _, s := range services {
		if s.ServiceBody == nil {
			continue
		}
		for _, r := range s.ServiceBody.RPCs {
			var info cliRPCInfo
			if r.RPCRequest != nil {
				info.Req = r.RPCRequest.MessageType
				info.ReqStream = r.RPCRequest.IsStream
			}
			if r.RPCResponse != nil {
				info.Resp = r.RPCResponse.MessageType
				info.RespStream = r.RPCResponse.IsStream
			}
			res[s.ServiceName+"."+r.RPCName] = info
		}
	}
	return res
}

func sortedMissingKeys(prev, curr map[string]bool) []string {
	var missing []string
	for k := range prev {
		if !curr[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}

func renderDriftDiffReport(w io.Writer, report LocalDriftDiffReport, format string) error {
	switch strings.ToLower(format) {
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(data))
		return err
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

func shouldFailDriftDiff(items []LocalDriftDiffItem, failOn string) bool {
	threshold := severityRank(failOn)
	if threshold == 0 {
		return false
	}
	for _, item := range items {
		if severityRank(item.Severity) >= threshold {
			return true
		}
	}
	return false
}

func severityRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return 3
	case "warning", "warn":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}
