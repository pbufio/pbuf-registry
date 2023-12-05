package model

import (
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
)

type ParsedProtoFile struct {
	Filename  string
	Proto     *unordered.Proto
	ProtoJson string
}

type TagMeta struct {
	Packages    []string
	Imports     []string
	RefPackages []string
	FilesMeta   []*FileMeta
}

type FileMeta struct {
	Filename    string
	Packages    []string
	Imports     []string
	RefPackages []string
}
