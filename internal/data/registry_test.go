package data

import (
	"context"
	"testing"
)

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
