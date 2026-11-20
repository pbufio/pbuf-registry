package utils

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/yoheimuta/go-protoparser/v4"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
	"github.com/yoheimuta/go-protoparser/v4/parser"
)

// ProtoDiffItem represents a single schema change detected between two parsed .proto files.
type ProtoDiffItem struct {
	Filename string `json:"filename"`
	Severity string `json:"severity"`
	Change   string `json:"change"`
	Details  string `json:"details"`
}

func ParseProtoFilesContents(protoFiles []*v1.ProtoFile) ([]*model.ParsedProtoFile, error) {
	parsedProtoFiles := make([]*model.ParsedProtoFile, len(protoFiles))

	for i, protoFile := range protoFiles {
		if protoFile.Content == "" {
			return parsedProtoFiles, fmt.Errorf("content of %s cannot be empty", protoFile.Filename)
		}

		// check that file contents is valid
		parsed, err := parseProtoFile(protoFile.Content)
		if err != nil {
			return parsedProtoFiles, fmt.Errorf("invalid proto file %s: %w", protoFile.Filename, err)
		}

		proto, err := unordered.InterpretProto(parsed)
		if err != nil {
			return parsedProtoFiles, fmt.Errorf("cannot interpret file %s: %w", protoFile.Filename, err)
		}

		jsonContent, err := json.Marshal(proto)
		if err != nil {
			return parsedProtoFiles, fmt.Errorf("cannot marshall %s: %w", protoFile.Filename, err)
		}

		parsedProtoFiles[i] = &model.ParsedProtoFile{
			Filename:  protoFile.Filename,
			Proto:     proto,
			ProtoJson: string(jsonContent),
		}
	}

	return parsedProtoFiles, nil
}

func parseProtoFile(content string) (*parser.Proto, error) {
	parsed, err := protoparser.Parse(strings.NewReader(content))
	if err != nil {
		return nil, err
	}

	return parsed, nil
}

// DiffParsedProtoFile compares two parsed .proto syntax trees and returns all detected changes.
func DiffParsedProtoFile(filename string, prev, curr *unordered.Proto) []ProtoDiffItem {
	if prev == nil || curr == nil || prev.ProtoBody == nil || curr.ProtoBody == nil {
		return []ProtoDiffItem{{
			Filename: filename,
			Severity: "WARNING",
			Change:   "FILE_MODIFIED",
			Details:  "Proto file modified",
		}}
	}

	var items []ProtoDiffItem
	prevBody := prev.ProtoBody
	currBody := curr.ProtoBody

	prevPkg := firstPackageName(prevBody)
	currPkg := firstPackageName(currBody)
	if prevPkg != currPkg {
		items = append(items, ProtoDiffItem{
			Filename: filename,
			Severity: "CRITICAL",
			Change:   "PACKAGE_CHANGED",
			Details:  fmt.Sprintf("package changed from %q to %q", prevPkg, currPkg),
		})
	}

	prevSym := collectProtoSymbols(prevBody)
	currSym := collectProtoSymbols(currBody)

	for _, msg := range sortedMissingKeys(prevSym.msgs, currSym.msgs) {
		if idx := strings.LastIndexByte(msg, '.'); idx >= 0 && !currSym.msgs[msg[:idx]] {
			continue
		}
		items = append(items, ProtoDiffItem{
			Filename: filename,
			Severity: "CRITICAL",
			Change:   "MESSAGE_REMOVED",
			Details:  fmt.Sprintf("message %s was removed", msg),
		})
	}

	for _, svc := range sortedMissingKeys(prevSym.svcs, currSym.svcs) {
		items = append(items, ProtoDiffItem{
			Filename: filename,
			Severity: "CRITICAL",
			Change:   "SERVICE_REMOVED",
			Details:  fmt.Sprintf("service %s was removed", svc),
		})
	}

	for _, enumName := range sortedMissingKeys(prevSym.enums, currSym.enums) {
		if idx := strings.LastIndexByte(enumName, '.'); idx >= 0 && !currSym.msgs[enumName[:idx]] {
			continue
		}
		items = append(items, ProtoDiffItem{
			Filename: filename,
			Severity: "CRITICAL",
			Change:   "ENUM_REMOVED",
			Details:  fmt.Sprintf("enum %s was removed", enumName),
		})
	}

	for _, evKey := range sortedKeys(prevSym.enumVals) {
		if enumName, _, ok := strings.Cut(evKey, "#"); ok && !currSym.enums[enumName] {
			continue
		}
		currIdents := currSym.enumVals[evKey]
		for _, ident := range prevSym.enumVals[evKey] {
			if !slices.Contains(currIdents, ident) {
				items = append(items, ProtoDiffItem{
					Filename: filename,
					Severity: "CRITICAL",
					Change:   "ENUM_VALUE_REMOVED",
					Details:  fmt.Sprintf("enum value %s (%s) was removed", evKey, ident),
				})
			}
		}
	}

	for _, fk := range sortedKeys(prevSym.fields) {
		if msgName, _, ok := strings.Cut(fk, "#"); ok && !currSym.msgs[msgName] {
			continue
		}
		pf := prevSym.fields[fk]
		cf, exists := currSym.fields[fk]
		if !exists {
			items = append(items, ProtoDiffItem{
				Filename: filename,
				Severity: "CRITICAL",
				Change:   "FIELD_REMOVED",
				Details:  fmt.Sprintf("field %s (%s %s) was removed", fk, pf.Type, pf.Name),
			})
			continue
		}
		if pf.Type != cf.Type {
			items = append(items, ProtoDiffItem{
				Filename: filename,
				Severity: "CRITICAL",
				Change:   "FIELD_TYPE_CHANGED",
				Details:  fmt.Sprintf("field %s (%s) type changed from %s to %s", fk, pf.Name, pf.Type, cf.Type),
			})
		} else if pf.cardinality() != cf.cardinality() {
			items = append(items, ProtoDiffItem{
				Filename: filename,
				Severity: "CRITICAL",
				Change:   "FIELD_CARDINALITY_CHANGED",
				Details:  fmt.Sprintf("field %s (%s) cardinality changed from %s to %s", fk, pf.Name, pf.cardinality(), cf.cardinality()),
			})
		} else if pf.Name != cf.Name {
			items = append(items, ProtoDiffItem{
				Filename: filename,
				Severity: "WARNING",
				Change:   "FIELD_RENAMED",
				Details:  fmt.Sprintf("field %s renamed from %s to %s", fk, pf.Name, cf.Name),
			})
		}
	}

	for _, fk := range sortedKeys(currSym.fields) {
		cf := currSym.fields[fk]
		if _, existed := prevSym.fields[fk]; cf.Required && !existed {
			items = append(items, ProtoDiffItem{
				Filename: filename,
				Severity: "WARNING",
				Change:   "REQUIRED_FIELD_ADDED",
				Details:  "new required field added",
			})
			break
		}
	}

	for _, rk := range sortedKeys(prevSym.rpcs) {
		if svcName, _, ok := strings.Cut(rk, "."); ok && !currSym.svcs[svcName] {
			continue
		}
		pr := prevSym.rpcs[rk]
		cr, exists := currSym.rpcs[rk]
		if !exists {
			items = append(items, ProtoDiffItem{
				Filename: filename,
				Severity: "CRITICAL",
				Change:   "RPC_REMOVED",
				Details:  fmt.Sprintf("rpc %s was removed", rk),
			})
			continue
		}
		if pr != cr {
			items = append(items, ProtoDiffItem{
				Filename: filename,
				Severity: "CRITICAL",
				Change:   "RPC_SIGNATURE_CHANGED",
				Details:  fmt.Sprintf("rpc %s signature changed from %s to %s", rk, pr.formatSig(), cr.formatSig()),
			})
		}
	}

	if len(items) == 0 {
		items = append(items, ProtoDiffItem{
			Filename: filename,
			Severity: "INFO",
			Change:   "FILE_MODIFIED",
			Details:  "Backward-compatible schema update",
		})
	}

	return items
}

type protoFieldInfo struct {
	Name     string
	Type     string
	Repeated bool
	Required bool
}

func (f protoFieldInfo) cardinality() string {
	switch {
	case f.Repeated:
		return "repeated"
	case f.Required:
		return "required"
	default:
		return "singular"
	}
}

type protoRPCInfo struct {
	Req        string
	Resp       string
	ReqStream  bool
	RespStream bool
}

func (r protoRPCInfo) formatSig() string {
	req := r.Req
	if r.ReqStream {
		req = "stream " + req
	}
	resp := r.Resp
	if r.RespStream {
		resp = "stream " + resp
	}
	return fmt.Sprintf("(%s)->(%s)", req, resp)
}

func firstPackageName(body *unordered.ProtoBody) string {
	if body != nil && len(body.Packages) > 0 && body.Packages[0] != nil {
		return body.Packages[0].Name
	}
	return ""
}

type protoSymbols struct {
	msgs     map[string]bool
	svcs     map[string]bool
	enums    map[string]bool
	enumVals map[string][]string
	fields   map[string]protoFieldInfo
	rpcs     map[string]protoRPCInfo
}

func protoTagKey(scope, raw string) (string, bool) {
	n, err := strconv.ParseInt(raw, 0, 32)
	if err != nil {
		return "", false
	}
	return scope + "#" + strconv.FormatInt(n, 10), true
}

func collectProtoSymbols(body *unordered.ProtoBody) protoSymbols {
	sym := protoSymbols{
		msgs:     make(map[string]bool),
		svcs:     make(map[string]bool),
		enums:    make(map[string]bool),
		enumVals: make(map[string][]string),
		fields:   make(map[string]protoFieldInfo),
		rpcs:     make(map[string]protoRPCInfo),
	}
	if body == nil {
		return sym
	}

	walkEnums := func(list []*unordered.Enum, prefix string) {
		for _, e := range list {
			if e == nil {
				continue
			}
			enumName := e.EnumName
			if prefix != "" {
				enumName = prefix + "." + e.EnumName
			}
			sym.enums[enumName] = true
			if e.EnumBody == nil {
				continue
			}
			for _, f := range e.EnumBody.EnumFields {
				if f == nil {
					continue
				}
				if key, ok := protoTagKey(enumName, f.Number); ok {
					sym.enumVals[key] = append(sym.enumVals[key], f.Ident)
				}
			}
		}
	}

	var walkMsgs func(list []*unordered.Message, prefix string)
	walkMsgs = func(list []*unordered.Message, prefix string) {
		for _, m := range list {
			if m == nil {
				continue
			}
			msgName := m.MessageName
			if prefix != "" {
				msgName = prefix + "." + m.MessageName
			}
			sym.msgs[msgName] = true
			if m.MessageBody == nil {
				continue
			}
			for _, f := range m.MessageBody.Fields {
				if f == nil {
					continue
				}
				if key, ok := protoTagKey(msgName, f.FieldNumber); ok {
					sym.fields[key] = protoFieldInfo{
						Name:     f.FieldName,
						Type:     f.Type,
						Repeated: f.IsRepeated,
						Required: f.IsRequired,
					}
				}
			}
			for _, mf := range m.MessageBody.Maps {
				if mf == nil {
					continue
				}
				if key, ok := protoTagKey(msgName, mf.FieldNumber); ok {
					sym.fields[key] = protoFieldInfo{Name: mf.MapName, Type: "map<" + mf.KeyType + "," + mf.Type + ">"}
				}
			}
			for _, oo := range m.MessageBody.Oneofs {
				if oo == nil {
					continue
				}
				for _, oof := range oo.OneofFields {
					if oof == nil {
						continue
					}
					if key, ok := protoTagKey(msgName, oof.FieldNumber); ok {
						sym.fields[key] = protoFieldInfo{Name: oof.FieldName, Type: oof.Type}
					}
				}
			}
			walkEnums(m.MessageBody.Enums, msgName)
			walkMsgs(m.MessageBody.Messages, msgName)
		}
	}

	walkEnums(body.Enums, "")
	walkMsgs(body.Messages, "")

	for _, s := range body.Services {
		if s == nil {
			continue
		}
		sym.svcs[s.ServiceName] = true
		if s.ServiceBody == nil {
			continue
		}
		for _, r := range s.ServiceBody.RPCs {
			if r == nil {
				continue
			}
			var info protoRPCInfo
			if r.RPCRequest != nil {
				info.Req = r.RPCRequest.MessageType
				info.ReqStream = r.RPCRequest.IsStream
			}
			if r.RPCResponse != nil {
				info.Resp = r.RPCResponse.MessageType
				info.RespStream = r.RPCResponse.IsStream
			}
			sym.rpcs[s.ServiceName+"."+r.RPCName] = info
		}
	}

	return sym
}

func sortedMissingKeys(prev, curr map[string]bool) []string {
	var missing []string
	for k := range prev {
		if !curr[k] {
			missing = append(missing, k)
		}
	}
	slices.Sort(missing)
	return missing
}
