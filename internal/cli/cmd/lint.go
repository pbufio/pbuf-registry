package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
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
		Use:           "lint [paths...]",
		Short:         "Lint local .proto files offline",
		Long:          "Lint validates .proto syntax and common style rules in export.paths (or the given paths) without contacting a registry",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseLintOutputFormat(cmd)
			if err != nil {
				return err
			}

			paths := args
			if len(paths) == 0 {
				paths = resolveLocalProtoPaths(".", config)
			}

			files, err := collectLintProtoFiles(paths)
			if err != nil {
				return fmt.Errorf("failed to collect proto files: %w", err)
			}
			if len(files) == 0 {
				return fmt.Errorf("no .proto files found in %s", strings.Join(paths, ", "))
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

	lintCmd.Flags().StringP("output", "o", "table", "output format: table|json")
	lintCmd.Flags().String("format", "table", "output format: table|json")
	return lintCmd
}

func parseLintOutputFormat(cmd *cobra.Command) (string, error) {
	outChanged := cmd.Flags().Changed("output")
	fmtChanged := cmd.Flags().Changed("format")
	if outChanged && fmtChanged {
		outFmt, err := parseFlagFormat(cmd, "output")
		if err != nil {
			return "", err
		}
		altFmt, err := parseFlagFormat(cmd, "format")
		if err != nil {
			return "", err
		}
		if outFmt != altFmt {
			return "", fmt.Errorf("conflicting --output (%s) and --format (%s) values", outFmt, altFmt)
		}
		return outFmt, nil
	}
	if fmtChanged {
		return parseFlagFormat(cmd, "format")
	}
	return parseFlagFormat(cmd, "output")
}

func canonicalFileKey(path string) (string, error) {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		if abs, absErr := filepath.Abs(resolved); absErr == nil {
			return filepath.Clean(abs), nil
		}
		return filepath.Clean(resolved), nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func collectLintProtoFiles(paths []string) ([]*v1.ProtoFile, error) {
	var files []*v1.ProtoFile
	seenFiles := make(map[string]struct{})
	activeDirs := make(map[string]struct{})

	addFile := func(path string) error {
		key, err := canonicalFileKey(path)
		if err != nil {
			return err
		}
		if _, dup := seenFiles[key]; dup {
			return nil
		}
		seenFiles[key] = struct{}{}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, &v1.ProtoFile{
			Filename: filepath.ToSlash(filepath.Clean(path)),
			Content:  string(content),
		})
		return nil
	}

	var walkDir func(dir string) error
	walkDir = func(dir string) error {
		dirKey, err := canonicalFileKey(dir)
		if err != nil {
			return err
		}
		if _, cycle := activeDirs[dirKey]; cycle {
			return nil
		}
		activeDirs[dirKey] = struct{}{}
		defer delete(activeDirs, dirKey)

		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			child := filepath.Join(dir, entry.Name())
			info, statErr := os.Stat(child)
			if statErr != nil {
				if strings.HasSuffix(entry.Name(), ".proto") {
					return statErr
				}
				continue
			}
			if info.IsDir() {
				if err := walkDir(child); err != nil {
					return err
				}
				continue
			}
			if info.Mode().IsRegular() && strings.HasSuffix(entry.Name(), ".proto") {
				if err := addFile(child); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, target := range paths {
		info, err := os.Stat(target)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			if err := walkDir(target); err != nil {
				return nil, err
			}
			continue
		}
		if !info.Mode().IsRegular() || !strings.HasSuffix(target, ".proto") {
			return nil, fmt.Errorf("%s is not a regular .proto file", target)
		}
		if err := addFile(target); err != nil {
			return nil, err
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].GetFilename() < files[j].GetFilename()
	})
	return files, nil
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
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Message < b.Message
	})

	return report
}

type protoLinter struct {
	filename string
	syntax   string
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

	switch {
	case parsed.Edition != nil:
		l.add("SYNTAX", "editions are not supported (expected syntax proto3 or proto2)", parsed.Edition.Meta.Pos)
	case proto.Syntax == nil:
		l.add("SYNTAX", "missing explicit syntax (expected proto3 or proto2)", meta.Position{})
	case proto.Syntax.ProtobufVersion != "proto3" && proto.Syntax.ProtobufVersion != "proto2":
		l.add("SYNTAX", fmt.Sprintf("unsupported syntax %q (expected proto3 or proto2)", proto.Syntax.ProtobufVersion), proto.Syntax.Meta.Pos)
	default:
		l.syntax = proto.Syntax.ProtobufVersion
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
	for _, pkg := range body.Packages[1:] {
		l.add("PACKAGE", "multiple package declarations are not allowed", pkg.Meta.Pos)
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
			if !snakeCaseRe.MatchString(oo.OneofName) {
				l.add("FIELD_NAME", fmt.Sprintf("oneof %q should be snake_case", oo.OneofName), oo.Meta.Pos)
			}
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
		for _, ev := range e.EnumBody.EnumFields {
			if !upperSnakeCaseRe.MatchString(ev.Ident) {
				l.add("ENUM_VALUE_NAME", fmt.Sprintf("enum value %q should be UPPER_SNAKE_CASE", ev.Ident), ev.Meta.Pos)
			}
		}
		if l.syntax == "proto3" {
			if len(e.EnumBody.EnumFields) == 0 {
				l.add("ENUM_ZERO_VALUE", fmt.Sprintf("enum %q first value must have number 0 in proto3", e.EnumName), e.Meta.Pos)
				continue
			}
			first := e.EnumBody.EnumFields[0]
			if n, err := strconv.ParseInt(first.Number, 0, 32); err != nil || n != 0 {
				l.add("ENUM_ZERO_VALUE", fmt.Sprintf("enum %q first value must have number 0 in proto3", e.EnumName), first.Meta.Pos)
			}
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
		return printJSON(w, report)
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
