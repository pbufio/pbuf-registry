package utils

import (
	"reflect"
	"testing"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/require"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
	"github.com/yoheimuta/go-protoparser/v4/parser"
)

func TestToMetadata(t *testing.T) {
	parsedProtoFiles, err := ParseProtoFilesContents([]*v1.ProtoFile{
		{
			Filename: "test.proto",
			Content:  complexProtoFile,
		},
	})
	require.NoError(t, err)

	enumAndPackagelessFiles, err := ParseProtoFilesContents([]*v1.ProtoFile{
		{
			Filename: "order/status.proto",
			Content: `syntax = "proto3";
package acme.order.v1;
package acme.order.v1;

enum OrderStatus {
  ORDER_STATUS_UNSPECIFIED = 0;
  ORDER_STATUS_PAID = 1;
  ORDER_STATUS_FLAGGED = 0x10;
}

message Order {
  enum Priority {
    PRIORITY_UNSPECIFIED = 0;
    PRIORITY_HIGH = 2;
  }
  OrderStatus status = 1;
}`,
		},
		{
			Filename: "legacy.proto",
			Content: `syntax = "proto2";
enum LegacyCode {
  LEGACY_UNKNOWN = -1;
  LEGACY_OK = 0;
}`,
		},
	})
	require.NoError(t, err)

	nilBodyAST := &model.ParsedProtoFile{
		Filename: "sparse.proto",
		Proto: &unordered.Proto{
			ProtoBody: &unordered.ProtoBody{
				Packages: []*parser.Package{nil, {Name: "sparse.v1"}},
				Messages: []*unordered.Message{nil, {MessageName: "EmptyBody"}},
				Enums:    []*unordered.Enum{nil, {EnumName: "SparseEnum"}},
				Services: []*unordered.Service{nil, {ServiceName: "SparseService"}},
			},
		},
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
		{
			name: "top-level and nested enums, duplicate package dedup, packageless file, and nil AST bodies",
			args: args{
				files: append([]*model.ParsedProtoFile{nil, {}, nilBodyAST}, enumAndPackagelessFiles...),
			},
			want: []*v1.Package{
				{
					Name: "",
					ProtoFiles: []*v1.ParsedProtoFile{
						{
							Filename: "legacy.proto",
							Enums: []*v1.Enum{
								{
									Name: "LegacyCode",
									Values: []*v1.EnumValue{
										{Name: "LEGACY_UNKNOWN", Tag: -1},
										{Name: "LEGACY_OK", Tag: 0},
									},
								},
							},
						},
					},
				},
				{
					Name: "acme.order.v1",
					ProtoFiles: []*v1.ParsedProtoFile{
						{
							Filename: "order/status.proto",
							Messages: []*v1.Message{
								{
									Name: "Order",
									Fields: []*v1.Field{
										{Name: "status", MessageType: "OrderStatus", Tag: 1},
									},
									NestedEnums: []*v1.Enum{
										{
											Name: "Priority",
											Values: []*v1.EnumValue{
												{Name: "PRIORITY_UNSPECIFIED", Tag: 0},
												{Name: "PRIORITY_HIGH", Tag: 2},
											},
										},
									},
								},
							},
							Enums: []*v1.Enum{
								{
									Name: "OrderStatus",
									Values: []*v1.EnumValue{
										{Name: "ORDER_STATUS_UNSPECIFIED", Tag: 0},
										{Name: "ORDER_STATUS_PAID", Tag: 1},
										{Name: "ORDER_STATUS_FLAGGED", Tag: 16},
									},
								},
							},
						},
					},
				},
				{
					Name: "sparse.v1",
					ProtoFiles: []*v1.ParsedProtoFile{
						{
							Filename: "sparse.proto",
							Messages: []*v1.Message{{Name: "EmptyBody"}},
							Services: []*v1.Service{{Name: "SparseService"}},
							Enums:    []*v1.Enum{{Name: "SparseEnum"}},
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
