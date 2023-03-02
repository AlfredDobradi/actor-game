package scheduler

import (
	"log"

	"github.com/asynkron/protoactor-go/cluster"
)

type SchedulerGrain struct {
	ctx cluster.GrainContext
}

func (g *SchedulerGrain) Init(ctx cluster.GrainContext) {
	g.ctx = ctx
}

func (g SchedulerGrain) Terminate(ctx cluster.GrainContext) {
	g.persist() // nolint
}

func (g SchedulerGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (g *SchedulerGrain) persist() error {
	log.Println("dummy persist scheduler")
	return nil
}
