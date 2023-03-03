package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/alfreddobradi/actor-game/shared"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	KeyTimerID string = "timer_id"
)

type Timer struct {
	t       *time.Timer
	reply   string
	payload map[string]interface{}
}

func NewTimer(dur time.Duration) *Timer {
	timer := &Timer{
		t: time.NewTimer(dur),
	}

	return timer
}

type TimerStore struct {
	mx *sync.Mutex

	store map[uuid.UUID]*Timer
}

func (t *TimerStore) Add(timer *Timer) uuid.UUID {
	t.mx.Lock()
	defer t.mx.Unlock()

	id := uuid.New()
	t.store[id] = timer
	return id
}

type SchedulerGrain struct {
	ctx cluster.GrainContext

	notifier chan uuid.UUID
	timers   *TimerStore
}

func (g *SchedulerGrain) Init(ctx cluster.GrainContext) {
	g.ctx = ctx

	g.timers = &TimerStore{
		mx:    &sync.Mutex{},
		store: make(map[uuid.UUID]*Timer),
	}

	g.notifier = make(chan uuid.UUID)

	go func(n chan uuid.UUID) {
		for notice := range n {
			g.timers.mx.Lock()
			timerData := g.timers.store[notice]
			log.Printf("I should be replying to [%s] with this payload:\n%#v", timerData.reply, timerData.payload)
			delete(g.timers.store, notice)
			g.timers.mx.Unlock()
		}
	}(g.notifier)
}

func (g SchedulerGrain) Terminate(ctx cluster.GrainContext) {
	close(g.notifier)
	g.persist() // nolint
}

func (g SchedulerGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (g *SchedulerGrain) persist() error {
	log.Println("dummy persist scheduler")
	g.timers.mx.Lock()
	defer g.timers.mx.Unlock()
	for id, timer := range g.timers.store {
		log.Printf("Stopping timer %s", id)
		timer.t.Stop()
	}
	return nil
}

func (g *SchedulerGrain) Schedule(req *shared.ScheduleRequest, ctx cluster.GrainContext) (*shared.ScheduleResponse, error) {
	if req.Reply == "" {
		return &shared.ScheduleResponse{
			Timestamp: timestamppb.Now(),
			Status:    shared.Status_Error,
			Context: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					shared.KeyError: structpb.NewStringValue("reply topic shouldn't be empty"),
				},
			},
		}, nil
	}

	log.Printf("got a schedule request: %#v", req)

	dur, err := time.ParseDuration(req.Duration)
	if err != nil {
		return &shared.ScheduleResponse{
			Timestamp: timestamppb.Now(),
			Status:    shared.Status_Error,
			Context: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					shared.KeyError: structpb.NewStringValue(fmt.Sprintf("failed to parse duration: %v", err)),
				},
			},
		}, nil
	}

	t := time.NewTimer(dur)
	id := g.timers.Add(&Timer{
		t:       t,
		reply:   req.Reply,
		payload: req.Context.AsMap(),
	})
	log.Printf("created timer %s", id)

	go func() {
		for range t.C {
			g.notifier <- id
			return
		}
	}()

	return &shared.ScheduleResponse{
		Timestamp: timestamppb.Now(),
		Status:    shared.Status_Error,
		Context: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				KeyTimerID: structpb.NewStringValue(id.String()),
			},
		},
	}, nil
}
