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
		{
			name: "single line syntax and message without go_package",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: "api/v1",
				content:    `syntax = "proto3"; message Foo { string id = 1; }`,
			},
			want:    "syntax = \"proto3\";\noption go_package = \"github.com/pbufio/pbuf-registry/api/v1;v1\";\nmessage Foo { string id = 1; }",
			wantErr: false,
		},
		{
			name: "single line syntax only without trailing newline",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: "api/v1",
				content:    `syntax = "proto3";`,
			},
			want:    "syntax = \"proto3\";\noption go_package = \"github.com/pbufio/pbuf-registry/api/v1;v1\";",
			wantErr: false,
		},
		{
			name: "crlf line endings and trailing spaces after syntax",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: "api/v1",
				content:    "syntax = \"proto3\";   \r\nmessage Foo {}\r\n",
			},
			want:    "syntax = \"proto3\";\r\noption go_package = \"github.com/pbufio/pbuf-registry/api/v1;v1\";\r\nmessage Foo {}\r\n",
			wantErr: false,
		},
		{
			name: "proto2 without syntax statement",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: "api/v1",
				content:    "package legacy.v1;\nmessage Foo {}\n",
			},
			want:    "option go_package = \"github.com/pbufio/pbuf-registry/api/v1;v1\";\npackage legacy.v1;\nmessage Foo {}\n",
			wantErr: false,
		},
		{
			name: "protobuf edition 2023 inserts go_package after edition header",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: "api/v1",
				content:    "edition = \"2023\";\npackage acme.v1;\nmessage Foo {}\n",
			},
			want:    "edition = \"2023\";\noption go_package = \"github.com/pbufio/pbuf-registry/api/v1;v1\";\npackage acme.v1;\nmessage Foo {}\n",
			wantErr: false,
		},
		{
			name: "inline comment after syntax statement stays on syntax line",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: "api/v1",
				content:    "syntax = \"proto3\"; // inline comment\nmessage Foo {}\n",
			},
			want:    "syntax = \"proto3\"; // inline comment\noption go_package = \"github.com/pbufio/pbuf-registry/api/v1;v1\";\nmessage Foo {}\n",
			wantErr: false,
		},
		{
			name: "rejects quote breakout injection in outputPath",
			fields: fields{
				goModule: "github.com/pbufio/pbuf-registry",
			},
			args: args{
				outputPath: `gen"; option evil = "1`,
				content:    "syntax = \"proto3\";\nmessage Foo {}\n",
			},
			wantErr: true,
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
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("Patch() got = %v, want %v", got, tt.want)
				return
			}
			// Idempotency: patching an already-patched file must produce identical output
			second, err := p.Patch(tt.args.outputPath, got)
			if err != nil {
				t.Errorf("second Patch() error = %v", err)
				return
			}
			if second != tt.want {
				t.Errorf("second Patch() got = %v, want %v", second, tt.want)
			}
		})
	}
}

func TestNormalizeAndWriteVendoredProto(t *testing.T) {
	sub, err := NormalizeModuleSubpath("./.")
	if err != nil || sub != "" {
		t.Fatalf("NormalizeModuleSubpath(\"./.\") = %q, %v; want \"\", nil", sub, err)
	}
	sub, err = NormalizeModuleSubpath("api/..")
	if err != nil || sub != "" {
		t.Fatalf("NormalizeModuleSubpath(\"api/..\") = %q, %v; want \"\", nil", sub, err)
	}
	t.Chdir(t.TempDir())
	if err := WriteVendoredProto("vendor/a.proto", "vendor", "../outside", "syntax = \"proto3\";\n", nil); err == nil {
		t.Fatal("expected WriteVendoredProto to reject traversal genOutFolder")
	}
}
