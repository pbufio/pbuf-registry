package data

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/pbufio/pbuf-registry/internal/utils"
)

const (
	fakeUUID = "6c3a8661-0fea-4742-be2b-bdf757a74591"
)

var protofiles = []*v1.ProtoFile{
	{
		Filename: "hello/test.proto",
		Content:  "syntax = \"proto3\"; package hello; message Hello {}",
	},
}

func Test_registryRepository_RegisterModule(t *testing.T) {
	type args struct {
		moduleName string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "Register module",
			args:    args{moduleName: "pbuf.io/pbuf-registry"},
			wantErr: false,
		},
		{
			name:    "Register module 2",
			args:    args{moduleName: "pbuf.io/pbuf-registry-2"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.registryRepository

			if err := r.RegisterModule(context.Background(), tt.args.moduleName); (err != nil) != tt.wantErr {
				t.Errorf("RegisterModule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_registryRepository_GetModule(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.Module
		wantErr bool
	}{
		{
			name: "Get module",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
			},
			want: &v1.Module{
				Id:   fakeUUID,
				Name: "pbuf.io/pbuf-registry",
			},
			wantErr: false,
		},
		{
			name: "Get module not found",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry-not-found",
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.registryRepository

			got, err := r.GetModule(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetModule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != nil {
				got.Id = fakeUUID
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetModule() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_registryRepository_ListModules(t *testing.T) {
	type args struct {
		ctx      context.Context
		pageSize int
		token    string
	}
	tests := []struct {
		name      string
		args      args
		want      []*v1.Module
		wantToken string
		wantErr   bool
	}{
		{
			name: "List modules",
			args: args{
				ctx:      context.Background(),
				pageSize: 10,
				token:    "",
			},
			want: []*v1.Module{
				{
					Id:   fakeUUID,
					Name: "pbuf.io/pbuf-registry",
				},
				{
					Id:   fakeUUID,
					Name: "pbuf.io/pbuf-registry-2",
				},
			},
			wantToken: "",
			wantErr:   false,
		},
		{
			name: "List modules with page size = 1",
			args: args{
				ctx:      context.Background(),
				pageSize: 1,
				token:    "",
			},
			want: []*v1.Module{
				{
					Id:   fakeUUID,
					Name: "pbuf.io/pbuf-registry",
				},
			},
			wantToken: "cGJ1Zi5pby9wYnVmLXJlZ2lzdHJ5LTI=",
			wantErr:   false,
		},
		{
			name: "List modules with page size = 1 and token",
			args: args{
				ctx:      context.Background(),
				pageSize: 1,
				token:    "cGJ1Zi5pby9wYnVmLXJlZ2lzdHJ5LTI=",
			},
			want: []*v1.Module{
				{
					Id:   fakeUUID,
					Name: "pbuf.io/pbuf-registry-2",
				},
			},
			wantToken: "",
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.registryRepository

			got, token, err := r.ListModules(tt.args.ctx, tt.args.pageSize, tt.args.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListModules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for i := range got {
				got[i].Id = fakeUUID
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListModules() modules got = %v, want %v", got, tt.want)
			}
			if token != tt.wantToken {
				t.Errorf("ListModules() token = %v, want %v", token, tt.wantToken)
			}
		})
	}
}

func Test_registryRepository_PushModule(t *testing.T) {
	type args struct {
		ctx        context.Context
		name       string
		tag        string
		protofiles []*v1.ProtoFile
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.Module
		wantErr bool
	}{
		{
			name: "Push module",
			args: args{
				ctx:        context.Background(),
				name:       "pbuf.io/pbuf-registry",
				tag:        "v0.0.0",
				protofiles: protofiles,
			},
			want: &v1.Module{
				Id:   fakeUUID,
				Name: "pbuf.io/pbuf-registry",
				Tags: []string{"v0.0.0"},
			},
			wantErr: false,
		},
		{
			name: "Push module with existing tag",
			args: args{
				ctx:        context.Background(),
				name:       "pbuf.io/pbuf-registry",
				tag:        "v0.0.0",
				protofiles: protofiles,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Push module",
			args: args{
				ctx:        context.Background(),
				name:       "pbuf.io/pbuf-registry-2",
				tag:        "v0.0.0",
				protofiles: protofiles,
			},
			want: &v1.Module{
				Id:   fakeUUID,
				Name: "pbuf.io/pbuf-registry-2",
				Tags: []string{"v0.0.0"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.registryRepository

			got, err := r.PushModule(tt.args.ctx, tt.args.name, tt.args.tag, tt.args.protofiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("PushModule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != nil {
				got.Id = fakeUUID
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PushModule() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_registryRepository_PushDraftModule(t *testing.T) {
	type args struct {
		ctx          context.Context
		name         string
		tag          string
		protofiles   []*v1.ProtoFile
		dependencies []*v1.Dependency
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.Module
		wantErr bool
	}{
		{
			name: "Push draft module",
			args: args{
				ctx:        context.Background(),
				name:       "pbuf.io/pbuf-registry",
				tag:        "v0.0.0-rc.1",
				protofiles: protofiles,
			},
			want: &v1.Module{
				Id:        fakeUUID,
				Name:      "pbuf.io/pbuf-registry",
				Tags:      []string{"v0.0.0"},
				DraftTags: []string{"v0.0.0-rc.1"},
			},
			wantErr: false,
		},
		{
			name: "Push draft module with existing tag",
			args: args{
				ctx:        context.Background(),
				name:       "pbuf.io/pbuf-registry",
				tag:        "v0.0.0-rc.1",
				protofiles: protofiles,
			},
			want: &v1.Module{
				Id:        fakeUUID,
				Name:      "pbuf.io/pbuf-registry",
				Tags:      []string{"v0.0.0"},
				DraftTags: []string{"v0.0.0-rc.1"},
			},
			wantErr: false,
		},
		{
			name: "Push draft module",
			args: args{
				ctx:        context.Background(),
				name:       "pbuf.io/pbuf-registry-2",
				tag:        "v0.0.0-rc.1",
				protofiles: protofiles,
			},
			want: &v1.Module{
				Id:        fakeUUID,
				Name:      "pbuf.io/pbuf-registry-2",
				Tags:      []string{"v0.0.0"},
				DraftTags: []string{"v0.0.0-rc.1"},
			},
			wantErr: false,
		},
		{
			name: "Push draft module with dependencies",
			args: args{
				ctx:        context.Background(),
				name:       "pbuf.io/pbuf-registry-2",
				tag:        "v0.0.0-rc.1",
				protofiles: protofiles,
				dependencies: []*v1.Dependency{
					{
						Name: "pbuf.io/pbuf-registry",
						Tag:  "v0.0.0",
					},
				},
			},
			want: &v1.Module{
				Id:        fakeUUID,
				Name:      "pbuf.io/pbuf-registry-2",
				Tags:      []string{"v0.0.0"},
				DraftTags: []string{"v0.0.0-rc.1"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.registryRepository

			got, err := r.PushDraftModule(tt.args.ctx, tt.args.name, tt.args.tag, tt.args.protofiles, tt.args.dependencies)
			if (err != nil) != tt.wantErr {
				t.Errorf("PushDraftModule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != nil {
				got.Id = fakeUUID
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PushDraftModule() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_registryRepository_PullModule(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
		tag  string
	}
	tests := []struct {
		name       string
		args       args
		want       []*v1.ProtoFile
		wantModule *v1.Module
		wantErr    bool
	}{
		{
			name: "Pull module",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
				tag:  "v0.0.0",
			},
			wantModule: &v1.Module{
				Id:        fakeUUID,
				Name:      "pbuf.io/pbuf-registry",
				Tags:      []string{"v0.0.0"},
				DraftTags: []string{"v0.0.0-rc.1"},
			},
			want:    protofiles,
			wantErr: false,
		},
		{
			name: "Pull module not found",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry-not-found",
				tag:  "v0.0.0",
			},
			wantModule: nil,
			want:       nil,
			wantErr:    true,
		},
		{
			name: "Pull module - tag not found",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
				tag:  "v0.1.0",
			},
			wantModule: nil,
			want:       nil,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.registryRepository

			module, protoFiles, err := r.PullModule(tt.args.ctx, tt.args.name, tt.args.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("PullModule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if module != nil {
				module.Id = fakeUUID
			}

			if !reflect.DeepEqual(module, tt.wantModule) {
				t.Errorf("PullModule() got = %v, want %v", module, tt.want)
			}
			if !reflect.DeepEqual(protoFiles, tt.want) {
				t.Errorf("PullModule() got1 = %v, want %v", protoFiles, tt.want)
			}
		})
	}
}

func Test_registryRepository_PullDraftModule(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
		tag  string
	}
	tests := []struct {
		name       string
		args       args
		want       []*v1.ProtoFile
		wantModule *v1.Module
		wantErr    bool
	}{
		{
			name: "Pull draft module",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
				tag:  "v0.0.0-rc.1",
			},
			wantModule: &v1.Module{
				Id:        fakeUUID,
				Name:      "pbuf.io/pbuf-registry",
				Tags:      []string{"v0.0.0"},
				DraftTags: []string{"v0.0.0-rc.1"},
			},
			want:    protofiles,
			wantErr: false,
		},
		{
			name: "Pull draft module not found",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry-not-found",
				tag:  "v0.0.0-rc.1",
			},
			wantModule: nil,
			want:       nil,
			wantErr:    true,
		},
		{
			name: "Pull draft module - tag not found",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
				tag:  "v0.1.0",
			},
			wantModule: nil,
			want:       nil,
			wantErr:    true,
		},
		{
			name: "Pull draft module - tag not found",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
				tag:  "v0.1.0",
			},
			wantModule: nil,
			want:       nil,
			wantErr:    true,
		},
		{
			name: "Pull draft module - tag not found",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
				tag:  "v0.1.0",
			},
			wantModule: nil,
			want:       nil,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.registryRepository

			module, protoFiles, err := r.PullDraftModule(tt.args.ctx, tt.args.name, tt.args.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("PullDraftModule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if module != nil {
				module.Id = fakeUUID
			}

			if !reflect.DeepEqual(module, tt.wantModule) {
				t.Errorf("PullDraftModule() got = %v, want %v", module, tt.want)
			}
			if !reflect.DeepEqual(protoFiles, tt.want) {
				t.Errorf("PullDraftModule() got1 = %v, want %v", protoFiles, tt.want)
			}
		})
	}
}

func Test_registryRepository_AddModuleDependencies(t *testing.T) {
	type args struct {
		ctx          context.Context
		name         string
		tag          string
		dependencies []*v1.Dependency
	}
	type want struct {
		dependencies []*v1.Dependency
	}
	tests := []struct {
		name    string
		args    args
		want    want
		wantErr bool
	}{
		{
			name: "Add module dependencies",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
				tag:  "v0.0.0",
				dependencies: []*v1.Dependency{
					{
						Name: "pbuf.io/pbuf-registry-2",
						Tag:  "v0.0.0",
					},
				},
			},
			want: want{
				dependencies: []*v1.Dependency{
					{
						Name: "pbuf.io/pbuf-registry-2",
						Tag:  "v0.0.0",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.registryRepository

			if err := r.AddModuleDependencies(tt.args.ctx, tt.args.name, tt.args.tag, tt.args.dependencies); (err != nil) != tt.wantErr {
				t.Errorf("AddModuleDependencies() error = %v, wantErr %v", err, tt.wantErr)
			}

			dependencies, err := r.GetModuleDependencies(tt.args.ctx, tt.args.name, tt.args.tag)
			if err != nil {
				t.Errorf("GetModuleDependencies() error = %v", err)
			}

			if !reflect.DeepEqual(dependencies, tt.want.dependencies) {
				t.Errorf("GetModuleDependencies() got = %v, want %v", dependencies, tt.args.dependencies)
			}
		})
	}
}

func Test_registryRepository_GetModuleDependencies(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
		tag  string
	}
	tests := []struct {
		name    string
		args    args
		want    []*v1.Dependency
		wantErr bool
	}{
		{
			name: "Get module dependencies",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
			},
			want: []*v1.Dependency{
				{
					Name: "pbuf.io/pbuf-registry-2",
					Tag:  "v0.0.0",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.registryRepository

			got, err := r.GetModuleDependencies(tt.args.ctx, tt.args.name, tt.args.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetModuleDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetModuleDependencies() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_registryRepository_DeleteModuleTag(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
		tag  string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Delete module tag",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
				tag:  "v0.0.0",
			},
			wantErr: false,
		},
		{
			name: "Delete module tag not found",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
				tag:  "v0.0.0",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.registryRepository

			if err := r.DeleteModuleTag(tt.args.ctx, tt.args.name, tt.args.tag); (err != nil) != tt.wantErr {
				t.Errorf("DeleteModuleTag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_registryRepository_DeleteModule(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Delete module",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry",
			},
			wantErr: false,
		},
		{
			name: "Delete module 2",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry-2",
			},
			wantErr: false,
		},
		{
			name: "Delete module not found",
			args: args{
				ctx:  context.Background(),
				name: "pbuf.io/pbuf-registry-not-found",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := suite.registryRepository

			if err := r.DeleteModule(tt.args.ctx, tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("DeleteModule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_metadataRepo_GetUnprocessedTagIds(t *testing.T) {
	type args struct {
		moduleName string
		tagIds     []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "get unprocessed tag ids",
			args: args{
				moduleName: "test-module",
				tagIds: []string{
					"test-tag-1",
					"test-tag-2",
					"test-tag-3",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := suite.metadataRepository

			err := suite.registryRepository.RegisterModule(context.Background(), tt.args.moduleName)
			if err != nil {
				t.Errorf("error registering module: %v", err)
				return
			}

			for _, tagId := range tt.args.tagIds {
				_, err := suite.registryRepository.PushModule(
					context.Background(),
					tt.args.moduleName,
					tagId,
					protofiles)
				if err != nil {
					t.Errorf("error pushing module: %v", err)
					return
				}
			}

			got, err := m.GetUnprocessedTagIds(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUnprocessedTagIds() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != len(tt.args.tagIds) {
				t.Errorf("GetUnprocessedTagIds() got = %v, want %v", got, tt.args.tagIds)
			}
		})
	}
}

func Test_metadataRepo_GetProtoFilesForTagId(t *testing.T) {
	var noProtoFiles []*v1.ProtoFile

	type args struct {
		tagId string
	}
	tests := []struct {
		name    string
		args    args
		want    []*v1.ProtoFile
		wantErr bool
	}{
		{
			name: "get proto files for tag id",
			args: args{
				tagId: fakeUUID,
			},
			want:    protofiles,
			wantErr: false,
		},
		{
			name: "get proto files for tag id not found",
			args: args{
				tagId: "not-found",
			},
			want:    noProtoFiles,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := suite.metadataRepository

			tagIds, err := repository.GetUnprocessedTagIds(context.Background())
			if err != nil {
				t.Errorf("error getting unprocessed tag ids: %v", err)
				return
			}

			tagId := tt.args.tagId
			if tt.args.tagId == fakeUUID {
				tagId = tagIds[0]
			}

			got, err := repository.GetProtoFilesForTagId(context.Background(), tagId)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProtoFilesForTagId() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProtoFilesForTagId() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_metadataRepo_SaveParsedProtoFiles(t *testing.T) {
	type args struct {
		tagId string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "save parsed proto files",
			args: args{
				tagId: fakeUUID,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := suite.metadataRepository

			tagIds, err := repository.GetUnprocessedTagIds(context.Background())
			if err != nil {
				t.Errorf("error getting unprocessed tag ids: %v", err)
				return
			}

			tagId := tt.args.tagId
			if tt.args.tagId == fakeUUID {
				tagId = tagIds[0]
			}

			parsedProtoFiles, err := utils.ParseProtoFilesContents(protofiles)
			if err != nil {
				t.Errorf("error parsing proto files: %v", err)
				return
			}

			err = repository.SaveParsedProtoFiles(context.Background(), tagId, parsedProtoFiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveParsedProtoFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_metadataRepo_GetTagMeta(t *testing.T) {
	noImports := []string{}
	noRefPackages := []string{}

	tests := []struct {
		name    string
		want    *model.TagMeta
		wantErr bool
	}{
		{
			name: "get tag meta",
			want: &model.TagMeta{
				Packages:    []string{"hello"},
				Imports:     noImports,
				RefPackages: noRefPackages,
				FilesMeta: []*model.FileMeta{
					{
						Filename:    "hello/test.proto",
						Packages:    []string{"hello"},
						Imports:     noImports,
						RefPackages: noRefPackages,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := suite.metadataRepository

			tagIds, err := repository.GetUnprocessedTagIds(context.Background())
			if err != nil {
				t.Errorf("error getting unprocessed tag ids: %v", err)
				return
			}

			parsedProtoFiles, err := utils.ParseProtoFilesContents(protofiles)
			if err != nil {
				t.Errorf("error parsing proto files: %v", err)
				return
			}

			tagId := tagIds[0]
			err = repository.SaveParsedProtoFiles(context.Background(), tagId, parsedProtoFiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveParsedProtoFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			tagMeta, err := repository.GetTagMetaByTagId(context.Background(), tagId)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTagMetaByTagId() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(&tagMeta, &tt.want) {
				t.Errorf("GetTagMetaByTagId() got = %v, want %v", tagMeta, tt.want)
			}
		})
	}
}

func Test_registryRepository_GetModuleTagId(t *testing.T) {
	ctx := context.Background()
	r := suite.registryRepository

	moduleName := "test-get-tag-id-" + time.Now().Format("20060102150405")
	err := r.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	tagName := "v1.0.0"
	_, err = r.PushModule(ctx, moduleName, tagName, []*v1.ProtoFile{
		{Filename: "test.proto", Content: `syntax = "proto3"; message Test {}`},
	})
	require.NoError(t, err)

	// Test: get existing tag ID
	tagId, err := r.GetModuleTagId(ctx, moduleName, tagName)
	require.NoError(t, err)
	assert.NotEmpty(t, tagId)

	// Test: get non-existing tag ID returns empty
	tagId, err = r.GetModuleTagId(ctx, moduleName, "v999.0.0")
	require.NoError(t, err)
	assert.Empty(t, tagId)

	// Test: get tag for non-existing module returns empty
	tagId, err = r.GetModuleTagId(ctx, "nonexistent-module", tagName)
	require.NoError(t, err)
	assert.Empty(t, tagId)
}

func Test_registryRepository_GetTransitiveDependencies(t *testing.T) {
	ctx := context.Background()
	r := suite.registryRepository

	ts := time.Now().Format("20060102150405")
	moduleA := "test-transitive-a-" + ts
	moduleB := "test-transitive-b-" + ts
	moduleC := "test-transitive-c-" + ts

	// Register all modules
	for _, m := range []string{moduleA, moduleB, moduleC} {
		err := r.RegisterModule(ctx, m)
		require.NoError(t, err)
	}

	// Create module C (leaf dependency, no dependencies)
	_, err := r.PushModule(ctx, moduleC, "v1.0.0", []*v1.ProtoFile{
		{Filename: "c.proto", Content: `syntax = "proto3"; message C {}`},
	})
	require.NoError(t, err)

	// Create module B (depends on C)
	_, err = r.PushModule(ctx, moduleB, "v1.0.0", []*v1.ProtoFile{
		{Filename: "b.proto", Content: `syntax = "proto3"; message B {}`},
	})
	require.NoError(t, err)
	err = r.AddModuleDependencies(ctx, moduleB, "v1.0.0", []*v1.Dependency{
		{Name: moduleC, Tag: "v1.0.0"},
	})
	require.NoError(t, err)

	// Create module A (depends on B)
	_, err = r.PushModule(ctx, moduleA, "v1.0.0", []*v1.ProtoFile{
		{Filename: "a.proto", Content: `syntax = "proto3"; message A {}`},
	})
	require.NoError(t, err)
	err = r.AddModuleDependencies(ctx, moduleA, "v1.0.0", []*v1.Dependency{
		{Name: moduleB, Tag: "v1.0.0"},
	})
	require.NoError(t, err)

	// Test: get transitive dependencies for A should return B (direct) and C (transitive)
	deps, err := r.GetTransitiveDependencies(ctx, moduleA, "v1.0.0")
	require.NoError(t, err)
	require.Len(t, deps, 2)

	// Verify B is direct
	assert.Equal(t, moduleB, deps[0].Name)
	assert.Equal(t, "v1.0.0", deps[0].Tag)
	assert.Equal(t, "direct", deps[0].DependencyType)

	// Verify C is transitive
	assert.Equal(t, moduleC, deps[1].Name)
	assert.Equal(t, "v1.0.0", deps[1].Tag)
	assert.Equal(t, "transitive", deps[1].DependencyType)

	// Test: module with no dependencies returns empty
	deps, err = r.GetTransitiveDependencies(ctx, moduleC, "v1.0.0")
	require.NoError(t, err)
	assert.Empty(t, deps)
}

func Test_registryRepository_DeleteObsoleteDraftTags(t *testing.T) {
	ctx := context.Background()
	r := suite.registryRepository

	moduleName := "test-obsolete-drafts-" + time.Now().Format("20060102150405")
	err := r.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	// Push a draft module
	_, err = r.PushDraftModule(ctx, moduleName, "v0.0.1-rc.1", []*v1.ProtoFile{
		{Filename: "test.proto", Content: `syntax = "proto3"; message Test {}`},
	}, nil)
	require.NoError(t, err)

	// Verify draft tag exists
	module, err := r.GetModule(ctx, moduleName)
	require.NoError(t, err)
	assert.Contains(t, module.DraftTags, "v0.0.1-rc.1")

	// DeleteObsoleteDraftTags should NOT delete recent draft tags (they are < 7 days old)
	err = r.DeleteObsoleteDraftTags(ctx)
	require.NoError(t, err)

	// Verify draft tag still exists (it's recent)
	module, err = r.GetModule(ctx, moduleName)
	require.NoError(t, err)
	assert.Contains(t, module.DraftTags, "v0.0.1-rc.1")

	// Manually age the draft tag to make it obsolete (> 7 days old)
	_, err = suite.pool.Exec(ctx,
		"UPDATE draft_tags SET updated_at = NOW() - INTERVAL '8 days' WHERE module_id = $1 AND tag = $2",
		module.Id, "v0.0.1-rc.1")
	require.NoError(t, err)

	// Now DeleteObsoleteDraftTags should remove it
	err = r.DeleteObsoleteDraftTags(ctx)
	require.NoError(t, err)

	// Verify draft tag has been deleted
	module, err = r.GetModule(ctx, moduleName)
	require.NoError(t, err)
	assert.NotContains(t, module.DraftTags, "v0.0.1-rc.1")
}

func Test_metadataRepo_GetParsedProtoFiles(t *testing.T) {
	ctx := context.Background()

	moduleName := "test-get-parsed-" + time.Now().Format("20060102150405")
	err := suite.registryRepository.RegisterModule(ctx, moduleName)
	require.NoError(t, err)

	testProtofiles := []*v1.ProtoFile{
		{
			Filename: "hello/test.proto",
			Content:  "syntax = \"proto3\"; package hello; message Hello {}",
		},
	}

	_, err = suite.registryRepository.PushModule(ctx, moduleName, "v1.0.0", testProtofiles)
	require.NoError(t, err)

	tagId, err := suite.registryRepository.GetModuleTagId(ctx, moduleName, "v1.0.0")
	require.NoError(t, err)

	// Parse proto files and save them
	parsedProtoFiles, err := utils.ParseProtoFilesContents(testProtofiles)
	require.NoError(t, err)

	err = suite.metadataRepository.SaveParsedProtoFiles(ctx, tagId, parsedProtoFiles)
	require.NoError(t, err)

	// Now test GetParsedProtoFiles
	result, err := suite.metadataRepository.GetParsedProtoFiles(ctx, tagId)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "hello/test.proto", result[0].Filename)
	assert.NotNil(t, result[0].Proto)

	// Test: non-existing tag returns empty
	result, err = suite.metadataRepository.GetParsedProtoFiles(ctx, fakeUUID)
	require.NoError(t, err)
	assert.Empty(t, result)
}
