package registry

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
)

func CollectProtoFilesInDirs(dirs []string) ([]*v1.ProtoFile, error) {
	return CollectProtoFilesInRoot(".", dirs)
}

func CollectProtoFilesInRoot(rootDir string, dirs []string) ([]*v1.ProtoFile, error) {
	if rootDir == "" {
		rootDir = "."
	}
	var protoFiles []*v1.ProtoFile
	seen := make(map[string]struct{})
	fs := osfs.New(rootDir)

	for _, dir := range dirs {
		cleanDir := filepath.Clean(dir)
		if filepath.IsAbs(dir) || cleanDir == ".." || strings.HasPrefix(cleanDir, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("invalid proto export path %q: must be relative to project root", dir)
		}
		err := util.Walk(fs, cleanDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			norm := filepath.ToSlash(path)
			if info.Mode().IsRegular() && strings.HasSuffix(norm, ".proto") {
				if _, dup := seen[norm]; dup {
					return nil
				}
				seen[norm] = struct{}{}

				file, err := fs.Open(path)
				if err != nil {
					return err
				}

				fileContents, err := io.ReadAll(file)
				_ = file.Close()
				if err != nil {
					return err
				}

				protoFiles = append(protoFiles, &v1.ProtoFile{
					Filename: norm,
					Content:  string(fileContents),
				})
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return protoFiles, nil
}
