package utils

import (
	"encoding/json"
	"fmt"
	"sort"
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

	prevMsgs := collectMessageNames(prevBody.Messages, "")
	currMsgs := collectMessageNames(currBody.Messages, "")
	for _, msg := range sortedMissingKeys(prevMsgs, currMsgs) {
		items = append(items, ProtoDiffItem{
			Filename: filename,
			Severity: "CRITICAL",
			Change:   "MESSAGE_REMOVED",
			Details:  fmt.Sprintf("message %s was removed", msg),
		})
	}

	prevSvcs := make(map[string]bool, len(prevBody.Services))
	for _, s := range prevBody.Services {
		prevSvcs[s.ServiceName] = true
	}
	currSvcs := make(map[string]bool, len(currBody.Services))
	for _, s := range currBody.Services {
		currSvcs[s.ServiceName] = true
	}
	for _, svc := range sortedMissingKeys(prevSvcs, currSvcs) {
		items = append(items, ProtoDiffItem{
			Filename: filename,
			Severity: "CRITICAL",
			Change:   "SERVICE_REMOVED",
			Details:  fmt.Sprintf("service %s was removed", svc),
		})
	}

	prevEnums := make(map[string]bool, len(prevBody.Enums))
	for _, e := range prevBody.Enums {
		prevEnums[e.EnumName] = true
	}
	currEnums := make(map[string]bool, len(currBody.Enums))
	for _, e := range currBody.Enums {
		currEnums[e.EnumName] = true
	}
	for _, enumName := range sortedMissingKeys(prevEnums, currEnums) {
		items = append(items, ProtoDiffItem{
			Filename: filename,
			Severity: "CRITICAL",
			Change:   "ENUM_REMOVED",
			Details:  fmt.Sprintf("enum %s was removed", enumName),
		})
	}

	prevEnumVals := collectEnumValuesByNumber(prevBody.Enums)
	currEnumVals := collectEnumValuesByNumber(currBody.Enums)
	for _, evKey := range sortedMapKeys(prevEnumVals) {
		if _, exists := currEnumVals[evKey]; !exists {
			items = append(items, ProtoDiffItem{
				Filename: filename,
				Severity: "CRITICAL",
				Change:   "ENUM_VALUE_REMOVED",
				Details:  fmt.Sprintf("enum value %s (%s) was removed", evKey, prevEnumVals[evKey]),
			})
		}
	}

	prevFields := collectFieldsByTagNumber(prevBody.Messages, "")
	currFields := collectFieldsByTagNumber(currBody.Messages, "")
	for _, fk := range sortedMapKeys(prevFields) {
		pf := prevFields[fk]
		cf, exists := currFields[fk]
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
		} else if pf.Name != cf.Name {
			items = append(items, ProtoDiffItem{
				Filename: filename,
				Severity: "WARNING",
				Change:   "FIELD_RENAMED",
				Details:  fmt.Sprintf("field %s renamed from %s to %s", fk, pf.Name, cf.Name),
			})
		}
	}

	if countRequiredFields(currBody.Messages) > countRequiredFields(prevBody.Messages) {
		items = append(items, ProtoDiffItem{
			Filename: filename,
			Severity: "WARNING",
			Change:   "REQUIRED_FIELD_ADDED",
			Details:  "new required field added",
		})
	}

	prevRPCs := collectRPCDetails(prevBody.Services)
	currRPCs := collectRPCDetails(currBody.Services)
	for _, rk := range sortedMapKeys(prevRPCs) {
		pr := prevRPCs[rk]
		cr, exists := currRPCs[rk]
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
	Name string
	Type string
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

func collectEnumValuesByNumber(enums []*unordered.Enum) map[string]string {
	res := make(map[string]string)
	for _, e := range enums {
		if e.EnumBody == nil {
			continue
		}
		for _, f := range e.EnumBody.EnumFields {
			if n, err := strconv.ParseInt(f.Number, 10, 32); err == nil {
				res[e.EnumName+"#"+strconv.FormatInt(n, 10)] = f.Ident
			}
		}
	}
	return res
}

func collectFieldsByTagNumber(messages []*unordered.Message, prefix string) map[string]protoFieldInfo {
	res := make(map[string]protoFieldInfo)
	for _, m := range messages {
		msgName := m.MessageName
		if prefix != "" {
			msgName = prefix + "." + m.MessageName
		}
		if m.MessageBody == nil {
			continue
		}
		for _, f := range m.MessageBody.Fields {
			if n, err := strconv.ParseInt(f.FieldNumber, 10, 32); err == nil {
				res[msgName+"#"+strconv.FormatInt(n, 10)] = protoFieldInfo{Name: f.FieldName, Type: f.Type}
			}
		}
		for _, mf := range m.MessageBody.Maps {
			if n, err := strconv.ParseInt(mf.FieldNumber, 10, 32); err == nil {
				res[msgName+"#"+strconv.FormatInt(n, 10)] = protoFieldInfo{Name: mf.MapName, Type: "map<" + mf.KeyType + "," + mf.Type + ">"}
			}
		}
		for _, oo := range m.MessageBody.Oneofs {
			for _, oof := range oo.OneofFields {
				if n, err := strconv.ParseInt(oof.FieldNumber, 10, 32); err == nil {
					res[msgName+"#"+strconv.FormatInt(n, 10)] = protoFieldInfo{Name: oof.FieldName, Type: oof.Type}
				}
			}
		}
		for k, v := range collectFieldsByTagNumber(m.MessageBody.Messages, msgName) {
			res[k] = v
		}
	}
	return res
}

func countRequiredFields(messages []*unordered.Message) int {
	count := 0
	for _, m := range messages {
		if m.MessageBody == nil {
			continue
		}
		for _, f := range m.MessageBody.Fields {
			if f.IsRequired {
				count++
			}
		}
		count += countRequiredFields(m.MessageBody.Messages)
	}
	return count
}

func collectRPCDetails(services []*unordered.Service) map[string]protoRPCInfo {
	res := make(map[string]protoRPCInfo)
	for _, s := range services {
		if s.ServiceBody == nil {
			continue
		}
		for _, r := range s.ServiceBody.RPCs {
			var info protoRPCInfo
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

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
