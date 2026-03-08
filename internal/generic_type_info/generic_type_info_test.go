package generic_type_info

import (
	"reflect"
	"testing"

	"github.com/vphpersson/type_generation/pkg/types/shape"
)

func TestParseTypeArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{name: "no generics", input: "MyType", expected: nil},
		{name: "single arg", input: "MyType[int]", expected: []string{"int"}},
		{name: "two args", input: "MyType[int,string]", expected: []string{"int", "string"}},
		{name: "two args with spaces", input: "MyType[int, string]", expected: []string{"int", "string"}},
		{name: "nested generic arg", input: "MyType[pkg.Bar[int]]", expected: []string{"pkg.Bar[int]"}},
		{name: "full pkg path", input: "MyType[github.com/user/pkg.SomeType]", expected: []string{"github.com/user/pkg.SomeType"}},
		{name: "two full pkg paths", input: "MyType[github.com/a.X,github.com/b.Y]", expected: []string{"github.com/a.X", "github.com/b.Y"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTypeArgs(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d args, got %d: %v", len(tt.expected), len(result), result)
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("arg %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

type Inner struct {
	Value string
}

type GenericDirect[T any] struct {
	Data  T
	Other string
}

type GenericPointer[T any] struct {
	Data  *T
	Other string
}

type GenericSlice[T any] struct {
	Items []T
	Other string
}

type GenericMapValue[T any] struct {
	Mapping map[string]T
	Other   string
}

type GenericTwoParams[T any, U any] struct {
	First  T
	Second []U
}

func TestDiscoverUsingReflection(t *testing.T) {
	t.Run("direct", func(t *testing.T) {
		rt := reflect.TypeOf(GenericDirect[int]{})
		info, err := discoverUsingReflection(rt)
		if err != nil {
			t.Fatal(err)
		}
		if info == nil {
			t.Fatal("expected non-nil info")
		}
		if len(info.TypeParameterNames) != 1 {
			t.Fatalf("expected 1 type param, got %d", len(info.TypeParameterNames))
		}
		s, ok := info.FieldNameToShape["Data"]
		if !ok {
			t.Fatal("expected Data in FieldNameToShape")
		}
		if s.Kind != shape.KindDirect {
			t.Errorf("expected KindDirect, got %v", s.Kind)
		}
		if _, ok := info.FieldNameToShape["Other"]; ok {
			t.Error("Other should not be in FieldNameToShape")
		}
	})

	t.Run("pointer", func(t *testing.T) {
		rt := reflect.TypeOf(GenericPointer[Inner]{})
		info, err := discoverUsingReflection(rt)
		if err != nil {
			t.Fatal(err)
		}
		if info == nil {
			t.Fatal("expected non-nil info")
		}
		s, ok := info.FieldNameToShape["Data"]
		if !ok {
			t.Fatal("expected Data in FieldNameToShape")
		}
		if s.Kind != shape.KindPointer {
			t.Errorf("expected KindPointer, got %v", s.Kind)
		}
	})

	t.Run("slice", func(t *testing.T) {
		rt := reflect.TypeOf(GenericSlice[string]{})
		info, err := discoverUsingReflection(rt)
		if err != nil {
			t.Fatal(err)
		}
		if info == nil {
			t.Fatal("expected non-nil info")
		}
		s, ok := info.FieldNameToShape["Items"]
		if !ok {
			t.Fatal("expected Items in FieldNameToShape")
		}
		if s.Kind != shape.KindSlice {
			t.Errorf("expected KindSlice, got %v", s.Kind)
		}
	})

	t.Run("map value", func(t *testing.T) {
		rt := reflect.TypeOf(GenericMapValue[int]{})
		info, err := discoverUsingReflection(rt)
		if err != nil {
			t.Fatal(err)
		}
		if info == nil {
			t.Fatal("expected non-nil info")
		}
		s, ok := info.FieldNameToShape["Mapping"]
		if !ok {
			t.Fatal("expected Mapping in FieldNameToShape")
		}
		if s.Kind != shape.KindMapValue {
			t.Errorf("expected KindMapValue, got %v", s.Kind)
		}
	})

	t.Run("two params", func(t *testing.T) {
		rt := reflect.TypeOf(GenericTwoParams[int, Inner]{})
		info, err := discoverUsingReflection(rt)
		if err != nil {
			t.Fatal(err)
		}
		if info == nil {
			t.Fatal("expected non-nil info")
		}
		if len(info.TypeParameterNames) != 2 {
			t.Fatalf("expected 2 type params, got %d", len(info.TypeParameterNames))
		}

		s1, ok := info.FieldNameToShape["First"]
		if !ok {
			t.Fatal("expected First in FieldNameToShape")
		}
		if s1.Kind != shape.KindDirect {
			t.Errorf("expected KindDirect for First, got %v", s1.Kind)
		}

		s2, ok := info.FieldNameToShape["Second"]
		if !ok {
			t.Fatal("expected Second in FieldNameToShape")
		}
		if s2.Kind != shape.KindSlice {
			t.Errorf("expected KindSlice for Second, got %v", s2.Kind)
		}

		// Params should map to different fields
		if info.TypeParameterNameToFieldName[s1.Param] != "First" {
			t.Errorf("expected param %s to map to First", s1.Param)
		}
		if info.TypeParameterNameToFieldName[s2.Param] != "Second" {
			t.Errorf("expected param %s to map to Second", s2.Param)
		}
	})

	t.Run("non-generic returns nil", func(t *testing.T) {
		rt := reflect.TypeOf(Inner{})
		info, err := discoverUsingReflection(rt)
		if err != nil {
			t.Fatal(err)
		}
		if info != nil {
			t.Error("expected nil info for non-generic type")
		}
	})
}
