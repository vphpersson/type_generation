package context

import (
	"reflect"
	"testing"

	"github.com/vphpersson/type_generation/pkg/types/type_declaration"
)

type ctxLeaf struct {
	Value string `json:"value"`
}

type ctxOuter struct {
	Leaf ctxLeaf `json:"leaf"`
}

type CtxEmbeddedBase struct {
	Base string `json:"base"`
}

type ctxWithEmbedded struct {
	CtxEmbeddedBase
	Own int `json:"own"`
}

func TestAddRegistersStructWithProperties(t *testing.T) {
	c := New()
	rt := reflect.TypeOf(ctxLeaf{})
	if err := c.Add(rt); err != nil {
		t.Fatalf("Add: %v", err)
	}

	decl, ok := c.TypeDeclarations[rt]
	if !ok {
		t.Fatalf("expected type to be registered")
	}
	iface, ok := decl.(*type_declaration.InterfaceDeclaration)
	if !ok {
		t.Fatalf("expected InterfaceDeclaration, got %T", decl)
	}
	if len(iface.Properties) != 1 {
		t.Errorf("expected 1 property, got %d", len(iface.Properties))
	}
	if iface.Properties[0].Identifier != "Value" {
		t.Errorf("Identifier = %q, want %q", iface.Properties[0].Identifier, "Value")
	}
}

func TestAddNestedStructDiscoversBoth(t *testing.T) {
	c := New()
	if err := c.Add(reflect.TypeOf(ctxOuter{})); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, ok := c.TypeDeclarations[reflect.TypeOf(ctxOuter{})]; !ok {
		t.Error("expected ctxOuter to be registered")
	}
	if _, ok := c.TypeDeclarations[reflect.TypeOf(ctxLeaf{})]; !ok {
		t.Error("expected ctxLeaf (nested) to be registered")
	}

	// Order matters: the leaf is finalized first so that the outer can
	// reference it without forward declarations.
	if len(c.TypeDeclarationsInOrder) < 2 {
		t.Fatalf("expected 2 declarations in order, got %d", len(c.TypeDeclarationsInOrder))
	}
	leafIdent := ""
	outerIdent := ""
	for _, decl := range c.TypeDeclarationsInOrder {
		if iface, ok := decl.(*type_declaration.InterfaceDeclaration); ok {
			switch iface.Identifier {
			case "CtxLeaf":
				if leafIdent == "" {
					leafIdent = iface.Identifier
				}
			case "CtxOuter":
				outerIdent = iface.Identifier
				if leafIdent == "" {
					t.Error("CtxOuter appears before CtxLeaf — nested types should be finalized first")
				}
			}
		}
	}
	if leafIdent == "" || outerIdent == "" {
		t.Errorf("expected both CtxLeaf and CtxOuter in InOrder, got leaf=%q outer=%q", leafIdent, outerIdent)
	}
}

func TestAddEmbeddedStructFlattensProperties(t *testing.T) {
	c := New()
	rt := reflect.TypeOf(ctxWithEmbedded{})
	if err := c.Add(rt); err != nil {
		t.Fatalf("Add: %v", err)
	}

	decl := c.TypeDeclarations[rt].(*type_declaration.InterfaceDeclaration)

	identifiers := map[string]bool{}
	for _, p := range decl.Properties {
		identifiers[p.Identifier] = true
	}
	if !identifiers["Base"] {
		t.Errorf("expected `Base` field flattened from embedded struct, got: %v", identifiers)
	}
	if !identifiers["Own"] {
		t.Errorf("expected `Own` field, got: %v", identifiers)
	}
}

func TestAddNonStructIsIgnored(t *testing.T) {
	c := New()
	if err := c.Add(reflect.TypeOf(42)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(c.TypeDeclarations) != 0 {
		t.Errorf("expected no declarations for non-struct, got %d", len(c.TypeDeclarations))
	}
}

func TestAddIsIdempotent(t *testing.T) {
	c := New()
	rt := reflect.TypeOf(ctxLeaf{})
	if err := c.Add(rt); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := c.Add(rt); err != nil {
		t.Fatalf("second Add: %v", err)
	}

	if got := len(c.TypeDeclarationsInOrder); got != 1 {
		t.Errorf("expected adding the same type twice to keep one declaration, got %d", got)
	}
}
