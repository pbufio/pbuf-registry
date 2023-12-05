package utils

import (
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
)

// ValidateProtoFiles validates proto files
func ValidateProtoFiles(protoFiles []*v1.ProtoFile, logger *log.Helper) error {
	for _, protoFile := range protoFiles {
		if protoFile.Filename == "" {
			return errors.New("filename cannot be empty")
		}

		if protoFile.Content == "" {
			return fmt.Errorf("content of %s cannot be empty", protoFile.Filename)
		}

		// check that file contents is valid
		_, err := parseProtoFile(protoFile.Content)
		if err != nil {
			return fmt.Errorf("invalid proto file %s: %w", protoFile.Filename, err)
		}
	}

	return nil
}
