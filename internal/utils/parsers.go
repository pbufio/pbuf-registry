package utils

import (
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/yoheimuta/go-protoparser/v4"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
	"github.com/yoheimuta/go-protoparser/v4/parser"
)

func ParseProtoFilesContents(protoFiles []*v1.ProtoFile) ([]*model.ParsedProtoFile, error) {
	parsedProtoFiles := make([]*model.ParsedProtoFile, len(protoFiles))

	for i, protoFile := range protoFiles {
		if protoFile.Content == "" {
			return parsedProtoFiles, fmt.Errorf("content of %s cannot be empty", protoFile.Filename)
		}

		// check that file contents is valid
		parsed, err := parseProtoFile(protoFile.Content)
		if err != nil {
			return parsedProtoFiles, fmt.Errorf("invalid proto file %s: %w", protoFile.Filename, err)
		}

		proto, err := unordered.InterpretProto(parsed)
		if err != nil {
			return parsedProtoFiles, fmt.Errorf("cannot interpret file %s: %w", protoFile.Filename, err)
		}

		jsonContent, err := json.Marshal(proto)
		if err != nil {
			return parsedProtoFiles, fmt.Errorf("cannot marshall %s: %w", protoFile.Filename, err)
		}

		parsedProtoFiles[i] = &model.ParsedProtoFile{
			Filename:  protoFile.Filename,
			Proto:     proto,
			ProtoJson: string(jsonContent),
		}
	}

	return parsedProtoFiles, nil
}

func parseProtoFile(content string) (*parser.Proto, error) {
	parsed, err := protoparser.Parse(strings.NewReader(content))
	if err != nil {
		return nil, err
	}

	return parsed, nil
}
