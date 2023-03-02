package registry

var validBlueprints []string = []string{
	BlueprintHouse,
}

func IsValidBlueprint(name string) bool {
	for _, bp := range validBlueprints {
		if name == bp {
			return true
		}
	}

	return false
}
