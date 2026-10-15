package registry

import (
	"io"
	"log"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
)

func CollectProtoFilesInDirs(dirs []string) ([]*v1.ProtoFile, error) {
	var protoFiles []*v1.ProtoFile

	for _, dir := range dirs {
		fs := osfs.New(".")
		err := util.Walk(fs, dir, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() && strings.HasSuffix(path, ".proto") {
				file, err := fs.Open(path)
				if err != nil {
					log.Printf("failed to open file in repository: %s", path)
					return err
				}

				fileContents, err := io.ReadAll(file)
				if err != nil {
					log.Printf("failed to read file contents in repository: %s", path)
					return err
				}

				protoFile := &v1.ProtoFile{
					Filename: path,
					Content:  string(fileContents),
				}

				protoFiles = append(protoFiles, protoFile)
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return protoFiles, nil
}
