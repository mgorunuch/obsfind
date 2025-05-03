package model

import (
	"context"

	pb "github.com/qdrant/go-client/qdrant"
)

// QdrantClient interface defines the operations needed for the Qdrant vector database
type QdrantClient interface {
	// Collection management
	CollectionExists(ctx context.Context, name string) (bool, error)
	CreateCollection(ctx context.Context, name string, dimensions uint64, distance pb.Distance) error
	GetCollectionInfo(ctx context.Context, name string) (*pb.CollectionInfo, error)
	DeleteCollection(ctx context.Context, name string) error

	// Point operations
	UpsertPoints(ctx context.Context, collectionName string, points []*pb.PointStruct) error
	DeletePoints(ctx context.Context, collectionName string, ids []string) error
	GetPointsByPath(ctx context.Context, collectionName string, path string) ([]*pb.RetrievedPoint, error)

	// Search operations
	Search(
		ctx context.Context,
		collectionName string,
		vector []float32,
		limit uint64,
		offset uint64,
		filter *pb.Filter,
		params *pb.SearchParams,
	) ([]*pb.ScoredPoint, error)

	// Index operations
	CreatePayloadIndex(
		ctx context.Context,
		collectionName string,
		fieldName string,
		fieldType int,
	) error
}

// GetPayloadString extracts a string value from a Qdrant payload field
func GetPayloadString(payload map[string]*pb.Value, key string) (string, bool) {
	if val, ok := payload[key]; ok {
		if strVal, ok := val.GetKind().(*pb.Value_StringValue); ok {
			return strVal.StringValue, true
		}
	}
	return "", false
}

// GetPayloadInt extracts an integer value from a Qdrant payload field
func GetPayloadInt(payload map[string]*pb.Value, key string) (int, bool) {
	if val, ok := payload[key]; ok {
		if intVal, ok := val.GetKind().(*pb.Value_IntegerValue); ok {
			return int(intVal.IntegerValue), true
		}
	}
	return 0, false
}

// GetPayloadFloat extracts a float value from a Qdrant payload field
func GetPayloadFloat(payload map[string]*pb.Value, key string) (float64, bool) {
	if val, ok := payload[key]; ok {
		if floatVal, ok := val.GetKind().(*pb.Value_DoubleValue); ok {
			return floatVal.DoubleValue, true
		}
	}
	return 0, false
}

// GetPayloadBool extracts a boolean value from a Qdrant payload field
func GetPayloadBool(payload map[string]*pb.Value, key string) (bool, bool) {
	if val, ok := payload[key]; ok {
		if boolVal, ok := val.GetKind().(*pb.Value_BoolValue); ok {
			return boolVal.BoolValue, true
		}
	}
	return false, false
}

// GetPayloadStringSlice extracts a string slice from a Qdrant payload field
func GetPayloadStringSlice(payload map[string]*pb.Value, key string) ([]string, bool) {
	if val, ok := payload[key]; ok {
		if listVal, ok := val.GetKind().(*pb.Value_ListValue); ok {
			result := make([]string, 0, len(listVal.ListValue.Values))
			for _, v := range listVal.ListValue.Values {
				if strVal, ok := v.GetKind().(*pb.Value_StringValue); ok {
					result = append(result, strVal.StringValue)
				}
			}
			return result, true
		}
	}
	return nil, false
}

// GetPayloadIntSlice extracts an integer slice from a Qdrant payload field
func GetPayloadIntSlice(payload map[string]*pb.Value, key string) ([]int, bool) {
	if val, ok := payload[key]; ok {
		if listVal, ok := val.GetKind().(*pb.Value_ListValue); ok {
			result := make([]int, 0, len(listVal.ListValue.Values))
			for _, v := range listVal.ListValue.Values {
				if intVal, ok := v.GetKind().(*pb.Value_IntegerValue); ok {
					result = append(result, int(intVal.IntegerValue))
				}
			}
			return result, true
		}
	}
	return nil, false
}

// GetPayloadFloatSlice extracts a float slice from a Qdrant payload field
func GetPayloadFloatSlice(payload map[string]*pb.Value, key string) ([]float64, bool) {
	if val, ok := payload[key]; ok {
		if listVal, ok := val.GetKind().(*pb.Value_ListValue); ok {
			result := make([]float64, 0, len(listVal.ListValue.Values))
			for _, v := range listVal.ListValue.Values {
				if floatVal, ok := v.GetKind().(*pb.Value_DoubleValue); ok {
					result = append(result, floatVal.DoubleValue)
				}
			}
			return result, true
		}
	}
	return nil, false
}
