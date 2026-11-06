//go:build windows

package importers

import (
	"strconv"

	"golang.org/x/sys/windows/registry"
)

// readRegistrySessions reads HKEY_CURRENT_USER\<path>\<session> values.
func readRegistrySessions(path string) (map[string]map[string]string, error) {
	root, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]string{}
	for _, name := range names {
		k, err := registry.OpenKey(root, name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		values, _ := k.ReadValueNames(-1)
		kv := map[string]string{}
		for _, v := range values {
			if s, _, err := k.GetStringValue(v); err == nil {
				kv[v] = s
			} else if n, _, err := k.GetIntegerValue(v); err == nil {
				kv[v] = strconv.FormatUint(n, 10)
			}
		}
		k.Close()
		out[name] = kv
	}
	return out, nil
}
