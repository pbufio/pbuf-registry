package utils

import (
	"slices"
	"strconv"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
	"github.com/yoheimuta/go-protoparser/v4/parser"
)

// ToMetadata converts array of ParsedProtoFile to []v1.Packages
func ToMetadata(files []*model.ParsedProtoFile) []*v1.Package {
	packagesMap := make(map[string]*v1.Package)

	for _, file := range files {
		if file == nil || file.Proto == nil || file.Proto.ProtoBody == nil {
			continue
		}
		protoBody := file.Proto.ProtoBody
		pkgNames := make([]string, 0, max(1, len(protoBody.Packages)))
		for _, pkg := range protoBody.Packages {
			if pkg != nil && !slices.Contains(pkgNames, pkg.Name) {
				pkgNames = append(pkgNames, pkg.Name)
			}
		}
		if len(pkgNames) == 0 {
			pkgNames = append(pkgNames, "")
		}

		parsedFile := &v1.ParsedProtoFile{
			Filename: file.Filename,
			Messages: retrieveMessages(protoBody.Messages),
			Services: retrieveServices(protoBody.Services),
			Enums:    retrieveEnums(protoBody.Enums),
		}

		for _, pkgName := range pkgNames {
			if _, ok := packagesMap[pkgName]; !ok {
				packagesMap[pkgName] = &v1.Package{
					Name: pkgName,
				}
			}
			packagesMap[pkgName].ProtoFiles = append(packagesMap[pkgName].ProtoFiles, parsedFile)
		}
	}

	packagesNames := sortedKeys(packagesMap)

	var packages []*v1.Package
	for _, packageName := range packagesNames {
		packages = append(packages, packagesMap[packageName])
	}

	return packages
}

func parseProtoInt32(raw string) int32 {
	n, err := strconv.ParseInt(raw, 0, 32)
	if err != nil {
		return -1
	}
	return int32(n)
}

func retrieveMessages(messages []*unordered.Message) []*v1.Message {
	var result []*v1.Message
	for _, message := range messages {
		if message == nil {
			continue
		}
		msg := &v1.Message{Name: message.MessageName}
		if message.MessageBody != nil {
			fields := retrieveFields(message.MessageBody.Fields)
			fields = append(fields, retrieveOneOfFields(message.MessageBody.Oneofs)...)
			fields = append(fields, retrieveMapFields(message.MessageBody.Maps)...)
			msg.Fields = fields
			msg.NestedMessages = retrieveMessages(message.MessageBody.Messages)
			msg.NestedEnums = retrieveEnums(message.MessageBody.Enums)
		}
		result = append(result, msg)
	}
	return result
}

func retrieveFields(fields []*parser.Field) []*v1.Field {
	var result []*v1.Field
	for _, field := range fields {
		if field == nil {
			continue
		}
		result = append(result, &v1.Field{
			Name:        field.FieldName,
			MessageType: field.Type,
			Tag:         parseProtoInt32(field.FieldNumber),
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
		if oneof == nil {
			continue
		}
		oneOfNames := make([]string, 0, len(oneof.OneofFields))
		oneOfTypes := make([]string, 0, len(oneof.OneofFields))
		for _, field := range oneof.OneofFields {
			if field == nil {
				continue
			}
			oneOfNames = append(oneOfNames, field.FieldName)
			oneOfTypes = append(oneOfTypes, field.Type)
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
		if field == nil {
			continue
		}
		result = append(result, &v1.Field{
			Name:         field.MapName,
			Tag:          parseProtoInt32(field.FieldNumber),
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
		if enum == nil {
			continue
		}
		var values []*v1.EnumValue
		if enum.EnumBody != nil {
			values = retrieveEnumValues(enum.EnumBody.EnumFields)
		}
		result = append(result, &v1.Enum{
			Name:   enum.EnumName,
			Values: values,
		})
	}
	return result
}

func retrieveEnumValues(fields []*parser.EnumField) []*v1.EnumValue {
	var result []*v1.EnumValue
	for _, field := range fields {
		if field == nil {
			continue
		}
		result = append(result, &v1.EnumValue{
			Name: field.Ident,
			Tag:  parseProtoInt32(field.Number),
		})
	}
	return result
}

func retrieveServices(services []*unordered.Service) []*v1.Service {
	var result []*v1.Service
	for _, service := range services {
		if service == nil {
			continue
		}
		var methods []*v1.Method
		if service.ServiceBody != nil {
			methods = retrieveMethods(service.ServiceBody.RPCs)
		}
		result = append(result, &v1.Service{
			Name:    service.ServiceName,
			Methods: methods,
		})
	}
	return result
}

func retrieveMethods(cs []*parser.RPC) []*v1.Method {
	var result []*v1.Method
	for _, c := range cs {
		if c == nil {
			continue
		}
		var inType, outType string
		if c.RPCRequest != nil {
			inType = c.RPCRequest.MessageType
		}
		if c.RPCResponse != nil {
			outType = c.RPCResponse.MessageType
		}
		result = append(result, &v1.Method{
			Name:       c.RPCName,
			InputType:  inType,
			OutputType: outType,
		})
	}
	return result
}
