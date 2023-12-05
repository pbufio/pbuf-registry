package utils

import (
	"reflect"
	"testing"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
)

func TestRetrieveMeta(t *testing.T) {
	parsedProtoFiles, err := ParseProtoFilesContents([]*v1.ProtoFile{
		{
			Filename: "test.proto",
			Content:  complexProtoFile,
		},
	})
	if err != nil {
		t.Errorf("ParseProtoFilesContents() error = %v", err)
		return
	}

	type args struct {
		files []*model.ParsedProtoFile
	}
	tests := []struct {
		name    string
		args    args
		want    *model.TagMeta
		wantErr bool
	}{
		{
			name: "complex proto file",
			args: args{
				files: parsedProtoFiles,
			},
			want: &model.TagMeta{
				Packages:    []string{"pbufregistry.v1"},
				Imports:     []string{"google/api/annotations.proto", "pbuf-registry/v1/entities.proto"},
				RefPackages: []string{"google.api", "test1", "test2", "validate"},
				FilesMeta: []*model.FileMeta{
					{
						Filename:    "test.proto",
						Packages:    []string{"pbufregistry.v1"},
						Imports:     []string{"google/api/annotations.proto", "pbuf-registry/v1/entities.proto"},
						RefPackages: []string{"google.api", "test1", "test2", "validate"},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RetrieveMeta(tt.args.files)
			if (err != nil) != tt.wantErr {
				t.Errorf("RetrieveMeta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(&got, &tt.want) {
				t.Errorf("RetrieveMeta() got = %v, want %v", got, tt.want)
			}
		})
	}
}

const complexProtoFile = `
syntax = "proto3";

package pbufregistry.v1;

import "google/api/annotations.proto";
import "pbuf-registry/v1/entities.proto";

option go_package = "pbufregistry/api/v1;v1";

// Registry service definition
service Registry {
  // List all registered modules
  rpc ListModules(test2.ListModulesRequest) returns (ListModulesResponse) {
    option (google.api.http) = {
      get: "/v1/modules"
    };
  }
}

// ListModulesResponse is the response message for ListModules.
message ListModulesResponse {
  // The modules requested.
  repeated test1.Module modules = 1;
  string next_page_token = 2 [json_name = "next_page_token", (validate.rules).string.uuid = true];
}`
