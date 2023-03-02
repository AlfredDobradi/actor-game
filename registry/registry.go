package registry

import (
	"fmt"

	"github.com/alfreddobradi/actor-game/shared"
)

var blueprints *Blueprints

type Blueprint struct {
	Name         string
	Requirements []Blueprint
	Cost         map[string]int64
	Time         string
}

type Blueprints struct {
	store map[string]Blueprint
}

func GetBlueprint(name string) (Blueprint, error) {
	if blueprint, ok := blueprints.store[name]; ok {
		return blueprint, nil
	}
	return Blueprint{}, fmt.Errorf("blueprint not found")
}

func init() {
	loadBlueprints()
}

func loadBlueprints() {
	// TODO make a loader function here (from file, remote, etc)
	if blueprints == nil {
		blueprints = &Blueprints{
			store: map[string]Blueprint{
				BlueprintHouse: {
					Name:         "House",
					Requirements: nil,
					Cost: map[string]int64{
						shared.ResourceWood: 30,
					},
					Time: "1h",
				},
			},
		}
	}
}
