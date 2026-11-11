package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/registry"
	"github.com/spf13/cobra"
	protoparser "github.com/yoheimuta/go-protoparser/v4"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
	"github.com/yoheimuta/go-protoparser/v4/parser/meta"
)

var (
	pascalCaseRe     = regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)
	snakeCaseRe      = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	upperSnakeCaseRe = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)
)

// LintFinding is a single lint violation found in a .proto file.
type LintFinding struct {
	Filename string `json:"filename"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

// LintReport is the full result of a `pbuf lint` run.
type LintReport struct {
	Files    int           `json:"files"`
	Findings []LintFinding `json:"findings"`
}

// NewLintCmd creates the `pbuf lint` command. It validates local .proto files
// offline and never contacts a registry.
func NewLintCmd(config *model.Config) *cobra.Command {
	lintCmd := &cobra.Command{
		Use:   "lint [paths...]",
		Short: "Lint local .proto files offline",
		Long:  "Lint validates .proto syntax and common style rules in export.paths (or the given paths) without contacting a registry",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseLintFormat(cmd)
			if err != nil {
				return err
			}

			paths := args
			if len(paths) == 0 {
				paths = resolveLintPaths(config)
			}

			files, err := registry.CollectProtoFilesInDirs(paths)
			if err != nil {
				return fmt.Errorf("failed to collect proto files: %w", err)
			}

			report := lintProtoFiles(files)

			if err := renderLintReport(cmd.OutOrStdout(), report, format); err != nil {
				return err
			}

			if len(report.Findings) > 0 {
				return fmt.Errorf("lint failed: %d violation(s) found", len(report.Findings))
			}
			return nil
		},
	}

	lintCmd.Flags().String("format", "table", "output format: table|json")
	return lintCmd
}

func parseLintFormat(cmd *cobra.Command) (string, error) {
	raw, err := cmd.Flags().GetString("format")
	if err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "table", "":
		return "table", nil
	case "json":
		return "json", nil
	default:
		return "", fmt.Errorf("invalid --format value %q (must be table or json)", raw)
	}
}

// resolveLintPaths mirrors the default path resolution used by `pbuf drift diff`:
// prefer export.paths from pbuf.yaml, then fall back to api/ and proto/.
func resolveLintPaths(config *model.Config) []string {
	if config != nil && len(config.Export.Paths) > 0 {
		return config.Export.Paths
	}

	var paths []string
	for _, dir := range []string{"api", "proto"} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			paths = append(paths, dir)
		}
	}
	if len(paths) == 0 {
		paths = []string{"api"}
	}
	return paths
}

func lintProtoFiles(files []*v1.ProtoFile) LintReport {
	report := LintReport{Files: len(files)}
	for _, f := range files {
		report.Findings = append(report.Findings, lintProtoFile(f.GetFilename(), f.GetContent())...)
	}

	sort.SliceStable(report.Findings, func(i, j int) bool {
		a, b := report.Findings[i], report.Findings[j]
		if a.Filename != b.Filename {
			return a.Filename < b.Filename
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Rule < b.Rule
	})

	return report
}

type protoLinter struct {
	filename string
	findings []LintFinding
}

func (l *protoLinter) add(rule, message string, pos meta.Position) {
	l.findings = append(l.findings, LintFinding{
		Filename: l.filename,
		Line:     pos.Line,
		Column:   pos.Column,
		Rule:     rule,
		Message:  message,
	})
}

func lintProtoFile(filename, content string) []LintFinding {
	l := &protoLinter{filename: filename}

	if strings.TrimSpace(content) == "" {
		l.add("SYNTAX", "file is empty", meta.Position{})
		return l.findings
	}

	parsed, err := protoparser.Parse(strings.NewReader(content), protoparser.WithFilename(filename))
	if err != nil {
		l.add("SYNTAX", fmt.Sprintf("failed to parse: %v", err), meta.Position{})
		return l.findings
	}

	proto, err := unordered.InterpretProto(parsed)
	if err != nil {
		l.add("SYNTAX", fmt.Sprintf("failed to interpret: %v", err), meta.Position{})
		return l.findings
	}

	if proto.Syntax == nil ||
		(proto.Syntax.ProtobufVersion != "proto3" && proto.Syntax.ProtobufVersion != "proto2") {
		var pos meta.Position
		if proto.Syntax != nil {
			pos = proto.Syntax.Meta.Pos
		}
		l.add("SYNTAX", "missing explicit syntax (expected proto3 or proto2)", pos)
	}

	body := proto.ProtoBody
	if body == nil {
		return l.findings
	}

	if len(body.Packages) == 0 || strings.TrimSpace(body.Packages[0].Name) == "" {
		var pos meta.Position
		if len(body.Packages) > 0 {
			pos = body.Packages[0].Meta.Pos
		}
		l.add("PACKAGE", "missing non-empty package declaration", pos)
	}

	l.lintMessages(body.Messages)
	l.lintEnums(body.Enums)
	l.lintServices(body.Services)

	return l.findings
}

func (l *protoLinter) lintMessages(messages []*unordered.Message) {
	for _, m := range messages {
		if !pascalCaseRe.MatchString(m.MessageName) {
			l.add("MESSAGE_NAME", fmt.Sprintf("message %q should be PascalCase", m.MessageName), m.Meta.Pos)
		}
		if m.MessageBody == nil {
			continue
		}
		for _, f := range m.MessageBody.Fields {
			if !snakeCaseRe.MatchString(f.FieldName) {
				l.add("FIELD_NAME", fmt.Sprintf("field %q should be snake_case", f.FieldName), f.Meta.Pos)
			}
		}
		for _, mf := range m.MessageBody.Maps {
			if !snakeCaseRe.MatchString(mf.MapName) {
				l.add("FIELD_NAME", fmt.Sprintf("field %q should be snake_case", mf.MapName), mf.Meta.Pos)
			}
		}
		for _, oo := range m.MessageBody.Oneofs {
			for _, oof := range oo.OneofFields {
				if !snakeCaseRe.MatchString(oof.FieldName) {
					l.add("FIELD_NAME", fmt.Sprintf("field %q should be snake_case", oof.FieldName), oof.Meta.Pos)
				}
			}
		}
		l.lintEnums(m.MessageBody.Enums)
		l.lintMessages(m.MessageBody.Messages)
	}
}

func (l *protoLinter) lintEnums(enums []*unordered.Enum) {
	for _, e := range enums {
		if !pascalCaseRe.MatchString(e.EnumName) {
			l.add("ENUM_NAME", fmt.Sprintf("enum %q should be PascalCase", e.EnumName), e.Meta.Pos)
		}
		if e.EnumBody == nil {
			continue
		}
		hasZero := false
		for _, ev := range e.EnumBody.EnumFields {
			if !upperSnakeCaseRe.MatchString(ev.Ident) {
				l.add("ENUM_VALUE_NAME", fmt.Sprintf("enum value %q should be UPPER_SNAKE_CASE", ev.Ident), ev.Meta.Pos)
			}
			if n, err := strconv.ParseInt(ev.Number, 10, 32); err == nil && n == 0 {
				hasZero = true
			}
		}
		if !hasZero {
			l.add("ENUM_ZERO_VALUE", fmt.Sprintf("enum %q must define a value with number 0", e.EnumName), e.Meta.Pos)
		}
	}
}

func (l *protoLinter) lintServices(services []*unordered.Service) {
	for _, s := range services {
		if !pascalCaseRe.MatchString(s.ServiceName) {
			l.add("SERVICE_NAME", fmt.Sprintf("service %q should be PascalCase", s.ServiceName), s.Meta.Pos)
		}
		if s.ServiceBody == nil {
			continue
		}
		for _, r := range s.ServiceBody.RPCs {
			if !pascalCaseRe.MatchString(r.RPCName) {
				l.add("RPC_NAME", fmt.Sprintf("rpc %q should be PascalCase", r.RPCName), r.Meta.Pos)
			}
		}
	}
}

func renderLintReport(w io.Writer, report LintReport, format string) error {
	if strings.ToLower(format) == "json" {
		if report.Findings == nil {
			report.Findings = []LintFinding{}
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal lint report: %w", err)
		}
		_, err = fmt.Fprintln(w, string(data))
		return err
	}

	if len(report.Findings) == 0 {
		_, err := fmt.Fprintf(w, "Linted %d file(s). No issues found.\n", report.Files)
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "FILE\tLINE\tRULE\tMESSAGE"); err != nil {
		return err
	}
	for _, item := range report.Findings {
		if _, err := fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", item.Filename, item.Line, item.Rule, item.Message); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	_, err := fmt.Fprintf(w, "\n%d issue(s) found across %d file(s).\n", len(report.Findings), report.Files)
	return err
}
