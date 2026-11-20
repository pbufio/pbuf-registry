package patcher

import (
	"fmt"
	"go/token"
	"path"
	"strings"

	"github.com/yoheimuta/go-protoparser/v4"
	"github.com/yoheimuta/go-protoparser/v4/parser"
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

func hasUnsafeGoImportChars(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return r <= ' ' || r == '"' || r == '\\' || r == ';' || r == '\'' || r == '`' || r == 0x7f
	}) != -1
}

func (p *GoPackagePatcher) Patch(outputPath, content string) (string, error) {
	if strings.TrimSpace(p.goModule) == "" || hasUnsafeGoImportChars(p.goModule) {
		return "", fmt.Errorf("invalid Go module path %q", p.goModule)
	}

	parsed, err := protoparser.Parse(strings.NewReader(content))
	if err != nil {
		return "", err
	}

	rawOut := strings.TrimSpace(outputPath)
	cleaned := path.Clean(strings.ReplaceAll(rawOut, `\`, "/"))
	normalizedPath := strings.Trim(cleaned, "/")
	if normalizedPath != "" && normalizedPath != "." {
		if normalizedPath == ".." || strings.HasPrefix(normalizedPath, "../") || hasUnsafeGoImportChars(normalizedPath) {
			return "", fmt.Errorf("invalid go_package output path %q", outputPath)
		}
	}

	var goPackage string
	if normalizedPath == "" || normalizedPath == "." {
		pkgAlias := sanitizeGoPackageIdent(path.Base(p.goModule))
		goPackage = fmt.Sprintf(`option go_package = "%s;%s";`, p.goModule, pkgAlias)
	} else {
		pkgAlias := sanitizeGoPackageIdent(path.Base(normalizedPath))
		goPackage = fmt.Sprintf(`option go_package = "%s/%s;%s";`, p.goModule, normalizedPath, pkgAlias)
	}

	for _, elem := range parsed.ProtoBody {
		option, ok := elem.(*parser.Option)
		if !ok {
			continue
		}
		if option.OptionName == "go_package" || option.OptionName == "(go_package)" {
			start := option.Meta.Pos.Offset
			end := option.Meta.LastPos.Offset
			if start >= 0 && end >= start && end < len(content) {
				return content[:start] + goPackage + content[end+1:], nil
			}
			return "", fmt.Errorf("invalid go_package option byte offsets [%d:%d]", start, end)
		}
	}

	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}

	headerEnd := -1
	if parsed.Syntax != nil {
		headerEnd = parsed.Syntax.Meta.LastPos.Offset
	} else if parsed.Edition != nil {
		headerEnd = parsed.Edition.Meta.LastPos.Offset
	}

	if headerEnd < 0 {
		const utf8BOM = "\uFEFF"
		if strings.HasPrefix(content, utf8BOM) {
			body := content[len(utf8BOM):]
			if body == "" {
				return utf8BOM + goPackage, nil
			}
			return utf8BOM + goPackage + eol + body, nil
		}
		if content == "" {
			return goPackage, nil
		}
		return goPackage + eol + content, nil
	}

	insertAt := headerEnd + 1
	if insertAt < 0 || insertAt > len(content) {
		return "", fmt.Errorf("invalid header statement end offset %d", headerEnd)
	}
	rest := strings.TrimLeft(content[insertAt:], " \t")
	if strings.HasPrefix(rest, "//") {
		if nl := strings.IndexAny(content[insertAt:], "\r\n"); nl >= 0 {
			insertAt += nl
			rest = content[insertAt:]
		} else {
			insertAt = len(content)
			rest = ""
		}
	}
	if rest == "" {
		return content[:insertAt] + eol + goPackage, nil
	}
	if rest[0] == '\n' || rest[0] == '\r' {
		return content[:insertAt] + eol + goPackage + rest, nil
	}
	return content[:insertAt] + eol + goPackage + eol + rest, nil
}
