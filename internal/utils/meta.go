package utils

import (
	"sort"
	"strings"

	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
	"github.com/yoheimuta/go-protoparser/v4/parser"
	"golang.org/x/exp/maps"
)

// RetrieveMeta retrieves metadata from parsed proto files
func RetrieveMeta(files []*model.ParsedProtoFile) (*model.TagMeta, error) {
	packages := make(map[string]bool)
	imports := make(map[string]bool)
	refPackages := make(map[string]bool)

	filesMeta := make([]*model.FileMeta, len(files))

	for i, file := range files {
		filePackages := make(map[string]bool)
		fileImports := make(map[string]bool)
		fileRefPackages := make(map[string]bool)

		protoBody := file.Proto.ProtoBody

		for _, pkg := range protoBody.Packages {
			packages[pkg.Name] = true
			filePackages[pkg.Name] = true
		}

		for _, imp := range protoBody.Imports {
			location := strings.Trim(imp.Location, "\"")
			imports[location] = true
			fileImports[location] = true
		}

		var retrievedRefPackages []string

		retrievedRefPackages = append(retrievedRefPackages, getRefPackagesFromOptions(protoBody.Options)...)
		retrievedRefPackages = append(retrievedRefPackages, getRefPackagesForMessages(protoBody.Messages)...)
		retrievedRefPackages = append(retrievedRefPackages, getRefPackagesForEnums(protoBody.Enums)...)

		for _, service := range protoBody.Services {
			retrievedRefPackages = append(retrievedRefPackages, getRefPackagesFromOptions(service.ServiceBody.Options)...)
			for _, rpc := range service.ServiceBody.RPCs {
				retrievedRefPackages = append(retrievedRefPackages, getRefPackagesFromOptions(rpc.Options)...)
				retrievedRefPackages = append(retrievedRefPackages, getRefPackageName(rpc.RPCRequest.MessageType))
				retrievedRefPackages = append(retrievedRefPackages, getRefPackageName(rpc.RPCResponse.MessageType))
			}
		}

		for _, refPkg := range retrievedRefPackages {
			if refPkg != "" {
				refPackages[refPkg] = true
				fileRefPackages[refPkg] = true
			}
		}

		uniquePackages := maps.Keys(filePackages)
		sort.Strings(uniquePackages)

		uniqueImports := maps.Keys(fileImports)
		sort.Strings(uniqueImports)

		uniqueRefPackages := maps.Keys(fileRefPackages)
		sort.Strings(uniqueRefPackages)

		filesMeta[i] = &model.FileMeta{
			Filename:    file.Filename,
			Packages:    uniquePackages,
			Imports:     uniqueImports,
			RefPackages: uniqueRefPackages,
		}
	}

	uniquePackages := maps.Keys(packages)
	sort.Strings(uniquePackages)

	uniqueImports := maps.Keys(imports)
	sort.Strings(uniqueImports)

	uniqueRefPackages := maps.Keys(refPackages)
	sort.Strings(uniqueRefPackages)

	return &model.TagMeta{
		Packages:    uniquePackages,
		Imports:     uniqueImports,
		RefPackages: uniqueRefPackages,
		FilesMeta:   filesMeta,
	}, nil
}

func getRefPackagesForMessages(messages []*unordered.Message) []string {
	var names []string
	for _, message := range messages {
		names = append(names, getRefPackagesForMessages(message.MessageBody.Messages)...)

		names = append(names, getRefPackagesFromOptions(message.MessageBody.Options)...)
		names = append(names, getRefPackagesForEnums(message.MessageBody.Enums)...)
		names = append(names, getRefPackagesForFields(message.MessageBody.Fields)...)
		names = append(names, getRefPackagesForOneOfs(message.MessageBody.Oneofs)...)
		names = append(names, getRefPackagesForMaps(message.MessageBody.Maps)...)
	}

	return names
}

func getRefPackagesForMaps(fields []*parser.MapField) []string {
	var names []string
	for _, field := range fields {
		names = append(names, getRefPackagesFromFieldOptions(field.FieldOptions)...)
		names = append(names, getRefPackageName(field.Type))
	}

	return names
}

func getRefPackagesForFields(fields []*parser.Field) []string {
	var names []string
	for _, field := range fields {
		names = append(names, getRefPackagesFromFieldOptions(field.FieldOptions)...)
		names = append(names, getRefPackageName(field.Type))
	}

	return names
}

func getRefPackagesForOneOfs(oneofs []*parser.Oneof) []string {
	var names []string
	for _, oneof := range oneofs {
		names = append(names, getRefPackagesFromOptions(oneof.Options)...)

		for _, field := range oneof.OneofFields {
			names = append(names, getRefPackagesFromFieldOptions(field.FieldOptions)...)
			names = append(names, getRefPackageName(field.Type))
		}
	}

	return names
}

func getRefPackagesForEnums(enums []*unordered.Enum) []string {
	var names []string
	for _, enum := range enums {
		names = append(names, getRefPackagesFromOptions(enum.EnumBody.Options)...)
	}

	return names
}

func getRefPackagesFromOptions(options []*parser.Option) []string {
	names := make([]string, len(options))
	for i, option := range options {
		names[i] = getRefPackageName(option.OptionName)
	}

	return names
}

func getRefPackagesFromFieldOptions(options []*parser.FieldOption) []string {
	var names []string
	for _, option := range options {
		names = append(names, getRefPackageName(option.OptionName))
	}

	return names
}

func getRefPackageName(name string) string {
	if strings.HasPrefix(name, "(") {
		name = strings.Split(name, "(")[1]
		name = strings.Split(name, ")")[0]
	}

	if strings.Contains(name, ".") {
		split := strings.Split(name, ".")
		return strings.TrimSpace(strings.Join(split[:len(split)-1], "."))
	}

	return ""
}
