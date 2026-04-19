package mcp

import (
	"testing"
)

func TestSchemaString(t *testing.T) {
	t.Parallel()
	s := schemaString()
	if s["type"] != "string" {
		t.Errorf("expected type=string, got %v", s["type"])
	}
	if len(s) != 1 {
		t.Errorf("expected 1 key, got %d", len(s))
	}
}

func TestSchemaInt(t *testing.T) {
	t.Parallel()
	s := schemaInt()
	if s["type"] != "integer" {
		t.Errorf("expected type=integer, got %v", s["type"])
	}
}

func TestSchemaNum(t *testing.T) {
	t.Parallel()
	s := schemaNum()
	if s["type"] != "number" {
		t.Errorf("expected type=number, got %v", s["type"])
	}
}

func TestSchemaBool(t *testing.T) {
	t.Parallel()
	s := schemaBool()
	if s["type"] != "boolean" {
		t.Errorf("expected type=boolean, got %v", s["type"])
	}
}

func TestSchemaStringDesc(t *testing.T) {
	t.Parallel()
	s := schemaStringDesc("a test field")
	if s["type"] != "string" {
		t.Errorf("expected type=string, got %v", s["type"])
	}
	if s["description"] != "a test field" {
		t.Errorf("expected description='a test field', got %v", s["description"])
	}
	if len(s) != 2 {
		t.Errorf("expected 2 keys, got %d", len(s))
	}
}

func TestSchemaIntDesc(t *testing.T) {
	t.Parallel()
	s := schemaIntDesc("count of items")
	if s["type"] != "integer" {
		t.Errorf("expected type=integer, got %v", s["type"])
	}
	if s["description"] != "count of items" {
		t.Errorf("expected description='count of items', got %v", s["description"])
	}
}

func TestSchemaNumDesc(t *testing.T) {
	t.Parallel()
	s := schemaNumDesc("price in EUR")
	if s["type"] != "number" {
		t.Errorf("expected type=number, got %v", s["type"])
	}
	if s["description"] != "price in EUR" {
		t.Errorf("expected description='price in EUR', got %v", s["description"])
	}
}

func TestSchemaBoolDesc(t *testing.T) {
	t.Parallel()
	s := schemaBoolDesc("is active")
	if s["type"] != "boolean" {
		t.Errorf("expected type=boolean, got %v", s["type"])
	}
	if s["description"] != "is active" {
		t.Errorf("expected description='is active', got %v", s["description"])
	}
}

func TestSchemaArray(t *testing.T) {
	t.Parallel()
	s := schemaArray(schemaNum())
	if s["type"] != "array" {
		t.Errorf("expected type=array, got %v", s["type"])
	}
	items, ok := s["items"].(map[string]interface{})
	if !ok {
		t.Fatal("items should be map[string]interface{}")
	}
	if items["type"] != "number" {
		t.Errorf("expected items type=number, got %v", items["type"])
	}
}

func TestSchemaArrayDesc(t *testing.T) {
	t.Parallel()
	s := schemaArrayDesc("list of things", schemaInt())
	if s["type"] != "array" {
		t.Errorf("expected type=array, got %v", s["type"])
	}
	if s["description"] != "list of things" {
		t.Errorf("expected description, got %v", s["description"])
	}
}

func TestSchemaStringArray(t *testing.T) {
	t.Parallel()
	s := schemaStringArray()
	if s["type"] != "array" {
		t.Errorf("expected type=array, got %v", s["type"])
	}
	items, ok := s["items"].(map[string]interface{})
	if !ok {
		t.Fatal("items should be map[string]interface{}")
	}
	if items["type"] != "string" {
		t.Errorf("expected items type=string, got %v", items["type"])
	}
}

func TestSchemaStringArrayDesc(t *testing.T) {
	t.Parallel()
	s := schemaStringArrayDesc("tags for item")
	if s["type"] != "array" {
		t.Errorf("expected type=array, got %v", s["type"])
	}
	if s["description"] != "tags for item" {
		t.Errorf("expected description, got %v", s["description"])
	}
	items, ok := s["items"].(map[string]interface{})
	if !ok {
		t.Fatal("items should be map[string]interface{}")
	}
	if items["type"] != "string" {
		t.Errorf("expected items type=string, got %v", items["type"])
	}
}

func TestSchemaObject(t *testing.T) {
	t.Parallel()
	s := schemaObject()
	if s["type"] != "object" {
		t.Errorf("expected type=object, got %v", s["type"])
	}
}

// Verify each call returns a fresh map (no shared mutable state).
func TestSchemaHelpers_IndependentInstances(t *testing.T) {
	t.Parallel()
	a := schemaString()
	b := schemaString()
	a["extra"] = "modified"
	if _, ok := b["extra"]; ok {
		t.Error("modifying one schemaString result should not affect another")
	}
}
