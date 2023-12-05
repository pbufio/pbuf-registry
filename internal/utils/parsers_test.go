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
