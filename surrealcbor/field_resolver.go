package surrealcbor

import (
	"reflect"
	"strings"
	"sync"
)

// FieldResolver is an interface for resolving struct fields by name
type FieldResolver interface {
	// FindField finds a struct field by name, following the precedence:
	// 1. Exact match on CBOR/JSON tags
	// 2. Exact match on field names
	// 3. Case-insensitive match on field names
	FindField(v reflect.Value, name string) reflect.Value
}

// BasicFieldResolver is the original implementation without caching
type BasicFieldResolver struct{}

// NewBasicFieldResolver creates a new basic field resolver
func NewBasicFieldResolver() FieldResolver {
	return &BasicFieldResolver{}
}

func (r *BasicFieldResolver) FindField(v reflect.Value, name string) reflect.Value {
	// Try exact tag match
	if field := r.findFieldByTag(v, name); field.IsValid() {
		return field
	}

	// Try exact field name match (only for fields without tags)
	if field := r.findFieldByNameNoTag(v, name); field.IsValid() {
		return field
	}

	// Fallback to case-insensitive field name match (only for fields without tags)
	return r.findFieldByNameCaseInsensitiveNoTag(v, name)
}

func (r *BasicFieldResolver) findFieldByTag(v reflect.Value, name string) reflect.Value {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Handle embedded structs
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if embeddedField := r.findFieldByTag(v.Field(i), name); embeddedField.IsValid() {
				return embeddedField
			}
		}

		// Check for matching tag (only if tag name is not empty)
		// fxamacker/cbor does case-insensitive tag matching
		if tagName := getFieldTagName(&field); tagName != "" && strings.EqualFold(tagName, name) {
			return v.Field(i)
		}
	}

	return reflect.Value{}
}

// findFieldByNameNoTag finds a field by exact name match, but only if the field has no tags
func (r *BasicFieldResolver) findFieldByNameNoTag(v reflect.Value, name string) reflect.Value {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Skip fields that have tags
		if getFieldTagName(&field) != "" {
			continue
		}

		// Handle embedded structs
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if embeddedField := r.findFieldByNameNoTag(v.Field(i), name); embeddedField.IsValid() {
				return embeddedField
			}
		}

		if field.Name == name {
			return v.Field(i)
		}
	}

	return reflect.Value{}
}

// findFieldByNameCaseInsensitiveNoTag finds a field by case-insensitive name match, but only if the field has no tags
func (r *BasicFieldResolver) findFieldByNameCaseInsensitiveNoTag(v reflect.Value, name string) reflect.Value {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Skip fields that have tags
		if getFieldTagName(&field) != "" {
			continue
		}

		// Handle embedded structs
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if embeddedField := r.findFieldByNameCaseInsensitiveNoTag(v.Field(i), name); embeddedField.IsValid() {
				return embeddedField
			}
		}

		if strings.EqualFold(field.Name, name) {
			return v.Field(i)
		}
	}

	return reflect.Value{}
}

// getFieldTagName returns the CBOR or JSON tag name for a struct field
func getFieldTagName(field *reflect.StructField) string {
	if tag := field.Tag.Get("cbor"); tag != "" {
		// Parse tag to handle comma-separated options like "name,omitempty"
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		// Don't use empty tag or "-" as a name
		if tag == "" || tag == "-" {
			return ""
		}
		return tag
	}

	if tag := field.Tag.Get("json"); tag != "" {
		// Parse tag to handle comma-separated options
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		// Don't use empty tag or "-" as a name
		if tag == "" || tag == "-" {
			return ""
		}
		return tag
	}

	return ""
}

// fieldInfo contains cached information about a struct field
type fieldInfo struct {
	index      []int  // Field index path (for embedded structs)
	tagName    string // CBOR or JSON tag name
	fieldName  string // Actual field name
	isEmbedded bool   // Whether this is an embedded field
}

// structFieldsCache caches field information for struct types
type structFieldsCache struct {
	// Map from reflect.Type to field information
	// The inner map is keyed by: tag names, field names, and lowercase field names
	cache map[reflect.Type]map[string]*fieldInfo
	mu    sync.RWMutex
}

// CachedFieldResolver resolves struct fields with caching
type CachedFieldResolver struct {
	cache *structFieldsCache
}

// NewCachedFieldResolver creates a new cached field resolver
func NewCachedFieldResolver() FieldResolver {
	return &CachedFieldResolver{
		cache: &structFieldsCache{
			cache: make(map[reflect.Type]map[string]*fieldInfo),
		},
	}
}

func (r *CachedFieldResolver) FindField(v reflect.Value, name string) reflect.Value {
	t := v.Type()

	// Get or build the field map for this type
	fieldMap := r.cache.getOrBuildFieldMap(t)

	// Try exact match (tags and field names)
	if info, ok := fieldMap[name]; ok {
		return fieldByIndex(v, info.index)
	}

	// Try case-insensitive field name match
	nameLower := strings.ToLower(name)
	if info, ok := fieldMap["~"+nameLower]; ok { // Using ~ prefix for case-insensitive keys
		return fieldByIndex(v, info.index)
	}

	return reflect.Value{}
}

// fieldByIndex returns the field value at the given index path
func fieldByIndex(v reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		v = v.Field(i)
	}
	return v
}

func (c *structFieldsCache) getOrBuildFieldMap(t reflect.Type) map[string]*fieldInfo {
	c.mu.RLock()
	fieldMap, ok := c.cache[t]
	c.mu.RUnlock()

	if ok {
		return fieldMap
	}

	// Build the field map
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check in case another goroutine built it
	if fm, ok := c.cache[t]; ok {
		return fm
	}

	fieldMap = c.buildFieldMap(t, nil)
	c.cache[t] = fieldMap
	return fieldMap
}

func (c *structFieldsCache) buildFieldMap(t reflect.Type, parentIndex []int) map[string]*fieldInfo {
	fieldMap := make(map[string]*fieldInfo)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		index := append(append([]int(nil), parentIndex...), i)

		// Handle embedded structs
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			// Recursively add embedded struct fields
			embeddedMap := c.buildFieldMap(field.Type, index)
			for key, info := range embeddedMap {
				// Only add if not already present (outer fields take precedence)
				if _, exists := fieldMap[key]; !exists {
					fieldMap[key] = info
				}
			}
		}

		info := &fieldInfo{
			index:      index,
			fieldName:  field.Name,
			isEmbedded: field.Anonymous,
		}

		// Add tag-based entries (getFieldTagName already filters out empty and "-" tags)
		if tagName := getFieldTagName(&field); tagName != "" {
			info.tagName = tagName
			fieldMap[tagName] = info
			// Also add case-insensitive tag entry (fxamacker behavior)
			// Using ~ prefix to distinguish from exact matches
			fieldMap["~"+strings.ToLower(tagName)] = info
		} else {
			// Only add field name entries if there's no tag
			// Add field name entry (exact match)
			fieldMap[field.Name] = info

			// Add lowercase field name entry for case-insensitive matching
			// Using ~ prefix to distinguish from actual field/tag names
			fieldMap["~"+strings.ToLower(field.Name)] = info
		}
	}

	return fieldMap
}
