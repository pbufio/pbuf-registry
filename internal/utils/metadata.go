package utils

import (
	"sort"
	"strconv"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
	"github.com/yoheimuta/go-protoparser/v4/parser"
	"golang.org/x/exp/maps"
)

// ToMetadata converts array of ParsedProtoFile to []v1.Packages
func ToMetadata(files []*model.ParsedProtoFile) []*v1.Package {
	packagesMap := make(map[string]*v1.Package)

	for _, file := range files {
		protoBody := file.Proto.ProtoBody
		for _, pkg := range protoBody.Packages {
			if _, ok := packagesMap[pkg.Name]; !ok {
				packagesMap[pkg.Name] = &v1.Package{
					Name: pkg.Name,
				}
			}

			packagesMap[pkg.Name].ProtoFiles = append(packagesMap[pkg.Name].ProtoFiles, &v1.ParsedProtoFile{
				Filename: file.Filename,
				Messages: retrieveMessages(protoBody.Messages),
				Services: retrieveServices(protoBody.Services),
			})
		}
	}

	packagesNames := maps.Keys(packagesMap)
	sort.Strings(packagesNames)

	var packages []*v1.Package
	for _, packageName := range packagesNames {
		packages = append(packages, packagesMap[packageName])
	}

	return packages
}

func retrieveMessages(messages []*unordered.Message) []*v1.Message {
	var result []*v1.Message
	for _, message := range messages {
		fields := retrieveFields(message.MessageBody.Fields)
		fields = append(fields, retrieveOneOfFields(message.MessageBody.Oneofs)...)
		fields = append(fields, retrieveMapFields(message.MessageBody.Maps)...)
		result = append(result, &v1.Message{
			Name:           message.MessageName,
			Fields:         fields,
			NestedMessages: retrieveMessages(message.MessageBody.Messages),
			NestedEnums:    retrieveEnums(message.MessageBody.Enums),
		})
	}
	return result
}

func retrieveFields(fields []*parser.Field) []*v1.Field {
	var result []*v1.Field
	for _, field := range fields {
		fieldNumber, err := strconv.Atoi(field.FieldNumber)
		if err != nil {
			fieldNumber = -1
		}

		result = append(result, &v1.Field{
			Name:        field.FieldName,
			MessageType: field.Type,
			Tag:         int32(fieldNumber),
			Repeated:    field.IsRepeated,
			Map:         false,
			Oneof:       false,
			Required:    field.IsRequired,
			Optional:    field.IsOptional,
		})
	}
	return result
}

func retrieveOneOfFields(oneofs []*parser.Oneof) []*v1.Field {
	var result []*v1.Field
	for _, oneof := range oneofs {
		oneOfNames := make([]string, len(oneof.OneofFields))
		oneOfTypes := make([]string, len(oneof.OneofFields))
		for i, field := range oneof.OneofFields {
			oneOfNames[i] = field.FieldName
			oneOfTypes[i] = field.Type
		}

		result = append(result, &v1.Field{
			Name:       oneof.OneofName,
			Oneof:      true,
			OneofNames: oneOfNames,
			OneofTypes: oneOfTypes,
		})
	}

	return result
}

func retrieveMapFields(fields []*parser.MapField) []*v1.Field {
	var result []*v1.Field
	for _, field := range fields {
		fieldNumber, err := strconv.Atoi(field.FieldNumber)
		if err != nil {
			fieldNumber = -1
		}

		result = append(result, &v1.Field{
			Name:         field.MapName,
			Tag:          int32(fieldNumber),
			Map:          true,
			MapKeyType:   field.KeyType,
			MapValueType: field.Type,
		})
	}
	return result
}

func retrieveEnums(enums []*unordered.Enum) []*v1.Enum {
	var result []*v1.Enum
	for _, enum := range enums {
		result = append(result, &v1.Enum{
			Name:   enum.EnumName,
			Values: retrieveEnumValues(enum.EnumBody.EnumFields),
		})
	}
	return result
}

func retrieveEnumValues(fields []*parser.EnumField) []*v1.EnumValue {
	var result []*v1.EnumValue
	for _, field := range fields {
		fieldNumber, err := strconv.Atoi(field.Number)
		if err != nil {
			fieldNumber = -1
		}

		result = append(result, &v1.EnumValue{
			Name: field.Ident,
			Tag:  int32(fieldNumber),
		})
	}
	return result
}

func retrieveServices(services []*unordered.Service) []*v1.Service {
	var result []*v1.Service
	for _, service := range services {
		result = append(result, &v1.Service{
			Name:    service.ServiceName,
			Methods: retrieveMethods(service.ServiceBody.RPCs),
		})
	}
	return result
}

func retrieveMethods(cs []*parser.RPC) []*v1.Method {
	var result []*v1.Method
	for _, c := range cs {
		result = append(result, &v1.Method{
			Name:       c.RPCName,
			InputType:  c.RPCRequest.MessageType,
			OutputType: c.RPCResponse.MessageType,
		})
	}
	return result
}
