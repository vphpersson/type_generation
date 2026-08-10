package jsonschema

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type sample struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestConvertEmitsValidJSON(t *testing.T) {
	t.Parallel()

	out, err := Convert(reflect.TypeFor[sample]())
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal([]byte(out), &schema); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}

	if !strings.Contains(out, "$ref") {
		t.Errorf("expected $ref to point at the root struct, got:\n%s", out)
	}
	if !strings.Contains(out, "$defs") {
		t.Errorf("expected $defs section, got:\n%s", out)
	}
}

func TestConvertUnsupportedRootErrors(t *testing.T) {
	t.Parallel()

	if _, err := Convert(reflect.TypeFor[int]()); err == nil {
		t.Error("expected an error for a non-struct root")
	}
}
