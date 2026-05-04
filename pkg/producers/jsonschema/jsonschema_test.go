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
	out, err := Convert(reflect.TypeOf(sample{}))
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
