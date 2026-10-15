package registry

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	var wg = &sync.WaitGroup{}

	for _, protoFile := range response.Protofiles {
		originalFilename := protoFile.Filename
		protoFileContent := protoFile.Content
		outputPath := module.OutputFolder

		wg.Add(1)
		go func() {
			defer wg.Done()

			if module.Path != "" {
				modulePath := module.Path

				if strings.HasSuffix(module.Path, ".proto") {
					// skip if the file is not in the module path
					if originalFilename != module.Path {
						return
					}

					// get directory
					modulePath = filepath.Dir(module.Path)
				} else {
					// skip if the file is not in the module path
					if !strings.HasPrefix(originalFilename, modulePath) {
						return
					}
				}

				if outputPath != "" {
					originalFilename = strings.Replace(originalFilename, modulePath, outputPath, 1)
				}
			} else {
				if outputPath != "" {
					originalFilename = filepath.Join(outputPath, originalFilename)
				}
			}

			var content string
			outputDir := filepath.Dir(originalFilename)

			if module.GenerateOutputFolder != "" {
				content, err = patcher.ApplyPatchers(
					patchers,
					strings.Replace(outputDir, module.OutputFolder, module.GenerateOutputFolder, 1),
					protoFileContent,
				)
				if err != nil {
					log.Printf("failed to patch file %s: %v", originalFilename, err)
				}
			} else {
				content = protoFileContent
			}

			err = os.MkdirAll(outputDir, os.ModePerm)
			if err != nil {
				log.Fatalf("failed to create directory: %s", outputPath)
			}

			copiedFile, err := os.Create(originalFilename)
			if err != nil {
				log.Fatalf("failed to create file: %s", outputPath)
			}

			_, err = copiedFile.Write([]byte(content))
			if err != nil {
				log.Fatalf("failed to write file contents: %s", outputPath)
			}
		}()
	}

	wg.Wait()

	log.Printf("successfully vendoring .proto files. module name: %s, path: %s", module.Name, module.Path)

	return nil
}
