//go:build !windows

package importers

func readRegistrySessions(string) (map[string]map[string]string, error) {
	return nil, ErrRegistryUnsupported
}
