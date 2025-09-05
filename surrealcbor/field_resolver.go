package surrealcbor

import (
	"bytes"
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

	// FindFieldBytes is like FindField but accepts bytes to avoid string allocation
	// This is useful when the field name comes from a temporary buffer
	FindFieldBytes(v reflect.Value, name []byte) reflect.Value
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

func (r *BasicFieldResolver) FindFieldBytes(v reflect.Value, name []byte) reflect.Value {
	// For BasicFieldResolver, we just convert to string and use the regular path
	// This allocates, but BasicFieldResolver is only used as a fallback
	return r.FindField(v, string(name))
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
	// Byte lookup map for zero-allocation field resolution
	// Maps from reflect.Type to a slice of (keyBytes, fieldInfo) pairs
	byteCache map[reflect.Type][]bytesFieldEntry
	mu        sync.RWMutex
}

// bytesFieldEntry stores byte representation of field name for fast comparison
type bytesFieldEntry struct {
	keyBytes      []byte     // Byte representation of the key (tag or field name)
	keyBytesLower []byte     // Lowercase version for case-insensitive matching
	info          *fieldInfo // Field information
	isExact       bool       // Whether this requires exact match (tags and explicit field names)
}

// CachedFieldResolver resolves struct fields with caching
type CachedFieldResolver struct {
	cache *structFieldsCache
}

// NewCachedFieldResolver creates a new cached field resolver
func NewCachedFieldResolver() FieldResolver {
	return &CachedFieldResolver{
		cache: &structFieldsCache{
			cache:     make(map[reflect.Type]map[string]*fieldInfo),
			byteCache: make(map[reflect.Type][]bytesFieldEntry),
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

func (r *CachedFieldResolver) FindFieldBytes(v reflect.Value, name []byte) reflect.Value {
	t := v.Type()

	// Get or build the byte cache for this type
	byteEntries := r.cache.getOrBuildByteCache(t)

	// Try exact match first (tags and field names)
	for i := range byteEntries {
		entry := &byteEntries[i]
		if entry.isExact && bytesEqual(entry.keyBytes, name) {
			return fieldByIndex(v, entry.info.index)
		}
	}

	// Try case-insensitive field name match
	for i := range byteEntries {
		entry := &byteEntries[i]
		if !entry.isExact && bytesEqualFold(entry.keyBytes, name) {
			return fieldByIndex(v, entry.info.index)
		}
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

// bytesEqual compares two byte slices for equality
func bytesEqual(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// bytesEqualFold compares two byte slices case-insensitively
func bytesEqualFold(a, b []byte) bool {
	return bytes.EqualFold(a, b)
}

// getOrBuildByteCache returns the byte cache for a type, building it if necessary
func (c *structFieldsCache) getOrBuildByteCache(t reflect.Type) []bytesFieldEntry {
	c.mu.RLock()
	entries, ok := c.byteCache[t]
	c.mu.RUnlock()

	if ok {
		return entries
	}

	// Build the byte cache
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check in case another goroutine built it
	if e, ok := c.byteCache[t]; ok {
		return e
	}

	// First ensure the string cache is built
	fieldMap := c.cache[t]
	if fieldMap == nil {
		fieldMap = c.buildFieldMap(t, nil)
		c.cache[t] = fieldMap
	}

	// Build byte entries from the field map
	entries = c.buildByteEntries(fieldMap)
	c.byteCache[t] = entries
	return entries
}

// buildByteEntries builds byte entries from a field map
func (c *structFieldsCache) buildByteEntries(fieldMap map[string]*fieldInfo) []bytesFieldEntry {
	var entries []bytesFieldEntry

	// Track which fields we've already added to avoid duplicates
	seen := make(map[*fieldInfo]bool)

	for key, info := range fieldMap {
		// Skip case-insensitive entries (those starting with ~)
		if strings.HasPrefix(key, "~") {
			continue
		}

		// Skip if we've already processed this field
		if seen[info] {
			continue
		}
		seen[info] = true

		// If field has a tag, use that as the exact match
		if info.tagName != "" {
			entries = append(entries,
				bytesFieldEntry{
					keyBytes: []byte(info.tagName),
					info:     info,
					isExact:  true,
				},
				// Also add case-insensitive version
				bytesFieldEntry{
					keyBytes:      []byte(strings.ToLower(info.tagName)),
					keyBytesLower: []byte(strings.ToLower(info.tagName)),
					info:          info,
					isExact:       false,
				})
		} else {
			// No tag, use field name
			entries = append(entries,
				bytesFieldEntry{
					keyBytes: []byte(info.fieldName),
					info:     info,
					isExact:  true,
				},
				// Also add case-insensitive version
				bytesFieldEntry{
					keyBytes:      []byte(strings.ToLower(info.fieldName)),
					keyBytesLower: []byte(strings.ToLower(info.fieldName)),
					info:          info,
					isExact:       false,
				})
		}
	}

	return entries
}
