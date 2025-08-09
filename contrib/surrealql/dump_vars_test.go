package surrealql_test

import (
	"fmt"
	"maps"
	"slices"
	"sort"
)

// dumpVars prints all variables in ascending order by key
func dumpVars(vars map[string]any) {
	if len(vars) == 0 {
		fmt.Println("Vars: (empty)")
		return
	}

	keys := slices.Collect(maps.Keys(vars))
	sort.Strings(keys)

	fmt.Println("Vars:")
	for _, key := range keys {
		fmt.Printf("  %s: %v\n", key, vars[key])
	}
}
