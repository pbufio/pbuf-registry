package utils

import (
	"testing"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
)

func TestValidateProtoFiles(t *testing.T) {
	type args struct {
		protoFiles []*v1.ProtoFile
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Validate proto files",
			args: args{
				protoFiles: []*v1.ProtoFile{
					{
						Filename: "hello/test.proto",
						Content:  "syntax = \"proto3\";",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty filename",
			args: args{
				protoFiles: []*v1.ProtoFile{
					{
						Filename: "",
						Content:  "syntax = \"proto3\";",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty content",
			args: args{
				protoFiles: []*v1.ProtoFile{
					{
						Filename: "hello.proto",
						Content:  "",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "unparsable content",
			args: args{
				protoFiles: []*v1.ProtoFile{
					{
						Filename: "hello.proto",
						Content:  "syntax = \"proto3\"",
					},
				},
			},
			wantErr: true,
		},
		{
			name:    "nil entry rejected",
			args:    args{protoFiles: []*v1.ProtoFile{nil}},
			wantErr: true,
		},
		{
			name: "path traversal rejected",
			args: args{
				protoFiles: []*v1.ProtoFile{{Filename: "../evil.proto", Content: "syntax = \"proto3\";"}},
			},
			wantErr: true,
		},
		{
			name: "leading slash rejected",
			args: args{
				protoFiles: []*v1.ProtoFile{{Filename: "/abs/test.proto", Content: "syntax = \"proto3\";"}},
			},
			wantErr: true,
		},
		{
			name: "backslash rejected",
			args: args{
				protoFiles: []*v1.ProtoFile{{Filename: `api\v1\test.proto`, Content: "syntax = \"proto3\";"}},
			},
			wantErr: true,
		},
		{
			name: "non-clean path rejected",
			args: args{
				protoFiles: []*v1.ProtoFile{{Filename: "api//test.proto", Content: "syntax = \"proto3\";"}},
			},
			wantErr: true,
		},
		{
			name: "non-proto extension rejected",
			args: args{
				protoFiles: []*v1.ProtoFile{{Filename: "README.md", Content: "syntax = \"proto3\";"}},
			},
			wantErr: true,
		},
		{
			name: "bare .proto basename rejected",
			args: args{
				protoFiles: []*v1.ProtoFile{{Filename: ".proto", Content: "syntax = \"proto3\";"}},
			},
			wantErr: true,
		},
		{
			name:    "empty protoFiles slice rejected",
			args:    args{protoFiles: nil},
			wantErr: true,
		},
		{
			name: "windows drive-relative traversal rejected",
			args: args{
				protoFiles: []*v1.ProtoFile{{Filename: "C:../evil.proto", Content: "syntax = \"proto3\";"}},
			},
			wantErr: true,
		},
		{
			name: "control or NUL byte rejected",
			args: args{
				protoFiles: []*v1.ProtoFile{{Filename: "api/\x00evil.proto", Content: "syntax = \"proto3\";"}},
			},
			wantErr: true,
		},
		{
			name: "duplicate filenames rejected",
			args: args{
				protoFiles: []*v1.ProtoFile{
					{Filename: "api/v1/a.proto", Content: "syntax = \"proto3\";"},
					{Filename: "api/v1/a.proto", Content: "syntax = \"proto3\";"},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateProtoFiles(tt.args.protoFiles); (err != nil) != tt.wantErr {
				t.Errorf("ValidateProtoFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
