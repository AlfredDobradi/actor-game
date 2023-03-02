package inventory

import (
	"fmt"
	"sync"

	"github.com/alfreddobradi/actor-game/registry"
	"github.com/alfreddobradi/actor-game/shared"
	"github.com/asynkron/protoactor-go/cluster"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	KeyBlueprint string = "blueprint"

	KeyPopulation string = "population"
	KeyResources  string = "resources"
	KeyBuildings  string = "buildings"
)

type ResourceStore struct {
	mx *sync.Mutex

	store map[string]int64
}

type RollbackFn func()

func (r *ResourceStore) Reserve(cost map[string]int64) (RollbackFn, error) {
	r.mx.Lock()
	defer r.mx.Unlock()

	for k, v := range cost {
		if actual, ok := r.store[k]; !ok || actual < v {
			return nil, fmt.Errorf("not enough %s", k)
		}
	}

	for k, v := range cost {
		r.store[k] -= v
	}

	rollback := func() {
		r.mx.Lock()
		defer r.mx.Unlock()

		for k, v := range cost {
			r.store[k] += v
		}
	}

	return rollback, nil
}

type BuildingStore struct {
	mx *sync.Mutex

	store map[string]int64
}

func (b *BuildingStore) Build(bp registry.Blueprint) {
	b.mx.Lock()
	defer b.mx.Unlock()
	if _, ok := b.store[bp.Name]; !ok {
		b.store[bp.Name] = 1
	} else {
		b.store[bp.Name] += 1
	}
}

type InventoryGrain struct {
	ctx cluster.GrainContext

	population int64
	resources  *ResourceStore
	buildings  *BuildingStore
}

func (g *InventoryGrain) Init(ctx cluster.GrainContext) {
	g.ctx = ctx

	g.population = 100
	g.resources = &ResourceStore{
		mx: &sync.Mutex{},
		store: map[string]int64{
			shared.ResourceWood: 100,
		},
	}
	g.buildings = &BuildingStore{
		mx:    &sync.Mutex{},
		store: make(map[string]int64),
	}
}

func (g *InventoryGrain) Terminate(ctx cluster.GrainContext)      {}
func (g *InventoryGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (g *InventoryGrain) StartBuild(req *shared.BuildRequest, ctx cluster.GrainContext) (*shared.BuildResponse, error) {
	bppb, ok := req.Context.Fields[KeyBlueprint]
	if !ok {
		return &shared.BuildResponse{
			Timestamp: timestamppb.Now(),
			Status:    shared.Status_Error,
			Context: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					shared.KeyError: structpb.NewStringValue("requested blueprint not found"),
				},
			},
		}, nil
	}

	blueprintName := bppb.GetStringValue()
	blueprint, err := registry.GetBlueprint(blueprintName)
	if err != nil {
		return &shared.BuildResponse{
			Timestamp: timestamppb.Now(),
			Status:    shared.Status_Error,
			Context: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					shared.KeyError: structpb.NewStringValue("requested blueprint not found"),
				},
			},
		}, nil
	}

	_, err = g.resources.Reserve(blueprint.Cost)
	if err != nil {
		return &shared.BuildResponse{
			Timestamp: timestamppb.Now(),
			Status:    shared.Status_Error,
			Context: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					shared.KeyError: structpb.NewStringValue(err.Error()),
				},
			},
		}, nil
	}

	// TODO handle the build time

	g.buildings.Build(blueprint)

	// this should be persisted to the backend of choice immediately

	return &shared.BuildResponse{
		Timestamp: timestamppb.Now(),
		Status:    shared.Status_OK,
	}, nil
}

func (g *InventoryGrain) Describe(req *shared.DescribeInventoryRequest, ctx cluster.GrainContext) (*shared.DescribeInventoryResponse, error) {

	resources := make(map[string]*structpb.Value)
	g.resources.mx.Lock()
	for k, v := range g.resources.store {
		resources[k] = structpb.NewNumberValue(float64(v))
	}
	g.resources.mx.Unlock()

	g.buildings.mx.Lock()
	buildings := make(map[string]*structpb.Value)
	for k, v := range g.buildings.store {
		buildings[k] = structpb.NewNumberValue(float64(v))
	}
	defer g.buildings.mx.Unlock()

	res := &shared.DescribeInventoryResponse{
		Timestamp: timestamppb.Now(),
		Status:    shared.Status_OK,
		Context: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				KeyPopulation: structpb.NewNumberValue(float64(g.population)),
				KeyResources:  structpb.NewStructValue(&structpb.Struct{Fields: resources}),
				KeyBuildings:  structpb.NewStructValue(&structpb.Struct{Fields: buildings}),
			},
		},
	}

	return res, nil
}
