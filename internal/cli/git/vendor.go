package git

import (
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
		// clone repository with branch
		reference = plumbing.NewBranchReferenceName(module.Branch)
	} else if module.Tag != "" {
		// clone repository with tag
		reference = plumbing.NewTagReferenceName(module.Tag)
	}

	parsed, err := url.Parse(module.Repository)
	if err != nil {
		log.Printf("failed to parse url: %s", module.Repository)
		return err
	}

	var auth transport.AuthMethod
	if netrcAuth != nil {
		machine := netrcAuth.Machine(parsed.Host)
		if machine != nil {
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

	var modulePath string
	if strings.HasSuffix(module.Path, ".proto") {
		modulePath = filepath.Dir(module.Path)
	} else {
		modulePath = module.Path
	}

	baseDir := modulePath
	if module.OutputFolder != "" {
		baseDir = module.OutputFolder
	}

	err = os.MkdirAll(baseDir, os.ModePerm)
	if err != nil {
		log.Printf("failed to create directory: %s", baseDir)
		return err
	}

	var wg = &sync.WaitGroup{}

	err = util.Walk(fs, module.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("failed to walk by path: %s", path)
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			outputPath := strings.ReplaceAll(path, modulePath, baseDir)

			if info.IsDir() {
				err := os.Mkdir(outputPath, os.ModePerm)
				if err != nil {
					if os.IsExist(err) {
						return
					}

				}

				return
			}

			// skip if not a proto file
			if !strings.HasSuffix(path, ".proto") {
				return
			}

			file, err := fs.Open(path)
			if err != nil {
				log.Fatalf("failed to open file in repository: %s", path)
			}

			fileContents, err := io.ReadAll(file)
			if err != nil {
				log.Fatalf("failed to read file contents in repository: %s", path)
			}

			var content string
			outputDir := filepath.Dir(outputPath)

			if module.GenerateOutputFolder != "" {
				content, err = patcher.ApplyPatchers(
					patchers,
					strings.Replace(outputDir, module.OutputFolder, module.GenerateOutputFolder, 1),
					string(fileContents),
				)
				if err != nil {
					log.Printf("failed to patch file %s: %v", outputPath, err)
				}
			} else {
				content = string(fileContents)
			}

			if err != nil {
				log.Printf("failed to patch file %s: %v", outputPath, err)
			}

			copiedFile, err := os.Create(outputPath)
			if err != nil {
				log.Fatalf("failed to create file: %s", outputPath)
			}

			_, err = copiedFile.Write([]byte(content))
			if err != nil {
				log.Fatalf("failed to write file contents: %s", outputPath)
			}
		}()

		return nil
	})

	if err != nil {
		return err
	}

	wg.Wait()

	log.Printf("successfully vendoring .proto files. repo: %s, path: %s", module.Repository, module.Path)

	return nil
}
