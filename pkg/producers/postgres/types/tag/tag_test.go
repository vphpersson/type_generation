package tag

import (
	"reflect"
	"testing"
)

func TestNewEmpty(t *testing.T) {
	if got := New(""); got != nil {
		t.Errorf("expected nil for empty input, got %+v", got)
	}
	if got := New("   "); got != nil {
		t.Errorf("expected nil for whitespace input, got %+v", got)
	}
}

func TestNewSkip(t *testing.T) {
	tag := New("-")
	if tag == nil {
		t.Fatal("expected non-nil tag for -")
	}
	if !tag.Skip {
		t.Error("expected Skip=true")
	}
	if tag.Name != "" {
		t.Errorf("expected empty Name on skip tag, got %q", tag.Name)
	}
}

func TestNewBooleanFlags(t *testing.T) {
	tag := New("id_col,primarykey,unique,nullable,indexed,uniquecomposite")
	if tag == nil {
		t.Fatal("expected non-nil tag")
	}
	if tag.Name != "id_col" {
		t.Errorf("Name = %q, want %q", tag.Name, "id_col")
	}
	if !tag.PrimaryKey {
		t.Error("expected PrimaryKey=true")
	}
	if !tag.Unique {
		t.Error("expected Unique=true")
	}
	if !tag.Nullable {
		t.Error("expected Nullable=true")
	}
	if !tag.Indexed {
		t.Error("expected Indexed=true")
	}
	if !tag.UniqueComposite {
		t.Error("expected UniqueComposite=true")
	}
}

func TestNewKeyValuePairs(t *testing.T) {
	tag := New("col,default:'now()',check:'len > 0',ondelete:cascade,onupdate:restrict,type:bigint,generated:expr,generatedstored:expr2")
	if tag == nil {
		t.Fatal("expected non-nil tag")
	}

	// The parser preserves the raw value (including surrounding quotes)
	// so callers can choose whether to strip them. Don't assume the parser
	// unquotes — test the actual contract.
	if tag.Default != "'now()'" {
		t.Errorf("Default = %q, want %q", tag.Default, "'now()'")
	}
	if tag.Check != "'len > 0'" {
		t.Errorf("Check = %q, want %q", tag.Check, "'len > 0'")
	}
	if tag.OnDelete != "cascade" {
		t.Errorf("OnDelete = %q, want %q", tag.OnDelete, "cascade")
	}
	if tag.OnUpdate != "restrict" {
		t.Errorf("OnUpdate = %q, want %q", tag.OnUpdate, "restrict")
	}
	if tag.Type != "bigint" {
		t.Errorf("Type = %q, want %q", tag.Type, "bigint")
	}
	if tag.Generated != "expr" {
		t.Errorf("Generated = %q, want %q", tag.Generated, "expr")
	}
	if tag.GeneratedStored != "expr2" {
		t.Errorf("GeneratedStored = %q, want %q", tag.GeneratedStored, "expr2")
	}
}

func TestSplitTopCommas_PreservesCommasInQuotedSQL(t *testing.T) {
	input := "policy,check:'status IN (''A'', ''B'')'"
	parts := splitTopCommas(input)
	want := []string{"policy", "check:'status IN (''A'', ''B'')'"}
	if !reflect.DeepEqual(parts, want) {
		t.Errorf("got %v, want %v", parts, want)
	}
}

func TestSplitTopCommas_PreservesCommasInsideParens(t *testing.T) {
	// Top-level comma after the paren group splits; the comma inside parens does not.
	input := "col,default:func(a, b),unique"
	parts := splitTopCommas(input)
	want := []string{"col", "default:func(a, b)", "unique"}
	if !reflect.DeepEqual(parts, want) {
		t.Errorf("got %v, want %v", parts, want)
	}
}

func TestSplitTopCommas_DoubleQuotes(t *testing.T) {
	input := `col,default:"a,b,c",unique`
	parts := splitTopCommas(input)
	want := []string{"col", `default:"a,b,c"`, "unique"}
	if !reflect.DeepEqual(parts, want) {
		t.Errorf("got %v, want %v", parts, want)
	}
}

func TestNewUnknownOptionFallsToOtherOptions(t *testing.T) {
	tag := New("col,no_such_flag,custom:value")
	if tag == nil {
		t.Fatal("expected non-nil tag")
	}

	want := map[string]bool{"no_such_flag": true, "custom:value": true}
	got := map[string]bool{}
	for _, opt := range tag.OtherOptions {
		got[opt] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing %q in OtherOptions: %v", k, tag.OtherOptions)
		}
	}
}

func TestNewCaseInsensitiveFlags(t *testing.T) {
	// The parser lowercases flag matches.
	tag := New("c,PRIMARYKEY,Unique")
	if !tag.PrimaryKey {
		t.Error("expected PRIMARYKEY to match PrimaryKey")
	}
	if !tag.Unique {
		t.Error("expected Unique to match Unique")
	}
}
