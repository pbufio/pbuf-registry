package patcher

import (
	"fmt"
	"go/token"
	"path"
	"strings"

	"github.com/yoheimuta/go-protoparser/v4"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
)

type GoPackagePatcher struct {
	goModule string
}

func NewGoPackagePatcher(goModule string) *GoPackagePatcher {
	return &GoPackagePatcher{
		goModule: goModule,
	}
}

func sanitizeGoPackageIdent(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	ident := strings.Trim(b.String(), "_")
	if ident == "" {
		return "pb"
	}
	if ident[0] >= '0' && ident[0] <= '9' {
		ident = "pb_" + ident
	}
	if token.Lookup(ident).IsKeyword() {
		ident += "_pb"
	}
	return ident
}

func (p *GoPackagePatcher) Patch(outputPath, content string) (string, error) {
	parsed, err := protoparser.Parse(strings.NewReader(content))
	if err != nil {
		return "", err
	}

	proto, err := unordered.InterpretProto(parsed)
	if err != nil {
		return "", err
	}

	cleaned := path.Clean(strings.ReplaceAll(outputPath, `\`, "/"))
	normalizedPath := strings.Trim(cleaned, "/")
	var goPackage string
	if normalizedPath == "" || normalizedPath == "." {
		pkgAlias := sanitizeGoPackageIdent(path.Base(p.goModule))
		goPackage = fmt.Sprintf(`option go_package = "%s;%s";`, p.goModule, pkgAlias)
	} else {
		pkgAlias := sanitizeGoPackageIdent(path.Base(normalizedPath))
		goPackage = fmt.Sprintf(`option go_package = "%s/%s;%s";`, p.goModule, normalizedPath, pkgAlias)
	}

	for _, option := range proto.ProtoBody.Options {
		if option.OptionName == "go_package" {
			start := option.Meta.Pos.Offset
			end := option.Meta.LastPos.Offset
			if start >= 0 && end >= start && end < len(content) {
				return content[:start] + goPackage + content[end+1:], nil
			}
			splitted := strings.Split(content, "\n")
			splitted[option.Meta.Pos.Line-1] = goPackage
			return strings.Join(splitted, "\n"), nil
		}
	}

	// if no go_package option, add it
	splitted := strings.Split(content, "\n")
	line := 0
	if proto.Syntax != nil {
		line = proto.Syntax.Meta.LastPos.Line
	}
	splitted = append(splitted[:line], append([]string{goPackage}, splitted[line:]...)...)

	return strings.Join(splitted, "\n"), nil
}
