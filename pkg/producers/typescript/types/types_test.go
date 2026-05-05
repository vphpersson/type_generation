package types

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vphpersson/type_generation/pkg/types/shape"
	"github.com/vphpersson/type_generation/pkg/types/type_declaration"

	typeGenerationContext "github.com/vphpersson/type_generation/pkg/types/context"
)

type genericValue[V any] struct {
	Mapping map[string]V `json:"mapping"`
}

type genericKey[K comparable] struct {
	Mapping map[K]string `json:"mapping"`
}

type genericKeyValue[K comparable, V any] struct {
	Mapping map[K]V `json:"mapping"`
}

type genericPair[T any, U any] struct {
	First  T   `json:"first"`
	Second []U `json:"second"`
}

type genericPointerSlice[T any] struct {
	Pointer *T  `json:"pointer"`
	Slice   []T `json:"slice"`
}

func renderTS(t *testing.T, rt reflect.Type) string {
	t.Helper()
	ctx := &Context{Context: typeGenerationContext.New()}
	if err := ctx.Add(rt); err != nil {
		t.Fatalf("ctx.Add: %v", err)
	}
	out, err := ctx.Render()
	if err != nil {
		t.Fatalf("ctx.Render: %v", err)
	}
	return out
}

func TestRenderGenericMapValueOnly(t *testing.T) {
	out := renderTS(t, reflect.TypeOf(genericValue[int]{}))

	if !strings.Contains(out, "<V>") {
		t.Errorf("expected interface to declare type parameter <V>, got:\n%s", out)
	}
	if !strings.Contains(out, "{ [key: string]: V }") {
		t.Errorf("expected map index `string` and value `V`, got:\n%s", out)
	}
}

func TestRenderGenericTwoParams(t *testing.T) {
	out := renderTS(t, reflect.TypeOf(genericPair[int, string]{}))

	if !strings.Contains(out, "<T, U>") {
		t.Errorf("expected <T, U>, got:\n%s", out)
	}
	if !strings.Contains(out, "first: T") {
		t.Errorf("expected `first: T`, got:\n%s", out)
	}
	if !strings.Contains(out, "second: U[]") {
		t.Errorf("expected `second: U[]`, got:\n%s", out)
	}
}

func TestRenderGenericPointerAndSlice(t *testing.T) {
	// Both fields share the single type parameter T — Pointer becomes T,
	// Slice becomes T[]. This exercises the same param appearing under two
	// different field names.
	out := renderTS(t, reflect.TypeOf(genericPointerSlice[int]{}))

	if !strings.Contains(out, "<T>") {
		t.Errorf("expected <T>, got:\n%s", out)
	}
	if !strings.Contains(out, "pointer: T;") {
		t.Errorf("expected `pointer: T;`, got:\n%s", out)
	}
	if !strings.Contains(out, "slice: T[]") {
		t.Errorf("expected `slice: T[]`, got:\n%s", out)
	}
}

// addAndGetInterface registers the type with a fresh context and returns the
// resulting InterfaceDeclaration. Used by tests that need to assert on the
// discovered GenericTypeInfo without rendering all the way to a string.
func addAndGetInterface(t *testing.T, rt reflect.Type) *type_declaration.InterfaceDeclaration {
	t.Helper()
	ctx := &Context{Context: typeGenerationContext.New()}
	if err := ctx.Add(rt); err != nil {
		t.Fatalf("ctx.Add: %v", err)
	}
	decl, ok := ctx.TypeDeclarations[rt]
	if !ok {
		t.Fatalf("type declaration not registered for %v", rt)
	}
	iface, ok := decl.(*type_declaration.InterfaceDeclaration)
	if !ok {
		t.Fatalf("expected InterfaceDeclaration, got %T", decl)
	}
	return iface
}

func TestGenericMapKeyValueBothGeneric_DataLayer(t *testing.T) {
	// This is the case the bug fix targets: a single field whose generic
	// instantiation has BOTH a generic key and a generic value. Before the
	// fix, only one of K / V would be recorded; after the fix, the
	// FieldNameToShapes entry for the field carries both the KindMapKey
	// shape (with Param=K) and the KindMapValue shape (with Param=V).
	//
	// We assert at the data layer rather than rendering to a string,
	// because MapType.String() has a separate, pre-existing limitation
	// that rejects non-`string`/`number` index types — including type
	// parameters. That's a different issue and not part of this fix.
	iface := addAndGetInterface(t, reflect.TypeOf(genericKeyValue[string, int]{}))

	if iface.GenericTypeInfo == nil {
		t.Fatal("expected GenericTypeInfo to be populated")
	}
	if got := len(iface.GenericTypeInfo.TypeParameterNames); got != 2 {
		t.Fatalf("expected 2 type parameters, got %d", got)
	}

	shapes := iface.GenericTypeInfo.FieldNameToShapes["Mapping"]
	if len(shapes) != 2 {
		t.Fatalf("expected two shapes for Mapping (key + value), got %d: %v", len(shapes), shapes)
	}

	var keyParam, valueParam string
	for _, s := range shapes {
		switch s.Kind {
		case shape.KindMapKey:
			keyParam = s.Param
		case shape.KindMapValue:
			valueParam = s.Param
		default:
			t.Errorf("unexpected shape kind %v for Mapping", s.Kind)
		}
	}
	if keyParam == "" {
		t.Error("expected a KindMapKey shape for Mapping")
	}
	if valueParam == "" {
		t.Error("expected a KindMapValue shape for Mapping")
	}
	if keyParam == valueParam {
		t.Errorf("expected key and value to be different params, both got %q", keyParam)
	}

	// Both type parameters must point at this field.
	for _, paramName := range iface.GenericTypeInfo.TypeParameterNames {
		if got := iface.GenericTypeInfo.TypeParameterNameToFieldName[paramName]; got != "Mapping" {
			t.Errorf("expected param %s → Mapping, got %q", paramName, got)
		}
	}
}

type simpleInterface struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Done  bool   `json:"done"`
}

func TestRenderSimpleInterface(t *testing.T) {
	out := renderTS(t, reflect.TypeOf(simpleInterface{}))

	for _, want := range []string{
		"export interface SimpleInterface",
		"name: string",
		"count: number",
		"done: boolean",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

type withOptional struct {
	Required string `json:"required"`
	Maybe    string `json:"maybe,omitempty"`
	Skipped  string `json:"-"`
}

func TestRenderOptionalAndSkipped(t *testing.T) {
	out := renderTS(t, reflect.TypeOf(withOptional{}))

	if !strings.Contains(out, "maybe?:") {
		t.Errorf("expected `maybe?:` (optional marker), got:\n%s", out)
	}
	if !strings.Contains(out, "required:") || strings.Contains(out, "required?:") {
		t.Errorf("`required` should not be optional, got:\n%s", out)
	}
	if strings.Contains(out, "Skipped") || strings.Contains(out, `"-"`) {
		t.Errorf("json:\"-\" field should be omitted, got:\n%s", out)
	}
}

type withMapAndSlice struct {
	Mapping map[string]int `json:"mapping"`
	Items   []string       `json:"items"`
}

func TestRenderMapAndSlice(t *testing.T) {
	out := renderTS(t, reflect.TypeOf(withMapAndSlice{}))

	if !strings.Contains(out, "{ [key: string]: number }") {
		t.Errorf("expected map rendered as `{ [key: string]: number }`, got:\n%s", out)
	}
	if !strings.Contains(out, "items: string[]") {
		t.Errorf("expected slice rendered as `string[]`, got:\n%s", out)
	}
}

type EmbeddedBase struct {
	BaseField string `json:"base_field"`
}

type withEmbedded struct {
	EmbeddedBase
	OwnField int `json:"own_field"`
}

func TestRenderEmbeddedStruct(t *testing.T) {
	out := renderTS(t, reflect.TypeOf(withEmbedded{}))

	// Both fields from the embedded struct and the outer struct should appear
	// in the same interface — embedded structs are flattened into the outer
	// type, mirroring json.Marshal semantics.
	if !strings.Contains(out, "base_field: string") {
		t.Errorf("expected base_field from embedded struct, got:\n%s", out)
	}
	if !strings.Contains(out, "own_field: number") {
		t.Errorf("expected own_field from outer struct, got:\n%s", out)
	}
}

type MyInt int

type withTypeAlias struct {
	Count MyInt `json:"count"`
}

func TestRenderPrimitiveTypeAlias(t *testing.T) {
	// A field whose Go type is a named primitive (e.g. `type MyInt int`)
	// should both reference that name and emit a standalone
	// `export type MyInt = number;` declaration.
	out := renderTS(t, reflect.TypeOf(withTypeAlias{}))

	if !strings.Contains(out, "count: MyInt") {
		t.Errorf("expected count to reference MyInt, got:\n%s", out)
	}
	if !strings.Contains(out, "export type MyInt = number;") {
		t.Errorf("expected `export type MyInt = number;` declaration, got:\n%s", out)
	}
}

type nestedInner struct {
	Value string `json:"value"`
}

type nestedOuter struct {
	Inner nestedInner `json:"inner"`
}

func TestRenderNamedStructFieldDoesNotPanic(t *testing.T) {
	// Regression: GetTypeScriptType used to apply the type-alias path to any
	// named type, including struct types whose TypeDeclaration is an
	// *InterfaceDeclaration. That produced a "convert: conversion not ok:
	// *type_declaration.InterfaceDeclaration" error during rendering.
	out := renderTS(t, reflect.TypeOf(nestedOuter{}))

	if !strings.Contains(out, "inner: NestedInner") {
		t.Errorf("expected inner field to reference NestedInner, got:\n%s", out)
	}
}

func TestGenericMapKeyOnly_DataLayer(t *testing.T) {
	// Sanity check that map[K]string still works (only one shape, KindMapKey)
	// and was not broken by changing the underlying representation to a slice.
	iface := addAndGetInterface(t, reflect.TypeOf(genericKey[string]{}))

	shapes := iface.GenericTypeInfo.FieldNameToShapes["Mapping"]
	if len(shapes) != 1 {
		t.Fatalf("expected one shape for Mapping, got %d: %v", len(shapes), shapes)
	}
	if shapes[0].Kind != shape.KindMapKey {
		t.Errorf("expected KindMapKey, got %v", shapes[0].Kind)
	}
}
