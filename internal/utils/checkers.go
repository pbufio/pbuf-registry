package utils

import (
	"errors"
	"fmt"
	"path"
	"strings"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
)

const maxIdentifierBytes = 255

func hasControlRune(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// ValidateModuleName validates a module identifier before persistence.
func ValidateModuleName(name string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}
	if name == "*" {
		return errors.New(`module name "*" is reserved`)
	}
	if len(name) > maxIdentifierBytes {
		return fmt.Errorf("module name exceeds %d characters", maxIdentifierBytes)
	}
	if strings.TrimSpace(name) != name || hasControlRune(name) || strings.ContainsAny(name, " \t\\") {
		return fmt.Errorf("invalid module name %q", name)
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("invalid module name %q", name)
		}
	}
	return nil
}

// ValidateTagName validates a module version tag before persistence.
func ValidateTagName(tag string) error {
	if tag == "" {
		return errors.New("tag cannot be empty")
	}
	if tag == "*" || tag == "." || tag == ".." {
		return fmt.Errorf("invalid tag %q", tag)
	}
	if len(tag) > maxIdentifierBytes {
		return fmt.Errorf("tag exceeds %d characters", maxIdentifierBytes)
	}
	if strings.TrimSpace(tag) != tag || hasControlRune(tag) || strings.ContainsAny(tag, " \t/\\") {
		return fmt.Errorf("invalid tag %q", tag)
	}
	return nil
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

		parsed, err := parseProtoFile(protoFile.Content)
		if err != nil {
			return fmt.Errorf("invalid proto file %s: %w", fn, err)
		}
		if _, err := unordered.InterpretProto(parsed); err != nil {
			return fmt.Errorf("invalid proto file %s: %w", fn, err)
		}
	}

	return nil
}
