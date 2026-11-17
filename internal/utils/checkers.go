package utils

import (
	"errors"
	"fmt"
	"path"
	"strings"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
)

func hasControlRune(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// ValidateProtoFiles validates proto files
func ValidateProtoFiles(protoFiles []*v1.ProtoFile) error {
	if len(protoFiles) == 0 {
		return errors.New("at least one proto file is required")
	}

	seen := make(map[string]struct{}, len(protoFiles))
	for _, protoFile := range protoFiles {
		if protoFile == nil || protoFile.Filename == "" {
			return errors.New("filename cannot be empty")
		}

		fn := protoFile.Filename
		if strings.Contains(fn, `\`) || strings.Contains(fn, ":") || hasControlRune(fn) || strings.HasPrefix(fn, "/") || path.Clean(fn) != fn || strings.HasPrefix(fn, "../") || !strings.HasSuffix(fn, ".proto") || path.Base(fn) == ".proto" {
			return fmt.Errorf("invalid proto filename %q: must be a clean relative .proto path", fn)
		}
		if _, dup := seen[fn]; dup {
			return fmt.Errorf("duplicate proto filename %q", fn)
		}
		seen[fn] = struct{}{}

		if protoFile.Content == "" {
			return fmt.Errorf("content of %s cannot be empty", fn)
		}

		// check that file contents is valid
		_, err := parseProtoFile(protoFile.Content)
		if err != nil {
			return fmt.Errorf("invalid proto file %s: %w", fn, err)
		}
	}

	return nil
}
