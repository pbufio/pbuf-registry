package patcher

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Patcher interface {
	Patch(outputPath, content string) (string, error)
}

func ApplyPatchers(patchers []Patcher, outputPath string, content string) (string, error) {
	for _, patcher := range patchers {
		var err error
		content, err = patcher.Patch(outputPath, content)
		if err != nil {
			return "", err
		}
	}
	return content, nil
}

// NormalizeRelativeDir validates a workspace-relative directory (such as out or gen_out).
// Empty string is returned unchanged when unset.
func NormalizeRelativeDir(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	slash := strings.ReplaceAll(trimmed, `\`, "/")
	if slash == "" {
		return "", nil
	}
	if strings.HasPrefix(slash, "/") || filepath.VolumeName(filepath.FromSlash(slash)) != "" {
		return "", fmt.Errorf("invalid output path %q: must be relative to workspace", raw)
	}
	cleaned := filepath.Clean(filepath.FromSlash(slash))
	if !filepath.IsLocal(cleaned) {
		return "", fmt.Errorf("invalid output path %q: must stay within workspace", raw)
	}
	return cleaned, nil
}

// NormalizeModuleSubpath validates a subpath filter inside a remote module or repository.
// Leading/trailing slashes and root-equivalent "." paths normalize to ""; ".." traversal is rejected.
func NormalizeModuleSubpath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	slash := strings.Trim(strings.ReplaceAll(trimmed, `\`, "/"), "/")
	if slash == "" || slash == "." {
		return "", nil
	}
	if filepath.VolumeName(filepath.FromSlash(slash)) != "" {
		return "", fmt.Errorf("invalid module path %q: must not traverse outside module root", raw)
	}
	cleaned := filepath.Clean(filepath.FromSlash(slash))
	if cleaned == "." {
		return "", nil
	}
	if !filepath.IsLocal(cleaned) {
		return "", fmt.Errorf("invalid module path %q: must not traverse outside module root", raw)
	}
	return cleaned, nil
}

// MapVendoredProtoPath validates a remote .proto file path, filters it by moduleSubpath,
// and maps it under outputRoot. Returns ("", false) when the file should be skipped.
func MapVendoredProtoPath(remotePath, moduleSubpath, outputRoot string) (string, bool) {
	trimmedRemote := strings.TrimSpace(remotePath)
	rawSlash := strings.ReplaceAll(trimmedRemote, `\`, "/")
	if rawSlash == "" || strings.HasPrefix(rawSlash, "/") || filepath.VolumeName(filepath.FromSlash(rawSlash)) != "" {
		return "", false
	}
	cleanRemote := filepath.Clean(filepath.FromSlash(rawSlash))
	if cleanRemote == "." || !filepath.IsLocal(cleanRemote) || !strings.HasSuffix(cleanRemote, ".proto") {
		return "", false
	}

	sub, err := NormalizeModuleSubpath(moduleSubpath)
	if err != nil {
		return "", false
	}
	if sub == "" {
		if outputRoot != "" {
			return filepath.Join(outputRoot, cleanRemote), true
		}
		return cleanRemote, true
	}

	filterDir := sub
	if strings.HasSuffix(sub, ".proto") {
		if cleanRemote != sub {
			return "", false
		}
		filterDir = filepath.Dir(sub)
	} else if cleanRemote != sub && !strings.HasPrefix(cleanRemote, sub+string(filepath.Separator)) {
		return "", false
	}

	if outputRoot != "" {
		if rel, relErr := filepath.Rel(filterDir, cleanRemote); relErr == nil {
			return filepath.Join(outputRoot, rel), true
		}
	}
	return cleanRemote, true
}

// WriteVendoredProto applies optional patchers and writes a .proto file confined to the current workspace root.
func WriteVendoredProto(targetPath, outputRoot, genOutFolder, content string, patchers []Patcher) error {
	cleanRoot, err := NormalizeRelativeDir(outputRoot)
	if err != nil {
		return err
	}
	cleanGenOut, err := NormalizeRelativeDir(genOutFolder)
	if err != nil {
		return err
	}

	trimmedTarget := strings.TrimSpace(targetPath)
	targetSlash := strings.ReplaceAll(trimmedTarget, `\`, "/")
	if strings.HasPrefix(targetSlash, "/") || filepath.VolumeName(filepath.FromSlash(targetSlash)) != "" {
		return fmt.Errorf("invalid vendor target path %q: must be relative to workspace", targetPath)
	}
	cleaned := filepath.Clean(filepath.FromSlash(targetSlash))
	if cleaned == "." || !filepath.IsLocal(cleaned) || !strings.HasSuffix(cleaned, ".proto") {
		return fmt.Errorf("invalid vendor target path %q: must be a workspace-local .proto file", targetPath)
	}

	outputDir := filepath.Dir(cleaned)
	if cleanGenOut != "" {
		genDir := filepath.Join(cleanGenOut, outputDir)
		if cleanRoot != "" {
			if rel, relErr := filepath.Rel(cleanRoot, outputDir); relErr == nil && (rel == "." || filepath.IsLocal(rel)) {
				genDir = filepath.Join(cleanGenOut, rel)
			}
		}
		patched, patchErr := ApplyPatchers(patchers, genDir, content)
		if patchErr != nil {
			log.Printf("warning: failed to patch file %s: %v", cleaned, patchErr)
		} else {
			content = patched
		}
	}

	root, err := os.OpenRoot(".")
	if err != nil {
		return fmt.Errorf("failed to open workspace root: %w", err)
	}
	defer func() { _ = root.Close() }()

	for prefix := cleaned; prefix != "." && prefix != ""; prefix = filepath.Dir(prefix) {
		fi, statErr := root.Lstat(prefix)
		if statErr != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write vendored proto through symlink %q", prefix)
		}
		if prefix == cleaned && !fi.Mode().IsRegular() {
			return fmt.Errorf("refusing to overwrite non-regular file %q", cleaned)
		}
	}

	if outputDir != "." {
		if err := root.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", outputDir, err)
		}
	}
	if err := root.WriteFile(cleaned, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", cleaned, err)
	}
	return nil
}
