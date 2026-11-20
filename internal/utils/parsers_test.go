package utils

import (
	"reflect"
	"testing"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
	"github.com/yoheimuta/go-protoparser/v4/parser"
)

const (
	testProtoContent = `syntax = "proto3"; package hello; message Hello {}`
)

func TestParseProtoFilesContents(t *testing.T) {
	type args struct {
		protoFiles []*v1.ProtoFile
	}
	tests := []struct {
		name    string
		args    args
		want    []*model.ParsedProtoFile
		wantErr bool
	}{
		{
			name: "Parse proto files",
			args: args{
				protoFiles: []*v1.ProtoFile{
					{
						Filename: "hello/test.proto",
						Content:  testProtoContent,
					},
				},
			},
			want: []*model.ParsedProtoFile{
				{
					Filename: "hello/test.proto",
					Proto: &unordered.Proto{
						Syntax: &parser.Syntax{
							ProtobufVersion: "proto3",
						},
						ProtoBody: &unordered.ProtoBody{
							Packages: []*parser.Package{
								{
									Name: "hello",
								},
							},
							Messages: []*unordered.Message{
								{
									MessageName: "Hello",
								},
							},
						},
					},
					ProtoJson: `{"Syntax":{"ProtobufVersion":"proto3","ProtobufVersionQuote":"\"proto3\"","Comments":null,"InlineComment":null,"Meta":{"Pos":{"Filename":"","Offset":0,"Line":1,"Column":1},"LastPos":{"Filename":"","Offset":17,"Line":1,"Column":18}}},"ProtoBody":{"Imports":null,"Packages":[{"Name":"hello","Comments":null,"InlineComment":null,"Meta":{"Pos":{"Filename":"","Offset":19,"Line":1,"Column":20},"LastPos":{"Filename":"","Offset":32,"Line":1,"Column":33}}}],"Options":null,"Messages":[{"MessageName":"Hello","MessageBody":{"Fields":null,"Enums":null,"Messages":null,"Options":null,"Oneofs":null,"Maps":null,"Groups":null,"Reserves":null,"Extends":null,"EmptyStatements":null,"Extensions":null},"Comments":null,"InlineComment":null,"InlineCommentBehindLeftCurly":null,"Meta":{"Pos":{"Filename":"","Offset":34,"Line":1,"Column":35},"LastPos":{"Filename":"","Offset":49,"Line":1,"Column":50}}}],"Extends":null,"Enums":null,"Services":null,"EmptyStatements":null}}`,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProtoFilesContents(tt.args.protoFiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProtoFilesContents() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for i, g := range got {
				if !reflect.DeepEqual(g.Filename, tt.want[i].Filename) {
					t.Errorf("ParseProtoFilesContents() got = %v, want %v", g.Filename, tt.want[i].Filename)
				}
				if !reflect.DeepEqual(g.Proto.Syntax.ProtobufVersion, tt.want[i].Proto.Syntax.ProtobufVersion) {
					t.Errorf("ParseProtoFilesContents() got = %v, want %v", g.Proto.Syntax.ProtobufVersion, tt.want[i].Proto.Syntax.ProtobufVersion)
				}
				if !reflect.DeepEqual(g.Proto.ProtoBody.Packages[0].Name, tt.want[i].Proto.ProtoBody.Packages[0].Name) {
					t.Errorf("ParseProtoFilesContents() got = %v, want %v", g.Proto.ProtoBody.Packages[0].Name, tt.want[i].Proto.ProtoBody.Packages[0].Name)
				}
				if !reflect.DeepEqual(g.Proto.ProtoBody.Messages[0].MessageName, tt.want[i].Proto.ProtoBody.Messages[0].MessageName) {
					t.Errorf("ParseProtoFilesContents() got = %v, want %v", g.Proto.ProtoBody.Messages[0].MessageName, tt.want[i].Proto.ProtoBody.Messages[0].MessageName)
				}
				if !reflect.DeepEqual(g.ProtoJson, tt.want[i].ProtoJson) {
					t.Errorf("ParseProtoFilesContents() got = %v, want %v", g.ProtoJson, tt.want[i].ProtoJson)
				}
			}
		})
	}
}

func mustParseUnordered(t *testing.T, content string) *unordered.Proto {
	t.Helper()
	parsed, err := ParseProtoFilesContents([]*v1.ProtoFile{{Filename: "order.proto", Content: content}})
	if err != nil {
		t.Fatalf("ParseProtoFilesContents() unexpected error: %v", err)
	}
	return parsed[0].Proto
}

func TestDiffParsedProtoFile_NestedEnumsCardinalityAndHexTags(t *testing.T) {
	t.Run("detects nested enum and nested enum value removals", func(t *testing.T) {
		prev := mustParseUnordered(t, `
			syntax = "proto3";
			package shop.v1;
			message Order {
				enum Status {
					STATUS_UNSPECIFIED = 0;
					STATUS_PAID = 0x01;
					STATUS_REFUNDED = 02;
				}
				message LineItem {
					enum Kind {
						KIND_UNSPECIFIED = 0;
					}
				}
			}
		`)
		curr := mustParseUnordered(t, `
			syntax = "proto3";
			package shop.v1;
			message Order {
				enum Status {
					STATUS_UNSPECIFIED = 0;
					STATUS_PAID = 1;
				}
				message LineItem {}
			}
		`)

		items := DiffParsedProtoFile("order.proto", prev, curr)
		want := []ProtoDiffItem{
			{
				Filename: "order.proto",
				Severity: "CRITICAL",
				Change:   "ENUM_REMOVED",
				Details:  "enum Order.LineItem.Kind was removed",
			},
			{
				Filename: "order.proto",
				Severity: "CRITICAL",
				Change:   "ENUM_VALUE_REMOVED",
				Details:  "enum value Order.Status#2 (STATUS_REFUNDED) was removed",
			},
		}
		if !reflect.DeepEqual(items, want) {
			t.Fatalf("DiffParsedProtoFile() = %#v, want %#v", items, want)
		}
	})

	t.Run("detects repeated cardinality changes and hex/octal field tags", func(t *testing.T) {
		prev := mustParseUnordered(t, `
			syntax = "proto3";
			message Order {
				string tags = 1;
				string hex_same = 0x10;
				string hex_removed = 0x20;
			}
		`)
		curr := mustParseUnordered(t, `
			syntax = "proto3";
			message Order {
				repeated string tags = 1;
				string hex_same = 16;
			}
		`)

		items := DiffParsedProtoFile("order.proto", prev, curr)
		want := []ProtoDiffItem{
			{
				Filename: "order.proto",
				Severity: "CRITICAL",
				Change:   "FIELD_CARDINALITY_CHANGED",
				Details:  "field Order#1 (tags) cardinality changed from singular to repeated",
			},
			{
				Filename: "order.proto",
				Severity: "CRITICAL",
				Change:   "FIELD_REMOVED",
				Details:  "field Order#32 (string hex_removed) was removed",
			},
		}
		if !reflect.DeepEqual(items, want) {
			t.Fatalf("DiffParsedProtoFile() = %#v, want %#v", items, want)
		}
	})

	t.Run("detects proto2 required field added, required<->optional cardinality, enum aliases, swapped enum numbers, and suppresses child noise", func(t *testing.T) {
		prev := mustParseUnordered(t, `
			syntax = "proto2";
			enum Code {
				option allow_alias = true;
				CODE_LEGACY = -0x01;
				CODE_UNSPECIFIED = 0;
				CODE_PAID = 1;
				CODE_SETTLED = 1;
				CODE_REFUNDED = 2;
			}
			message Legacy {
				required string old_req = 1;
				required string downgraded = 3;
			}
			message DroppedMsg {
				message Nested {
					optional string id = 1;
				}
				enum InnerEnum {
					INNER_ZERO = 0;
				}
				optional string secret = 1;
			}
			service DroppedSvc {
				rpc Ping(Legacy) returns (Legacy);
			}
		`)
		curr := mustParseUnordered(t, `
			syntax = "proto2";
			enum Code {
				CODE_LEGACY = -1;
				CODE_UNSPECIFIED = 0;
				CODE_REFUNDED = 1;
				CODE_PAID = 2;
			}
			message Legacy {
				required string new_req = 2;
				optional string downgraded = 3;
			}
		`)

		items := DiffParsedProtoFile("legacy.proto", prev, curr)
		want := []ProtoDiffItem{
			{
				Filename: "legacy.proto",
				Severity: "CRITICAL",
				Change:   "MESSAGE_REMOVED",
				Details:  "message DroppedMsg was removed",
			},
			{
				Filename: "legacy.proto",
				Severity: "CRITICAL",
				Change:   "SERVICE_REMOVED",
				Details:  "service DroppedSvc was removed",
			},
			{
				Filename: "legacy.proto",
				Severity: "CRITICAL",
				Change:   "ENUM_VALUE_REMOVED",
				Details:  "enum value Code#1 (CODE_PAID) was removed",
			},
			{
				Filename: "legacy.proto",
				Severity: "CRITICAL",
				Change:   "ENUM_VALUE_REMOVED",
				Details:  "enum value Code#1 (CODE_SETTLED) was removed",
			},
			{
				Filename: "legacy.proto",
				Severity: "CRITICAL",
				Change:   "ENUM_VALUE_REMOVED",
				Details:  "enum value Code#2 (CODE_REFUNDED) was removed",
			},
			{
				Filename: "legacy.proto",
				Severity: "CRITICAL",
				Change:   "FIELD_REMOVED",
				Details:  "field Legacy#1 (string old_req) was removed",
			},
			{
				Filename: "legacy.proto",
				Severity: "CRITICAL",
				Change:   "FIELD_CARDINALITY_CHANGED",
				Details:  "field Legacy#3 (downgraded) cardinality changed from required to singular",
			},
			{
				Filename: "legacy.proto",
				Severity: "WARNING",
				Change:   "REQUIRED_FIELD_ADDED",
				Details:  "new required field added",
			},
		}
		if !reflect.DeepEqual(items, want) {
			t.Fatalf("DiffParsedProtoFile() = %#v, want %#v", items, want)
		}
	})
}
