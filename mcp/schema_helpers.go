package mcp

// Schema builder helpers for MCP tool output schemas.
// These replace verbose map[string]interface{}{"type": "..."} literals
// throughout tool definitions.

func schemaString() map[string]interface{} {
	return map[string]interface{}{"type": "string"}
}

func schemaInt() map[string]interface{} {
	return map[string]interface{}{"type": "integer"}
}

func schemaNum() map[string]interface{} {
	return map[string]interface{}{"type": "number"}
}

func schemaBool() map[string]interface{} {
	return map[string]interface{}{"type": "boolean"}
}

func schemaStringDesc(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
}

func schemaIntDesc(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "integer", "description": desc}
}

func schemaNumDesc(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "number", "description": desc}
}

func schemaBoolDesc(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "boolean", "description": desc}
}

func schemaArray(items map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"type": "array", "items": items}
}

func schemaArrayDesc(desc string, items map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"type": "array", "description": desc, "items": items}
}

func schemaStringArray() map[string]interface{} {
	return map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}}
}

func schemaStringArrayDesc(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "array", "description": desc, "items": map[string]interface{}{"type": "string"}}
}

func schemaObject() map[string]interface{} {
	return map[string]interface{}{"type": "object"}
}
