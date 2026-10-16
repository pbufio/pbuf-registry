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
		Progress:      os.Stdout,
	})
	if err != nil {
		log.Printf("failed to clone repository: %s", module.Repository)
		return err
	}

	modulePath := filepath.Clean(module.Path)
	if strings.HasSuffix(modulePath, ".proto") {
		modulePath = filepath.Dir(modulePath)
	}

	baseDir := modulePath
	if module.OutputFolder != "" {
		baseDir = module.OutputFolder
	}

	err = util.Walk(fs, module.Path, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			log.Printf("failed to walk by path: %s", path)
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(path, ".proto") {
			return nil
		}

		outputPath := path
		if relPath, relErr := filepath.Rel(modulePath, path); relErr == nil {
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

		content := string(fileContents)
		outputDir := filepath.Dir(outputPath)

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
				content,
			)
			if patchErr != nil {
				log.Printf("failed to patch file %s: %v", outputPath, patchErr)
			} else {
				content = patched
			}
		}

		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", outputDir, err)
		}
		if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", outputPath, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Printf("successfully vendoring .proto files. repo: %s, path: %s", module.Repository, module.Path)

	return nil
}
