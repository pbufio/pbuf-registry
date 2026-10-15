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
