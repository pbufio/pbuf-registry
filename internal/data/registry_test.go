package data

import (
	"context"
	"reflect"
	"testing"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
)

const (
	fakeUUID = "6c3a8661-0fea-4742-be2b-bdf757a74591"
)

var protofiles = []*v1.ProtoFile{
	{
		Filename: "hello/test.proto",
		Content:  "syntax = \"proto3\";",
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
