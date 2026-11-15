package patcher

import "testing"

const (
	noPackageProtoFile = `
syntax = "proto3";
package pbufregistry.v1;

// Module is a module registered in the registry.
message Module {
}
`
	noPackageProtoFilePatched = `
syntax = "proto3";
option go_package = "github.com/pbufio/pbuf-registry/api/v1;v1";
package pbufregistry.v1;

// Module is a module registered in the registry.
message Module {
}
`

	withPackageProtoFile = `
syntax = "proto3";
package pbufregistry.v1;

option go_package = "pbufregistry/api/v2;v2";

// Module is a module registered in the registry.
message Module {
}
`

	withPackageProtoFilePatched = `
syntax = "proto3";
package pbufregistry.v1;

option go_package = "github.com/pbufio/pbuf-registry/api/v1;v1";

// Module is a module registered in the registry.
message Module {
}
`
)

func TestGoPackagePatcher_Patch(t *testing.T) {
	type fields struct {
		goModule string
	}
	type args struct {
		outputPath string
		content    string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "no package",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: "api/v1",
				content:    noPackageProtoFile,
			},
			want:    noPackageProtoFilePatched,
			wantErr: false,
		},
		{
			name: "with package",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: "api/v1",
				content:    withPackageProtoFile,
			},
			want:    withPackageProtoFilePatched,
			wantErr: false,
		},
		{
			name: "single line and multiline option",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: "api/v1",
				content:    `syntax = "proto3"; package pbufregistry.v1; option go_package = "old/pkg;v1"; message Module {}`,
			},
			want:    `syntax = "proto3"; package pbufregistry.v1; option go_package = "github.com/pbufio/pbuf-registry/api/v1;v1"; message Module {}`,
			wantErr: false,
		},
		{
			name: "windows backslash output path",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: `api\v1`,
				content:    withPackageProtoFile,
			},
			want:    withPackageProtoFilePatched,
			wantErr: false,
		},
		{
			name: "hyphenated directory name sanitized to valid Go identifier",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: "gen/api/pbuf-registry",
				content:    "syntax = \"proto3\";\nmessage Module {}",
			},
			want:    "syntax = \"proto3\";\noption go_package = \"github.com/pbufio/pbuf-registry/gen/api/pbuf-registry;pbuf_registry\";\nmessage Module {}",
			wantErr: false,
		},
		{
			name: "leading digit and keyword directory names sanitized",
			fields: fields{
				goModule: "github.com/acme/app",
			},
			args: args{
				outputPath: "gen/type",
				content:    "syntax = \"proto3\";\nmessage Kind {}",
			},
			want:    "syntax = \"proto3\";\noption go_package = \"github.com/acme/app/gen/type;type_pb\";\nmessage Kind {}",
			wantErr: false,
		},
		{
			name: "leading digit directory name prefixed",
			fields: fields{
				goModule: "github.com/acme/app",
			},
			args: args{
				outputPath: "gen/2fa.v1",
				content:    "syntax = \"proto3\";\nmessage Token {}",
			},
			want:    "syntax = \"proto3\";\noption go_package = \"github.com/acme/app/gen/2fa.v1;pb_2fa_v1\";\nmessage Token {}",
			wantErr: false,
		},
		{
			name: "root outputPath uses goModule without trailing dot",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: ".",
				content:    "syntax = \"proto3\";\nmessage Root {}",
			},
			want:    "syntax = \"proto3\";\noption go_package = \"github.com/pbufio/pbuf-registry;pbuf_registry\";\nmessage Root {}",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewGoPackagePatcher(tt.fields.goModule)

			got, err := p.Patch(tt.args.outputPath, tt.args.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Patch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Patch() got = %v, want %v", got, tt.want)
			}
		})
	}
}
