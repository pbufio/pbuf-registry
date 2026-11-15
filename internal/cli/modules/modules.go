package modules

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jdx/go-netrc"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/bsr"
	"github.com/pbufio/pbuf-registry/internal/cli/git"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/patcher"
	"github.com/pbufio/pbuf-registry/internal/cli/registry"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
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

func depOutputKey(name, outputFolder string) string {
	return name + "\x00" + filepath.Clean(outputFolder)
}

func normalizeSemver(tag string) string {
	if strings.HasPrefix(tag, "v") {
		return tag
	}
	return "v" + tag
}

// resolveTransitiveDependencies fetches direct and multi-hop transitive dependencies
// from the registry for each configured registry module so protoc has the full import closure.
func resolveTransitiveDependencies(config *model.Config, client v1.RegistryClient) []*model.Module {
	configured := make(map[string]bool)
	for _, m := range config.Modules {
		if m.Name != "" && !bsr.IsBSRModule(m) && m.Repository == "" {
			configured[depOutputKey(m.Name, m.OutputFolder)] = true
		}
	}

	var additional []*model.Module
	seenIdx := make(map[string]int)

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
			if dep.Name == "" || dep.Tag == "" || dep.Name == config.Name {
				continue
			}

			seenKey := depOutputKey(dep.Name, module.OutputFolder)
			if configured[seenKey] {
				continue
			}
			if idx, exists := seenIdx[seenKey]; exists {
				existing := additional[idx]
				if existing.Tag != dep.Tag {
					oldVer, newVer := normalizeSemver(existing.Tag), normalizeSemver(dep.Tag)
					if semver.IsValid(newVer) && (!semver.IsValid(oldVer) || semver.Compare(newVer, oldVer) > 0) {
						log.Printf("warning: transitive dependency %s version mismatch (%s vs %s); using %s", dep.Name, existing.Tag, dep.Tag, dep.Tag)
						existing.Tag = dep.Tag
					} else {
						log.Printf("warning: transitive dependency %s version mismatch (%s vs %s); keeping %s", dep.Name, existing.Tag, dep.Tag, existing.Tag)
					}
				}
				continue
			}

			seenIdx[seenKey] = len(additional)
			additional = append(additional, &model.Module{
				Name:                 dep.Name,
				Tag:                  dep.Tag,
				OutputFolder:         module.OutputFolder,
				GenerateOutputFolder: module.GenerateOutputFolder,
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
				return fmt.Errorf("failed to vendor BSR module %s: %w", module.Name, err)
			}
		case module.Repository == "":
			if config.HasRegistry() {
				if module.Name == "" {
					return fmt.Errorf("no module name found for module: %v", module)
				}

				if module.Tag == "" {
					return fmt.Errorf("no module tag found for module: %v", module)
				}

				err := registry.VendorRegistryModule(module, client, patchers)
				if err != nil {
					return fmt.Errorf("failed to vendor module %s: %w", module.Name, err)
				}
			} else {
				return fmt.Errorf("no repository found for module: %s", module.Name)
			}
		default:
			err := git.VendorGitModule(module, netrcAuth, patchers)
			if err != nil {
				return fmt.Errorf("failed to vendor module %s: %w", module.Repository, err)
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
				return fmt.Errorf("failed to vendor transitive dependency %s: %w", module.Name, err)
			}
		}
	}

	return nil
}
