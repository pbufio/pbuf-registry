package registry

import (
	"context"
	"log"
	"time"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/patcher"
)

const timeout = 60 * time.Second

// VendorRegistryModule function that iterate over the modules and vendor proto files from PBUF registry
func VendorRegistryModule(module *model.Module, client v1.RegistryClient, patchers []patcher.Patcher) error {
	log.Printf("start vendoring .proto files. module name: %s, path: %s", module.Name, module.Path)

	outputRoot, err := patcher.NormalizeRelativeDir(module.OutputFolder)
	if err != nil {
		return err
	}
	moduleSubpath, err := patcher.NormalizeModuleSubpath(module.Path)
	if err != nil {
		return err
	}

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
		targetPath, include := patcher.MapVendoredProtoPath(protoFile.Filename, moduleSubpath, outputRoot)
		if !include {
			continue
		}
		if err := patcher.WriteVendoredProto(targetPath, outputRoot, module.GenerateOutputFolder, protoFile.Content, patchers); err != nil {
			return err
		}
	}

	log.Printf("successfully vendoring .proto files. module name: %s, path: %s", module.Name, module.Path)

	return nil
}
