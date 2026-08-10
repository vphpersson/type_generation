package postgres

import (
	"strings"
	"testing"
)

type sample struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestConvertEmitsCreateTable(t *testing.T) {
	t.Parallel()

	out, err := Convert(sample{})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if !strings.Contains(out, "CREATE TABLE sample") {
		t.Errorf("expected CREATE TABLE for sample, got:\n%s", out)
	}
	if !strings.Contains(out, "Name text") {
		t.Errorf("expected `Name text` column, got:\n%s", out)
	}
	if !strings.Contains(out, "Age integer") {
		t.Errorf("expected `Age integer` column, got:\n%s", out)
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
