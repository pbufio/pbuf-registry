package bsr

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/patcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBSRModule(t *testing.T) {
	assert.False(t, IsBSRModule(nil))
	assert.False(t, IsBSRModule(&model.Module{Name: "pbufio/pbuf-registry"}))
	assert.True(t, IsBSRModule(&model.Module{Name: "buf.build/bufbuild/protovalidate"}))
	assert.True(t, IsBSRModule(&model.Module{Name: "protovalidate", Repository: "https://buf.build/bufbuild/protovalidate"}))
}

func TestParseBSRModule(t *testing.T) {
	baseURL, owner, repo, ref, err := ParseBSRModule(&model.Module{
		Name: "buf.build/googleapis/googleapis",
		Tag:  "v1.2.3",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://buf.build", baseURL)
	assert.Equal(t, "googleapis", owner)
	assert.Equal(t, "googleapis", repo)
	assert.Equal(t, "v1.2.3", ref)

	// Inline ref syntax and default main
	_, owner, repo, ref, err = ParseBSRModule(&model.Module{
		Name: "buf.build/bufbuild/protovalidate:v0.14.0",
	})
	require.NoError(t, err)
	assert.Equal(t, "bufbuild", owner)
	assert.Equal(t, "protovalidate", repo)
	assert.Equal(t, "v0.14.0", ref)
}

type handlerRoundTripper struct {
	handler http.Handler
}

func (h handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func TestVendorBSRModule(t *testing.T) {
	protoContent := `syntax = "proto3";
package buf.validate;

message ValidateRules {
  string id = 1;
}
`

	var receivedAuthHeader string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/buf.registry.module.v1beta1.DownloadService/Download" {
			http.NotFound(w, r)
			return
		}
		receivedAuthHeader = r.Header.Get("Authorization")

		resp := map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"files": []map[string]interface{}{
						{
							"path":    "buf/validate/validate.proto",
							"content": base64.StdEncoding.EncodeToString([]byte(protoContent)),
						},
						{
							"path":    "README.md",
							"content": base64.StdEncoding.EncodeToString([]byte("# ignored")),
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	t.Setenv("BUF_TOKEN", "test-buf-token-xyz")

	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "third_party")

	mod := &model.Module{
		Name:                 "buf.build/bufbuild/protovalidate",
		Tag:                  "v0.14.0",
		OutputFolder:         outDir,
		GenerateOutputFolder: "gen/protovalidate",
	}

	origClient := bsrHTTPClient
	bsrHTTPClient = &http.Client{Transport: handlerRoundTripper{handler: handler}}
	defer func() { bsrHTTPClient = origClient }()

	err := VendorBSRModule(mod, nil, []patcher.Patcher{
		patcher.NewGoPackagePatcher("github.com/example/myservice"),
	})
	require.NoError(t, err)
	assert.Equal(t, "Bearer test-buf-token-xyz", receivedAuthHeader)

	writtenFile := filepath.Join(outDir, "buf/validate/validate.proto")
	data, err := os.ReadFile(writtenFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), `option go_package = "github.com/example/myservice/gen/protovalidate/buf/validate;validate";`)

	// Non-proto file should not be written
	_, err = os.Stat(filepath.Join(outDir, "README.md"))
	assert.True(t, os.IsNotExist(err))
}
