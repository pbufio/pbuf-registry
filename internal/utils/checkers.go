package utils

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/martian/log"
	v1 "github.com/pbufio/pbuf-registry/gen/v1"
	"github.com/yoheimuta/go-protoparser/v4"
)

// ValidateProtoFiles validates proto files
func ValidateProtoFiles(protoFiles []*v1.ProtoFile) error {
	for _, protoFile := range protoFiles {
		if protoFile.Filename == "" {
			return errors.New("filename cannot be empty")
		}

		if protoFile.Content == "" {
			return errors.New(fmt.Sprintf("content of %s cannot be empty", protoFile.Filename))
		}

		// check that file contents is valid
		err := parseProtoFile(protoFile.Filename, protoFile.Content)
		if err != nil {
			return fmt.Errorf("invalid proto file %s: %w", protoFile.Filename, err)
		}
	}

	return nil
}

func parseProtoFile(filename, content string) error {
	_, err := protoparser.Parse(strings.NewReader(content))
	if err != nil {
		log.Infof("error parsing proto file %s: %v", filename, err)
		return err
	}

	return nil
}
