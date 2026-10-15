package patcher

import (
	"fmt"
	"strings"

	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
)
import "github.com/yoheimuta/go-protoparser/v4"

type GoPackagePatcher struct {
	goModule string
}

func NewGoPackagePatcher(goModule string) *GoPackagePatcher {
	return &GoPackagePatcher{
		goModule: goModule,
	}
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

	dirs := strings.Split(outputPath, "/")
	goPackage := fmt.Sprintf(`option go_package = "%s/%s;%s";`, p.goModule, outputPath, dirs[len(dirs)-1])

	for _, option := range proto.ProtoBody.Options {
		if option.OptionName == "go_package" {
			// break by lines
			// option.Meta.Pos.Line as the line to change
			splitted := strings.Split(content, "\n")
			splitted[option.Meta.Pos.Line-1] = goPackage
			return strings.Join(splitted, "\n"), nil
		}
	}

	// if no go_package option, add it
	splitted := strings.Split(content, "\n")
	// add the element after syntax line
	line := proto.Syntax.Meta.LastPos.Line
	splitted = append(splitted[:line], append([]string{goPackage}, splitted[line:]...)...)

	return strings.Join(splitted, "\n"), nil
}
