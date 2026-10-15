package bsr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jdx/go-netrc"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/patcher"
)

const (
	defaultBSRBaseURL = "https://buf.build"
	bsrPrefix         = "buf.build/"
	bsrTimeout        = 60 * time.Second
)

type bsrDownloadRequest struct {
	References []bsrResourceRef `json:"references"`
}

type bsrResourceRef struct {
	Name bsrRefName `json:"name"`
}

type bsrRefName struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
	Ref        string `json:"ref,omitempty"`
	Track      string `json:"track,omitempty"`
}

type bsrDownloadResponse struct {
	Contents []bsrModuleContent `json:"contents"`
	// v1beta1 single-module fallback shape
	Manifest *bsrManifest `json:"manifest,omitempty"`
	Files    []bsrFile    `json:"files,omitempty"`
}

type bsrModuleContent struct {
	Files []bsrFile `json:"files"`
}

type bsrManifest struct {
	Paths []bsrFile `json:"paths"`
}

type bsrFile struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

// IsBSRModule returns true if the module is hosted on Buf Schema Registry (buf.build).
func IsBSRModule(module *model.Module) bool {
	if module == nil {
		return false
	}
	name := strings.TrimSpace(module.Name)
	repo := strings.TrimSpace(module.Repository)
	return strings.HasPrefix(name, bsrPrefix) ||
		strings.HasPrefix(repo, bsrPrefix) ||
		strings.HasPrefix(repo, "https://buf.build/") ||
		strings.HasPrefix(repo, "http://buf.build/")
}

// ParseBSRModule extracts the base URL, owner, repository name, and reference (tag/branch/commit)
// from a BSR module definition.
func ParseBSRModule(module *model.Module) (baseURL, owner, repo, ref string, err error) {
	if module == nil {
		return "", "", "", "", errors.New("module is nil")
	}

	baseURL = strings.TrimSpace(os.Getenv("PBUF_BSR_URL"))
	if baseURL == "" {
		baseURL = defaultBSRBaseURL
	}

	rawTarget := strings.TrimSpace(module.Name)
	if !strings.HasPrefix(rawTarget, bsrPrefix) && module.Repository != "" {
		rawTarget = strings.TrimSpace(module.Repository)
	}

	if strings.HasPrefix(rawTarget, "http://") || strings.HasPrefix(rawTarget, "https://") {
		u, parseErr := url.Parse(rawTarget)
		if parseErr != nil {
			return "", "", "", "", fmt.Errorf("invalid BSR URL %q: %w", rawTarget, parseErr)
		}
		baseURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		rawTarget = strings.TrimPrefix(u.Path, "/")
	} else {
		rawTarget = strings.TrimPrefix(rawTarget, bsrPrefix)
	}

	// Support optional inline :ref syntax (e.g., buf.build/owner/repo:v1.0.0)
	if idx := strings.Index(rawTarget, ":"); idx != -1 {
		ref = rawTarget[idx+1:]
		rawTarget = rawTarget[:idx]
	}

	parts := strings.Split(strings.Trim(rawTarget, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", "", fmt.Errorf("invalid BSR module reference %q (expected buf.build/<owner>/<repo>)", module.Name)
	}

	owner = parts[0]
	repo = parts[1]

	if module.Tag != "" {
		ref = module.Tag
	} else if module.Branch != "" {
		ref = module.Branch
	}
	if ref == "" {
		ref = "main"
	}

	return strings.TrimSuffix(baseURL, "/"), owner, repo, ref, nil
}

// VendorBSRModule downloads .proto files from Buf Schema Registry and writes them to disk.
func VendorBSRModule(module *model.Module, netrcAuth *netrc.Netrc, patchers []patcher.Patcher) error {
	baseURL, owner, repo, ref, err := ParseBSRModule(module)
	if err != nil {
		return err
	}

	log.Printf("start vendoring .proto files from BSR: buf.build/%s/%s@%s", owner, repo, ref)

	ctx, cancel := context.WithTimeout(context.Background(), bsrTimeout)
	defer cancel()

	token := resolveBSRToken(baseURL, netrcAuth)
	files, err := downloadBSRFiles(ctx, baseURL, owner, repo, ref, token)
	if err != nil {
		return fmt.Errorf("failed to download BSR module buf.build/%s/%s@%s: %w", owner, repo, ref, err)
	}

	outputRoot := module.OutputFolder
	if outputRoot == "" {
		outputRoot = "third_party"
	}

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".proto") {
			continue
		}

		targetRelPath, include := filterAndMapPath(f.Path, module.Path, outputRoot)
		if !include {
			continue
		}

		content := string(f.Content)
		outputDir := filepath.Dir(targetRelPath)

		if module.GenerateOutputFolder != "" {
			patched, patchErr := patcher.ApplyPatchers(
				patchers,
				strings.Replace(outputDir, outputRoot, module.GenerateOutputFolder, 1),
				content,
			)
			if patchErr != nil {
				log.Printf("warning: failed to patch file %s: %v", targetRelPath, patchErr)
			} else {
				content = patched
			}
		}

		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", outputDir, err)
		}

		if err := os.WriteFile(targetRelPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", targetRelPath, err)
		}
	}

	log.Printf("successfully vendored .proto files from BSR: buf.build/%s/%s@%s", owner, repo, ref)
	return nil
}

var bsrHTTPClient = &http.Client{Timeout: bsrTimeout}

func downloadBSRFiles(ctx context.Context, baseURL, owner, repo, ref, token string) ([]bsrFile, error) {
	payload := bsrDownloadRequest{
		References: []bsrResourceRef{
			{
				Name: bsrRefName{
					Owner:      owner,
					Repository: repo,
					Ref:        ref,
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	endpoints := []string{
		baseURL + "/buf.registry.module.v1.DownloadService/Download",
		baseURL + "/buf.registry.module.v1beta1.DownloadService/Download",
	}

	client := bsrHTTPClient
	var lastErr error

	for _, endpoint := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Connect-Protocol-Version", "1")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			lastErr = fmt.Errorf("BSR endpoint %s returned 404", endpoint)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("BSR returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}

		var parsed bsrDownloadResponse
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return nil, fmt.Errorf("failed to decode BSR response: %w", err)
		}

		var result []bsrFile
		for _, c := range parsed.Contents {
			result = append(result, c.Files...)
		}
		if len(result) == 0 && len(parsed.Files) > 0 {
			result = parsed.Files
		}
		return result, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("failed to download from BSR")
}

func filterAndMapPath(filePath, filterPath, outputRoot string) (string, bool) {
	filePath = filepath.ToSlash(filePath)
	filterPath = filepath.ToSlash(strings.TrimSpace(filterPath))

	if filterPath == "" {
		return filepath.Join(outputRoot, filePath), true
	}

	if strings.HasSuffix(filterPath, ".proto") {
		if filePath != filterPath {
			return "", false
		}
		baseDir := filepath.ToSlash(filepath.Dir(filterPath))
		mapped := strings.TrimPrefix(strings.TrimPrefix(filePath, baseDir), "/")
		return filepath.Join(outputRoot, mapped), true
	}

	if !strings.HasPrefix(filePath, filterPath) {
		return "", false
	}

	mapped := strings.TrimPrefix(strings.TrimPrefix(filePath, filterPath), "/")
	return filepath.Join(outputRoot, mapped), true
}

func resolveBSRToken(baseURL string, netrcAuth *netrc.Netrc) string {
	if envToken := strings.TrimSpace(os.Getenv("BUF_TOKEN")); envToken != "" {
		return envToken
	}
	if netrcAuth == nil {
		return ""
	}

	host := "buf.build"
	if u, err := url.Parse(baseURL); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}

	m := netrcAuth.Machine(host)
	if m == nil {
		return ""
	}
	if pw := strings.TrimSpace(m.Get("password")); pw != "" && pw != "unused" {
		return pw
	}
	return strings.TrimSpace(m.Get("token"))
}
