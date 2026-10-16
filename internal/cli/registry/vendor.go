package registry

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/patcher"
)

const timeout = 60 * time.Second

// VendorRegistryModule function that iterate over the modules and vendor proto files from PBUF registry
func VendorRegistryModule(module *model.Module, client v1.RegistryClient, patchers []patcher.Patcher) error {
	log.Printf("start vendoring .proto files. module name: %s, path: %s", module.Name, module.Path)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	response, err := client.PullModule(ctx, &v1.PullModuleRequest{
		Name: module.Name,
		Tag:  module.Tag,
	})
	if err != nil {
		log.Printf("failed to pull module: %v", err)
		return err
	}

	for _, protoFile := range response.Protofiles {
		originalFilename := filepath.Clean(protoFile.Filename)
		if originalFilename == "." || strings.HasPrefix(originalFilename, "..") || filepath.IsAbs(originalFilename) || !strings.HasSuffix(originalFilename, ".proto") {
			continue
		}
		protoFileContent := protoFile.Content
		outputPath := module.OutputFolder

		if module.Path != "" {
			modulePath := filepath.Clean(module.Path)

			if strings.HasSuffix(modulePath, ".proto") {
				if originalFilename != modulePath {
					continue
				}
				modulePath = filepath.Dir(modulePath)
			} else if modulePath != "." && originalFilename != modulePath && !strings.HasPrefix(originalFilename, modulePath+string(filepath.Separator)) {
				continue
			}

			if outputPath != "" {
				if relPath, relErr := filepath.Rel(modulePath, originalFilename); relErr == nil {
					originalFilename = filepath.Join(outputPath, relPath)
				}
			}
		} else if outputPath != "" {
			originalFilename = filepath.Join(outputPath, originalFilename)
		}

		content := protoFileContent
		outputDir := filepath.Dir(originalFilename)

		if module.GenerateOutputFolder != "" {
			genDir := filepath.Join(module.GenerateOutputFolder, outputDir)
			if module.OutputFolder != "" {
				if rel, relErr := filepath.Rel(module.OutputFolder, outputDir); relErr == nil {
					genDir = filepath.Join(module.GenerateOutputFolder, rel)
				}
			}
			patched, patchErr := patcher.ApplyPatchers(
				patchers,
				genDir,
				protoFileContent,
			)
			if patchErr != nil {
				log.Printf("failed to patch file %s: %v", originalFilename, patchErr)
			} else {
				content = patched
			}
		}

		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", outputDir, err)
		}
		if err := os.WriteFile(originalFilename, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", originalFilename, err)
		}
	}

	log.Printf("successfully vendoring .proto files. module name: %s, path: %s", module.Name, module.Path)

	return nil
}
