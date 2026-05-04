package types

import (
	"reflect"
	"strings"
	"testing"
	"time"

	typeGenerationContext "github.com/vphpersson/type_generation/pkg/types/context"
)

type pgPrimitives struct {
	S  string  `json:"s"`
	I  int     `json:"i"`
	I8 int8    `json:"i8"`
	I64 int64  `json:"i64"`
	F  float64 `json:"f"`
	B  bool    `json:"b"`
}

type pgWithTime struct {
	When time.Time `json:"when"`
}

type pgWithBytes struct {
	Blob []byte `json:"blob"`
}

type pgWithStringSlice struct {
	Tags []string `json:"tags"`
}

type pgInner struct {
	Name string `json:"name"`
}

type pgWithReference struct {
	Inner pgInner `json:"inner"`
}

type pgWithStructSlice struct {
	Items []pgInner `json:"items"`
}

type pgWithTagOverrides struct {
	ID   string `json:"id" postgres:"id,primarykey,type:uuid"`
	When string `json:"when" postgres:"when,default:'now()',unique"`
}

func renderPG(t *testing.T, values ...any) string {
	t.Helper()
	ctx := &Context{Context: typeGenerationContext.New()}
	if err := ctx.Add(values...); err != nil {
		t.Fatalf("ctx.Add: %v", err)
	}
	out, err := ctx.Render()
	if err != nil {
		t.Fatalf("ctx.Render: %v", err)
	}
	return out
}

func TestPrimitiveTypeMappings(t *testing.T) {
	out := renderPG(t, reflect.TypeOf(pgPrimitives{}))

	// The postgres producer uses the Go field name (capitalized) — it does
	// not consult the `json` tag for column naming.
	expectations := map[string]string{
		"S ":   "text",
		"I ":   "integer",
		"I8 ":  "smallint",
		"I64 ": "bigint",
		"F ":   "double precision",
		"B ":   "boolean",
	}
	for fieldPrefix, wantType := range expectations {
		needle := "\t" + fieldPrefix + wantType
		if !strings.Contains(out, needle) {
			t.Errorf("expected line containing %q, got:\n%s", needle, out)
		}
	}
}

func TestTimeMapsToTimestampTZ(t *testing.T) {
	out := renderPG(t, reflect.TypeOf(pgWithTime{}))
	if !strings.Contains(out, "When timestamptz") {
		t.Errorf("expected `When timestamptz`, got:\n%s", out)
	}
}

func TestByteSliceMapsToByteA(t *testing.T) {
	out := renderPG(t, reflect.TypeOf(pgWithBytes{}))
	if !strings.Contains(out, "Blob bytea") {
		t.Errorf("expected `Blob bytea`, got:\n%s", out)
	}
}

func TestStringSliceMapsToTextArray(t *testing.T) {
	out := renderPG(t, reflect.TypeOf(pgWithStringSlice{}))
	if !strings.Contains(out, "Tags text[]") {
		t.Errorf("expected `Tags text[]`, got:\n%s", out)
	}
}

func TestStructFieldBecomesReference(t *testing.T) {
	out := renderPG(t, reflect.TypeOf(pgWithReference{}))

	// The inner struct also gets its own table.
	if !strings.Contains(out, "CREATE TABLE pg_inner") {
		t.Errorf("expected nested struct's own table, got:\n%s", out)
	}
	// The outer field should reference the inner table.
	if !strings.Contains(out, "REFERENCES pg_inner(id)") {
		t.Errorf("expected REFERENCES pg_inner(id), got:\n%s", out)
	}
}

func TestStructSliceCreatesAssociativeTable(t *testing.T) {
	out := renderPG(t, reflect.TypeOf(pgWithStructSlice{}))

	// The associative table joins the two participating tables; we don't
	// assert the exact schema, just that an additional CREATE TABLE for
	// the join exists in the output.
	createCount := strings.Count(out, "CREATE TABLE ")
	if createCount < 3 {
		t.Errorf("expected at least 3 CREATE TABLE statements (outer + inner + assoc), got %d:\n%s", createCount, out)
	}
}

func TestPostgresTagOverrides(t *testing.T) {
	out := renderPG(t, reflect.TypeOf(pgWithTagOverrides{}))

	// type:uuid should override the inferred type for the `id` field.
	if !strings.Contains(out, "id uuid") {
		t.Errorf("expected id type overridden to uuid, got:\n%s", out)
	}
	// primarykey should add the PRIMARY KEY clause.
	if !strings.Contains(out, "PRIMARY KEY") {
		t.Errorf("expected PRIMARY KEY clause, got:\n%s", out)
	}
	// default:'now()' should be emitted as DEFAULT 'now()'.
	if !strings.Contains(out, "DEFAULT 'now()'") {
		t.Errorf("expected DEFAULT 'now()', got:\n%s", out)
	}
	// unique should add the UNIQUE clause.
	if !strings.Contains(out, "UNIQUE") {
		t.Errorf("expected UNIQUE clause, got:\n%s", out)
	}
}

func TestAutoIdInsertedWhenNoPrimaryKey(t *testing.T) {
	// pgPrimitives has no field tagged with primarykey, so the renderer
	// should append a default `id uuid PRIMARY KEY DEFAULT gen_random_uuid()` line.
	out := renderPG(t, reflect.TypeOf(pgPrimitives{}))
	if !strings.Contains(out, "id uuid PRIMARY KEY DEFAULT gen_random_uuid()") {
		t.Errorf("expected auto id column, got:\n%s", out)
	}
}

func TestNotNullByDefault(t *testing.T) {
	// Required (non-omitempty) fields should pick up NOT NULL.
	out := renderPG(t, reflect.TypeOf(pgPrimitives{}))
	if !strings.Contains(out, "NOT NULL") {
		t.Errorf("expected at least one NOT NULL clause, got:\n%s", out)
	}
}

type pgGeneric[T any] struct {
	Data T `json:"data"`
}

func TestGenericTypesUnsupported(t *testing.T) {
	ctx := &Context{Context: typeGenerationContext.New()}
	if err := ctx.Add(reflect.TypeOf(pgGeneric[int]{})); err != nil {
		t.Fatalf("ctx.Add: %v", err)
	}
	if _, err := ctx.Render(); err == nil {
		t.Error("expected error for generic struct, got nil")
	}
}
