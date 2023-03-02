package hello

import (
	"fmt"

	"github.com/alfreddobradi/actor-game/shared"
	"github.com/asynkron/protoactor-go/cluster"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	KeyName    string = "name"
	KeyMessage string = "message"
	KeyError   string = "error"
)

type HelloGrain struct{}

func (h HelloGrain) Init(ctx cluster.GrainContext)           {}
func (h HelloGrain) Terminate(ctx cluster.GrainContext)      {}
func (h HelloGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (h HelloGrain) SayHello(request *shared.HelloRequest, ctx cluster.GrainContext) (*shared.HelloResponse, error) {
	fields := make(map[string]*structpb.Value)
	var status shared.Status = shared.Status_OK
	if field, ok := request.Context.Fields["name"]; !ok || field.GetStringValue() == "" {
		fields["error"] = structpb.NewStringValue("name cannot be empty")
		status = shared.Status_Error
	} else {
		fields["message"] = structpb.NewStringValue(fmt.Sprintf("hello %s", field.GetStringValue()))
	}

	ts := timestamppb.Now()
	context := &structpb.Struct{
		Fields: fields,
	}
	return &shared.HelloResponse{Timestamp: ts, Context: context, Status: status}, nil
}
