package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"
	pb "github.com/qdrant/go-client/qdrant"
)

// HashString creates a deterministic UUID v5 from a string
func HashString(input string) string {
	// Use a fixed namespace UUID (this is arbitrary but must be consistent)
	// Using a UUID based on the name "ObsFind" to keep it domain-specific
	namespaceUUID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // UUID namespace for DNS

	// Generate a UUID from the namespace and the input string
	// This ensures deterministic UUID generation based on the input
	return uuid.NewSHA1(namespaceUUID, []byte(input)).String()
}

// StructToPayload converts a struct or map to a Qdrant payload
func StructToPayload(input interface{}) map[string]*pb.Value {
	payload := make(map[string]*pb.Value)

	// If it's already a map, process it directly
	if m, ok := input.(map[string]interface{}); ok {
		for k, v := range m {
			payload[k] = toValue(v)
		}
		return payload
	}

	// Otherwise, convert struct to map first
	data, err := json.Marshal(input)
	if err != nil {
		return payload
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return payload
	}

	for k, v := range m {
		payload[k] = toValue(v)
	}

	return payload
}

// toValue converts a Go value to a Qdrant Value
func toValue(v interface{}) *pb.Value {
	if v == nil {
		return &pb.Value{
			Kind: &pb.Value_NullValue{
				NullValue: pb.NullValue_NULL_VALUE,
			},
		}
	}

	// Use reflection to handle different types
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.String:
		return &pb.Value{
			Kind: &pb.Value_StringValue{
				StringValue: val.String(),
			},
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &pb.Value{
			Kind: &pb.Value_IntegerValue{
				IntegerValue: val.Int(),
			},
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &pb.Value{
			Kind: &pb.Value_IntegerValue{
				IntegerValue: int64(val.Uint()),
			},
		}
	case reflect.Float32, reflect.Float64:
		return &pb.Value{
			Kind: &pb.Value_DoubleValue{
				DoubleValue: val.Float(),
			},
		}
	case reflect.Bool:
		return &pb.Value{
			Kind: &pb.Value_BoolValue{
				BoolValue: val.Bool(),
			},
		}
	case reflect.Slice, reflect.Array:
		// Handle arrays and slices
		values := make([]*pb.Value, val.Len())
		for i := 0; i < val.Len(); i++ {
			values[i] = toValue(val.Index(i).Interface())
		}
		return &pb.Value{
			Kind: &pb.Value_ListValue{
				ListValue: &pb.ListValue{
					Values: values,
				},
			},
		}
	case reflect.Map:
		// Handle maps
		struct_value := &pb.Value{
			Kind: &pb.Value_StructValue{
				StructValue: &pb.Struct{
					Fields: make(map[string]*pb.Value),
				},
			},
		}

		iter := val.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()

			// Only process string keys
			if k.Kind() == reflect.String {
				struct_value.GetStructValue().Fields[k.String()] = toValue(v.Interface())
			}
		}
		return struct_value
	default:
		// Try to convert to string for unknown types
		return &pb.Value{
			Kind: &pb.Value_StringValue{
				StringValue: strings.TrimSpace(fmt.Sprintf("%v", v)),
			},
		}
	}
}
