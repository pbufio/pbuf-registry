package background

import (
	"strings"
	"testing"

	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yoheimuta/go-protoparser/v4"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
)

// parseProtoString parses a proto string into an unordered.Proto for testing
func parseProtoString(t *testing.T, content string) *unordered.Proto {
	t.Helper()
	parsed, err := protoparser.Parse(strings.NewReader(content))
	require.NoError(t, err, "failed to parse proto")
	proto, err := unordered.InterpretProto(parsed)
	require.NoError(t, err, "failed to interpret proto")
	return proto
}

func TestDetermineSeverityFromParsed_NilInputs(t *testing.T) {
	// Test when previous is nil
	t.Run("previous nil", func(t *testing.T) {
		current := parseProtoString(t, `syntax = "proto3"; message Foo {}`)
		severity := determineSeverityFromParsed(nil, current)
		assert.Equal(t, model.DriftSeverityWarning, severity)
	})

	// Test when current is nil
	t.Run("current nil", func(t *testing.T) {
		previous := parseProtoString(t, `syntax = "proto3"; message Foo {}`)
		severity := determineSeverityFromParsed(previous, nil)
		assert.Equal(t, model.DriftSeverityWarning, severity)
	})

	// Test when both are nil
	t.Run("both nil", func(t *testing.T) {
		severity := determineSeverityFromParsed(nil, nil)
		assert.Equal(t, model.DriftSeverityWarning, severity)
	})
}

func TestDetermineSeverityFromParsed_MessageRemoval(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			string name = 1;
		}
		message Bar {
			int32 id = 1;
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			string name = 1;
		}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityCritical, severity, "removing a message should be critical")
}

func TestDetermineSeverityFromParsed_ServiceRemoval(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		message Request {}
		message Response {}
		service MyService {
			rpc DoSomething(Request) returns (Response);
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		message Request {}
		message Response {}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityCritical, severity, "removing a service should be critical")
}

func TestDetermineSeverityFromParsed_EnumRemoval(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		enum Status {
			UNKNOWN = 0;
			ACTIVE = 1;
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityCritical, severity, "removing an enum should be critical")
}

func TestDetermineSeverityFromParsed_FieldRemoval(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			string name = 1;
			int32 age = 2;
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			string name = 1;
		}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityCritical, severity, "removing a field should be critical")
}

func TestDetermineSeverityFromParsed_FieldNumberChange(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			string name = 1;
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			string name = 2;
		}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityCritical, severity, "changing a field number should be critical")
}

func TestDetermineSeverityFromParsed_RPCRemoval(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		message Request {}
		message Response {}
		service MyService {
			rpc Method1(Request) returns (Response);
			rpc Method2(Request) returns (Response);
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		message Request {}
		message Response {}
		service MyService {
			rpc Method1(Request) returns (Response);
		}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityCritical, severity, "removing an RPC should be critical")
}

func TestDetermineSeverityFromParsed_EnumValueRemoval(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		enum Status {
			UNKNOWN = 0;
			ACTIVE = 1;
			INACTIVE = 2;
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		enum Status {
			UNKNOWN = 0;
			ACTIVE = 1;
		}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityCritical, severity, "removing an enum value should be critical")
}

func TestDetermineSeverityFromParsed_NestedMessageRemoval(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		message Outer {
			message Inner {
				string value = 1;
			}
			Inner inner = 1;
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		message Outer {
			string value = 1;
		}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityCritical, severity, "removing a nested message should be critical")
}

func TestDetermineSeverityFromParsed_FieldTypeChange(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			string name = 1;
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			int32 name = 1;
		}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityWarning, severity, "changing a field type should be warning")
}

func TestDetermineSeverityFromParsed_PackageChange(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		package com.example.v1;
		message Foo {}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		package com.example.v2;
		message Foo {}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityWarning, severity, "changing package should be warning")
}

func TestDetermineSeverityFromParsed_AddOptionalField(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			string name = 1;
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			string name = 1;
			int32 age = 2;
		}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityInfo, severity, "adding an optional field should be info")
}

func TestDetermineSeverityFromParsed_AddMessage(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		message Foo {}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		message Foo {}
		message Bar {}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityInfo, severity, "adding a message should be info")
}

func TestDetermineSeverityFromParsed_AddService(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		message Request {}
		message Response {}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		message Request {}
		message Response {}
		service MyService {
			rpc DoSomething(Request) returns (Response);
		}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityInfo, severity, "adding a service should be info")
}

func TestDetermineSeverityFromParsed_AddRPC(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		message Request {}
		message Response {}
		service MyService {
			rpc Method1(Request) returns (Response);
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		message Request {}
		message Response {}
		service MyService {
			rpc Method1(Request) returns (Response);
			rpc Method2(Request) returns (Response);
		}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityInfo, severity, "adding an RPC should be info")
}

func TestDetermineSeverityFromParsed_AddEnumValue(t *testing.T) {
	previous := parseProtoString(t, `
		syntax = "proto3";
		enum Status {
			UNKNOWN = 0;
			ACTIVE = 1;
		}
	`)
	current := parseProtoString(t, `
		syntax = "proto3";
		enum Status {
			UNKNOWN = 0;
			ACTIVE = 1;
			INACTIVE = 2;
		}
	`)

	severity := determineSeverityFromParsed(previous, current)
	assert.Equal(t, model.DriftSeverityInfo, severity, "adding an enum value should be info")
}

func TestHasBreakingChangesFromParsed(t *testing.T) {
	tests := []struct {
		name           string
		previousProto  string
		currentProto   string
		expectBreaking bool
	}{
		{
			name: "message removed",
			previousProto: `
				syntax = "proto3";
				message Foo {}
				message Bar {}
			`,
			currentProto: `
				syntax = "proto3";
				message Foo {}
			`,
			expectBreaking: true,
		},
		{
			name: "service removed",
			previousProto: `
				syntax = "proto3";
				message Request {}
				message Response {}
				service MyService {
					rpc Do(Request) returns (Response);
				}
			`,
			currentProto: `
				syntax = "proto3";
				message Request {}
				message Response {}
			`,
			expectBreaking: true,
		},
		{
			name: "no breaking changes - field added",
			previousProto: `
				syntax = "proto3";
				message Foo {
					string name = 1;
				}
			`,
			currentProto: `
				syntax = "proto3";
				message Foo {
					string name = 1;
					int32 age = 2;
				}
			`,
			expectBreaking: false,
		},
		{
			name: "no breaking changes - same content",
			previousProto: `
				syntax = "proto3";
				message Foo {
					string name = 1;
				}
			`,
			currentProto: `
				syntax = "proto3";
				message Foo {
					string name = 1;
				}
			`,
			expectBreaking: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := parseProtoString(t, tt.previousProto)
			current := parseProtoString(t, tt.currentProto)
			result := hasBreakingChangesFromParsed(previous, current)
			assert.Equal(t, tt.expectBreaking, result)
		})
	}
}

func TestHasPotentiallyBreakingChangesFromParsed(t *testing.T) {
	tests := []struct {
		name                      string
		previousProto             string
		currentProto              string
		expectPotentiallyBreaking bool
	}{
		{
			name: "field type change",
			previousProto: `
				syntax = "proto3";
				message Foo {
					string name = 1;
				}
			`,
			currentProto: `
				syntax = "proto3";
				message Foo {
					int32 name = 1;
				}
			`,
			expectPotentiallyBreaking: true,
		},
		{
			name: "package change",
			previousProto: `
				syntax = "proto3";
				package v1;
				message Foo {}
			`,
			currentProto: `
				syntax = "proto3";
				package v2;
				message Foo {}
			`,
			expectPotentiallyBreaking: true,
		},
		{
			name: "no potentially breaking changes",
			previousProto: `
				syntax = "proto3";
				message Foo {
					string name = 1;
				}
			`,
			currentProto: `
				syntax = "proto3";
				message Foo {
					string name = 1;
					int32 age = 2;
				}
			`,
			expectPotentiallyBreaking: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := parseProtoString(t, tt.previousProto)
			current := parseProtoString(t, tt.currentProto)
			result := hasPotentiallyBreakingChangesFromParsed(previous, current)
			assert.Equal(t, tt.expectPotentiallyBreaking, result)
		})
	}
}

func TestExtractMessagesFromParsed(t *testing.T) {
	proto := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			message Nested {
				string value = 1;
			}
			string name = 1;
		}
		message Bar {}
	`)

	messages := extractMessagesFromParsed(proto.ProtoBody.Messages)

	assert.True(t, messages["Foo"], "should contain Foo")
	assert.True(t, messages["Bar"], "should contain Bar")
	assert.True(t, messages["Foo.Nested"], "should contain nested message Foo.Nested")
}

func TestExtractServicesFromParsed(t *testing.T) {
	proto := parseProtoString(t, `
		syntax = "proto3";
		message Request {}
		message Response {}
		service ServiceA {
			rpc Do(Request) returns (Response);
		}
		service ServiceB {
			rpc Do(Request) returns (Response);
		}
	`)

	services := extractServicesFromParsed(proto.ProtoBody.Services)

	assert.True(t, services["ServiceA"], "should contain ServiceA")
	assert.True(t, services["ServiceB"], "should contain ServiceB")
	assert.Equal(t, 2, len(services))
}

func TestExtractEnumsFromParsed(t *testing.T) {
	proto := parseProtoString(t, `
		syntax = "proto3";
		enum Status {
			UNKNOWN = 0;
		}
		enum Type {
			DEFAULT = 0;
		}
	`)

	enums := extractEnumsFromParsed(proto.ProtoBody.Enums)

	assert.True(t, enums["Status"], "should contain Status")
	assert.True(t, enums["Type"], "should contain Type")
	assert.Equal(t, 2, len(enums))
}

func TestExtractFieldsFromMessages(t *testing.T) {
	proto := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			string name = 1;
			int32 age = 2;
		}
	`)

	fields := extractFieldsFromMessages(proto.ProtoBody.Messages)

	assert.Equal(t, "1", fields["Foo.name"], "should have correct field number for name")
	assert.Equal(t, "2", fields["Foo.age"], "should have correct field number for age")
}

func TestExtractRPCsFromParsed(t *testing.T) {
	proto := parseProtoString(t, `
		syntax = "proto3";
		message Request {}
		message Response {}
		service MyService {
			rpc Method1(Request) returns (Response);
			rpc Method2(Request) returns (Response);
		}
	`)

	rpcs := extractRPCsFromParsed(proto.ProtoBody.Services)

	assert.True(t, rpcs["MyService.Method1"], "should contain MyService.Method1")
	assert.True(t, rpcs["MyService.Method2"], "should contain MyService.Method2")
	assert.Equal(t, 2, len(rpcs))
}

func TestExtractEnumValuesFromParsed(t *testing.T) {
	proto := parseProtoString(t, `
		syntax = "proto3";
		enum Status {
			UNKNOWN = 0;
			ACTIVE = 1;
			INACTIVE = 2;
		}
	`)

	values := extractEnumValuesFromParsed(proto.ProtoBody.Enums)

	assert.True(t, values["Status.UNKNOWN"], "should contain Status.UNKNOWN")
	assert.True(t, values["Status.ACTIVE"], "should contain Status.ACTIVE")
	assert.True(t, values["Status.INACTIVE"], "should contain Status.INACTIVE")
	assert.Equal(t, 3, len(values))
}

func TestExtractPackageFromParsed(t *testing.T) {
	proto := parseProtoString(t, `
		syntax = "proto3";
		package com.example.v1;
		message Foo {}
	`)

	pkg := extractPackageFromParsed(proto.ProtoBody.Packages)
	assert.Equal(t, "com.example.v1", pkg)
}

func TestExtractFieldTypesFromMessages(t *testing.T) {
	proto := parseProtoString(t, `
		syntax = "proto3";
		message Foo {
			string name = 1;
			int32 age = 2;
			bool active = 3;
		}
	`)

	types := extractFieldTypesFromMessages(proto.ProtoBody.Messages)

	assert.Equal(t, "string", types["Foo.name"], "should have correct type for name")
	assert.Equal(t, "int32", types["Foo.age"], "should have correct type for age")
	assert.Equal(t, "bool", types["Foo.active"], "should have correct type for active")
}
