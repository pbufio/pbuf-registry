package utils

import (
	"reflect"
	"testing"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
)

func TestToMetadata(t *testing.T) {
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
		name string
		args args
		want []*v1.Package
	}{
		{
			name: "complex proto file",
			args: args{
				files: parsedProtoFiles,
			},
			want: []*v1.Package{
				{
					Name: "pbufregistry.v1",
					ProtoFiles: []*v1.ParsedProtoFile{
						{
							Filename: "test.proto",
							Messages: []*v1.Message{
								{
									Name: "ListModulesResponse",
									Fields: []*v1.Field{
										{
											Name:        "modules",
											MessageType: "test1.Module",
											Tag:         1,
											Repeated:    true,
										},
										{
											Name:        "next_page_token",
											MessageType: "string",
											Tag:         2,
										},
									},
								},
							},
							Services: []*v1.Service{
								{
									Name: "Registry",
									Methods: []*v1.Method{
										{
											Name:       "ListModules",
											InputType:  "test2.ListModulesRequest",
											OutputType: "ListModulesResponse",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToMetadata(tt.args.files); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}
