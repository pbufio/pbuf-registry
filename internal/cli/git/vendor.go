package git

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/jdx/go-netrc"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/patcher"
)

func VendorGitModule(module *model.Module, netrcAuth *netrc.Netrc, patchers []patcher.Patcher) error {
	log.Printf("start vendoring .proto files. repo: %s, path: %s", module.Repository, module.Path)

	outputRoot, err := patcher.NormalizeRelativeDir(module.OutputFolder)
	if err != nil {
		return err
	}
	moduleSubpath, err := patcher.NormalizeModuleSubpath(module.Path)
	if err != nil {
		return err
	}

	var reference plumbing.ReferenceName
	if module.Branch != "" {
		reference = plumbing.NewBranchReferenceName(module.Branch)
	} else if module.Tag != "" {
		reference = plumbing.NewTagReferenceName(module.Tag)
	}

	parsed, err := url.Parse(module.Repository)
	if err != nil {
		log.Printf("failed to parse url: %s", module.Repository)
		return err
	}

	var auth transport.AuthMethod
	if netrcAuth != nil {
		if machine := netrcAuth.Machine(parsed.Host); machine != nil {
			auth = &http.BasicAuth{Username: machine.Get("login"), Password: machine.Get("password")}
		}
	}

	fs := memfs.New()

	_, err = git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		URL:           module.Repository,
		Auth:          auth,
		ReferenceName: reference,
		SingleBranch:  true,
		Depth:         1,
		Progress:      os.Stderr,
	})
	if err != nil {
		log.Printf("failed to clone repository: %s", module.Repository)
		return err
	}

	walkRoot := "."
	moduleDir := "."
	if moduleSubpath != "" {
		walkRoot = filepath.ToSlash(moduleSubpath)
		moduleDir = moduleSubpath
		if strings.HasSuffix(moduleDir, ".proto") {
			moduleDir = filepath.Dir(moduleDir)
		}
	}

	baseDir := moduleDir
	if outputRoot != "" {
		baseDir = outputRoot
	}

	err = util.Walk(fs, walkRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			log.Printf("failed to walk by path: %s", path)
			return walkErr
		}
		if !info.Mode().IsRegular() || !strings.HasSuffix(path, ".proto") {
			return nil
		}

		cleanWalkPath := filepath.Clean(filepath.FromSlash(path))
		outputPath := cleanWalkPath
		if relPath, relErr := filepath.Rel(moduleDir, cleanWalkPath); relErr == nil {
			outputPath = filepath.Join(baseDir, relPath)
		}

		file, err := fs.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file in repository %s: %w", path, err)
		}
		fileContents, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			return fmt.Errorf("failed to read file contents in repository %s: %w", path, err)
		}

		return patcher.WriteVendoredProto(outputPath, outputRoot, module.GenerateOutputFolder, string(fileContents), patchers)
	})
	if err != nil {
		return err
	}

	log.Printf("successfully vendoring .proto files. repo: %s, path: %s", module.Repository, module.Path)

	return nil
}
