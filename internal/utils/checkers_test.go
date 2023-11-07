package utils

import (
	"testing"

	v1 "github.com/pbufio/pbuf-registry/gen/v1"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateProtoFiles(tt.args.protoFiles); (err != nil) != tt.wantErr {
				t.Errorf("ValidateProtoFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
