package modules

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jdx/go-netrc"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/bsr"
	"github.com/pbufio/pbuf-registry/internal/cli/git"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/patcher"
	"github.com/pbufio/pbuf-registry/internal/cli/registry"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v2"
)

// NewConfig create a struct for bytes array
func NewConfig(contents []byte) (*model.Config, error) {
	modulesConfig := &model.Config{}
	err := yaml.Unmarshal(contents, modulesConfig)
	if err != nil {
		return nil, err
	}
	return modulesConfig, nil
}

// newProtoPatchers create a slice of proto patchers
func newProtoPatchers() []patcher.Patcher {
	var result []patcher.Patcher

	// if we have go.mod file
	// then parse it and fetch the module name
	// and pass it to the go package patcher
	file, err := os.ReadFile("go.mod")
	if err == nil {
		// that's ok, we cannot find go mod file
		path := modfile.ModulePath(file)
		if path != "" {
			result = append(result, patcher.NewGoPackagePatcher(path))
		}
	}
	return result
}

const depsTimeout = 60 * time.Second

// resolveTransitiveDependencies fetches transitive dependencies from the registry
// for each configured registry module and returns additional modules to vendor.
// Only "direct" type transitive dependencies are returned (needed for compilation).
func resolveTransitiveDependencies(config *model.Config, client v1.RegistryClient) []*model.Module {
	configured := make(map[string]bool)
	for _, m := range config.Modules {
		if m.Name != "" {
			configured[m.Name] = true
		}
	}

	var additional []*model.Module
	seen := make(map[string]bool)

	for _, module := range config.Modules {
		if bsr.IsBSRModule(module) || module.Repository != "" || module.Name == "" || module.Tag == "" {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), depsTimeout)
		resp, err := client.GetModuleDependencies(ctx, &v1.GetModuleDependenciesRequest{
			Name:              module.Name,
			Tag:               module.Tag,
			ResolveTransitive: true,
		})
		cancel()

		if err != nil {
			log.Printf("warning: failed to resolve transitive dependencies for %s: %v", module.Name, err)
			continue
		}

		for _, dep := range resp.Dependencies {
			if dep.DependencyType != "direct" {
				continue
			}

			if configured[dep.Name] || seen[dep.Name] {
				continue
			}

			seen[dep.Name] = true
			additional = append(additional, &model.Module{
				Name: dep.Name,
				Tag:  dep.Tag,
			})
		}
	}

	return additional
}

// Vendor iterates over configured modules and vendors proto files from BSR, Git, or the PBUF registry.
func Vendor(config *model.Config, netrcAuth *netrc.Netrc, client v1.RegistryClient) error {
	patchers := newProtoPatchers()

	for _, module := range config.Modules {
		switch {
		case bsr.IsBSRModule(module):
			err := bsr.VendorBSRModule(module, netrcAuth, patchers)
			if err != nil {
				log.Fatalf("failed to vendor BSR module %s: %v", module.Name, err)
			}
		case module.Repository == "":
			if config.HasRegistry() {
				if module.Name == "" {
					log.Fatalf("no module name found for module: %v", module)
				}

				if module.Tag == "" {
					log.Fatalf("no module tag found for module: %v", module)
				}

				err := registry.VendorRegistryModule(module, client, patchers)
				if err != nil {
					log.Fatalf("failed to vendor module %s: %v", module.Name, err)
				}
			} else {
				log.Fatalf("no repository found for module: %s", module.Name)
			}
		default:
			err := git.VendorGitModule(module, netrcAuth, patchers)
			if err != nil {
				log.Fatalf("failed to vendor module %s: %v", module.Repository, err)
			}
		}
	}

	// resolve and vendor transitive dependencies from registry
	if config.HasRegistry() && client != nil {
		transitiveDeps := resolveTransitiveDependencies(config, client)
		for _, module := range transitiveDeps {
			log.Printf("vendoring transitive dependency: %s@%s", module.Name, module.Tag)
			err := registry.VendorRegistryModule(module, client, patchers)
			if err != nil {
				log.Fatalf("failed to vendor transitive dependency %s: %v", module.Name, err)
			}
		}
	}

	return nil
}
