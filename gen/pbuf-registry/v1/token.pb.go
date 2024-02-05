// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        (unknown)
// source: pbuf-registry/v1/token.proto

package v1

import (
	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// RegisterTokenRequest is the request message for RegisterToken.
type RegisterTokenRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The name of token to be displayed.
	DisplayName string `protobuf:"bytes,1,opt,name=display_name,json=displayName,proto3" json:"display_name,omitempty"`
	// The token access role
	Role string `protobuf:"bytes,2,opt,name=role,proto3" json:"role,omitempty"`
}

func (x *RegisterTokenRequest) Reset() {
	*x = RegisterTokenRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbuf_registry_v1_token_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RegisterTokenRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegisterTokenRequest) ProtoMessage() {}

func (x *RegisterTokenRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pbuf_registry_v1_token_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RegisterTokenRequest.ProtoReflect.Descriptor instead.
func (*RegisterTokenRequest) Descriptor() ([]byte, []int) {
	return file_pbuf_registry_v1_token_proto_rawDescGZIP(), []int{0}
}

func (x *RegisterTokenRequest) GetDisplayName() string {
	if x != nil {
		return x.DisplayName
	}
	return ""
}

func (x *RegisterTokenRequest) GetRole() string {
	if x != nil {
		return x.Role
	}
	return ""
}

// RegisterTokenResponse is the response message for RegisterToken.
type RegisterTokenResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The authorization token.
	Token string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
}

func (x *RegisterTokenResponse) Reset() {
	*x = RegisterTokenResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbuf_registry_v1_token_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RegisterTokenResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegisterTokenResponse) ProtoMessage() {}

func (x *RegisterTokenResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pbuf_registry_v1_token_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RegisterTokenResponse.ProtoReflect.Descriptor instead.
func (*RegisterTokenResponse) Descriptor() ([]byte, []int) {
	return file_pbuf_registry_v1_token_proto_rawDescGZIP(), []int{1}
}

func (x *RegisterTokenResponse) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

// RegisterTokenRequest is the request message for RevokeToken.
type RevokeTokenRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The authorization token.
	Token string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
}

func (x *RevokeTokenRequest) Reset() {
	*x = RevokeTokenRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbuf_registry_v1_token_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RevokeTokenRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RevokeTokenRequest) ProtoMessage() {}

func (x *RevokeTokenRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pbuf_registry_v1_token_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RevokeTokenRequest.ProtoReflect.Descriptor instead.
func (*RevokeTokenRequest) Descriptor() ([]byte, []int) {
	return file_pbuf_registry_v1_token_proto_rawDescGZIP(), []int{2}
}

func (x *RevokeTokenRequest) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

// RevokeTokenResponse is the response message for RevokeToken.
type RevokeTokenResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The authorization token.
	Result string `protobuf:"bytes,1,opt,name=result,proto3" json:"result,omitempty"`
}

func (x *RevokeTokenResponse) Reset() {
	*x = RevokeTokenResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbuf_registry_v1_token_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RevokeTokenResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RevokeTokenResponse) ProtoMessage() {}

func (x *RevokeTokenResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pbuf_registry_v1_token_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RevokeTokenResponse.ProtoReflect.Descriptor instead.
func (*RevokeTokenResponse) Descriptor() ([]byte, []int) {
	return file_pbuf_registry_v1_token_proto_rawDescGZIP(), []int{3}
}

func (x *RevokeTokenResponse) GetResult() string {
	if x != nil {
		return x.Result
	}
	return ""
}

var File_pbuf_registry_v1_token_proto protoreflect.FileDescriptor

var file_pbuf_registry_v1_token_proto_rawDesc = []byte{
	0x0a, 0x1c, 0x70, 0x62, 0x75, 0x66, 0x2d, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x2f,
	0x76, 0x31, 0x2f, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0f,
	0x70, 0x62, 0x75, 0x66, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x1a,
	0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e, 0x6e, 0x6f,
	0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1f, 0x70,
	0x62, 0x75, 0x66, 0x2d, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x2f, 0x76, 0x31, 0x2f,
	0x65, 0x6e, 0x74, 0x69, 0x74, 0x69, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x4d,
	0x0a, 0x14, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x21, 0x0a, 0x0c, 0x64, 0x69, 0x73, 0x70, 0x6c, 0x61,
	0x79, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x69,
	0x73, 0x70, 0x6c, 0x61, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x72, 0x6f, 0x6c,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x72, 0x6f, 0x6c, 0x65, 0x22, 0x2d, 0x0a,
	0x15, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0x2a, 0x0a, 0x12,
	0x52, 0x65, 0x76, 0x6f, 0x6b, 0x65, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0x2d, 0x0a, 0x13, 0x52, 0x65, 0x76, 0x6f,
	0x6b, 0x65, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x16, 0x0a, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x32, 0x86, 0x02, 0x0a, 0x0c, 0x54, 0x6f, 0x6b, 0x65,
	0x6e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x7e, 0x0a, 0x0d, 0x52, 0x65, 0x67, 0x69,
	0x73, 0x74, 0x65, 0x72, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x12, 0x25, 0x2e, 0x70, 0x62, 0x75, 0x66,
	0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x2e, 0x52, 0x65, 0x67, 0x69,
	0x73, 0x74, 0x65, 0x72, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x26, 0x2e, 0x70, 0x62, 0x75, 0x66, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x2e,
	0x76, 0x31, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x54, 0x6f, 0x6b, 0x65, 0x6e,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x1e, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x18,
	0x3a, 0x01, 0x2a, 0x22, 0x13, 0x2f, 0x76, 0x31, 0x2f, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x73, 0x2f,
	0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x12, 0x76, 0x0a, 0x0b, 0x52, 0x65, 0x76, 0x6f,
	0x6b, 0x65, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x12, 0x23, 0x2e, 0x70, 0x62, 0x75, 0x66, 0x72, 0x65,
	0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x2e, 0x52, 0x65, 0x76, 0x6f, 0x6b, 0x65,
	0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x24, 0x2e, 0x70,
	0x62, 0x75, 0x66, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x2e, 0x52,
	0x65, 0x76, 0x6f, 0x6b, 0x65, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x22, 0x1c, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x16, 0x3a, 0x01, 0x2a, 0x22, 0x11, 0x2f,
	0x76, 0x31, 0x2f, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x73, 0x2f, 0x72, 0x65, 0x76, 0x6f, 0x6b, 0x65,
	0x42, 0x18, 0x5a, 0x16, 0x70, 0x62, 0x75, 0x66, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79,
	0x2f, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x31, 0x3b, 0x76, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_pbuf_registry_v1_token_proto_rawDescOnce sync.Once
	file_pbuf_registry_v1_token_proto_rawDescData = file_pbuf_registry_v1_token_proto_rawDesc
)

func file_pbuf_registry_v1_token_proto_rawDescGZIP() []byte {
	file_pbuf_registry_v1_token_proto_rawDescOnce.Do(func() {
		file_pbuf_registry_v1_token_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbuf_registry_v1_token_proto_rawDescData)
	})
	return file_pbuf_registry_v1_token_proto_rawDescData
}

var file_pbuf_registry_v1_token_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_pbuf_registry_v1_token_proto_goTypes = []interface{}{
	(*RegisterTokenRequest)(nil),  // 0: pbufregistry.v1.RegisterTokenRequest
	(*RegisterTokenResponse)(nil), // 1: pbufregistry.v1.RegisterTokenResponse
	(*RevokeTokenRequest)(nil),    // 2: pbufregistry.v1.RevokeTokenRequest
	(*RevokeTokenResponse)(nil),   // 3: pbufregistry.v1.RevokeTokenResponse
}
var file_pbuf_registry_v1_token_proto_depIdxs = []int32{
	0, // 0: pbufregistry.v1.TokenService.RegisterToken:input_type -> pbufregistry.v1.RegisterTokenRequest
	2, // 1: pbufregistry.v1.TokenService.RevokeToken:input_type -> pbufregistry.v1.RevokeTokenRequest
	1, // 2: pbufregistry.v1.TokenService.RegisterToken:output_type -> pbufregistry.v1.RegisterTokenResponse
	3, // 3: pbufregistry.v1.TokenService.RevokeToken:output_type -> pbufregistry.v1.RevokeTokenResponse
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_pbuf_registry_v1_token_proto_init() }
func file_pbuf_registry_v1_token_proto_init() {
	if File_pbuf_registry_v1_token_proto != nil {
		return
	}
	file_pbuf_registry_v1_entities_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_pbuf_registry_v1_token_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RegisterTokenRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pbuf_registry_v1_token_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RegisterTokenResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pbuf_registry_v1_token_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RevokeTokenRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pbuf_registry_v1_token_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RevokeTokenResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pbuf_registry_v1_token_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_pbuf_registry_v1_token_proto_goTypes,
		DependencyIndexes: file_pbuf_registry_v1_token_proto_depIdxs,
		MessageInfos:      file_pbuf_registry_v1_token_proto_msgTypes,
	}.Build()
	File_pbuf_registry_v1_token_proto = out.File
	file_pbuf_registry_v1_token_proto_rawDesc = nil
	file_pbuf_registry_v1_token_proto_goTypes = nil
	file_pbuf_registry_v1_token_proto_depIdxs = nil
}
