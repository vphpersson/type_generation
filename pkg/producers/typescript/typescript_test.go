package typescript

import (
	"strings"
	"testing"
)

type sample struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestConvertEmitsExpectedInterface(t *testing.T) {
	t.Parallel()

	out, err := Convert(sample{})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	for _, want := range []string{"export interface Sample", "name: string", "age: number"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestConvertNonStructIsNoOp(t *testing.T) {
	t.Parallel()

	// Add() silently filters out non-struct values, so Convert returns
	// empty output without error rather than failing.
	out, err := Convert(42)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output for non-struct, got:\n%s", out)
	}
}

type withUnsupportedField struct {
	Ch chan int `json:"ch"`
}

func TestConvertUnsupportedFieldErrors(t *testing.T) {
	t.Parallel()

	if _, err := Convert(withUnsupportedField{}); err == nil {
		t.Error("expected an error for a chan field")
	}
}
