package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/alfreddobradi/actor-game/actor/hello"
	"github.com/alfreddobradi/actor-game/actor/inventory"
	"github.com/alfreddobradi/actor-game/api"
	"github.com/alfreddobradi/actor-game/registry"
	"github.com/alfreddobradi/actor-game/shared"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/etcd"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	etcdEndpoints := os.Getenv("GAMED_ETCD_ENDPOINTS")
	if etcdEndpoints == "" {
		log.Fatalln("Please set GAMED_ETCD_ENDPOINTS env var")
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, os.Kill)

	endpoints := strings.Split(etcdEndpoints, ",")

	system := actor.NewActorSystem()

	provider, _ := etcd.NewWithConfig("/actor-game", clientv3.Config{
		Endpoints: endpoints,
	})
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)

	helloKind := shared.NewHelloKind(func() shared.Hello {
		return &hello.HelloGrain{}
	}, 0)
	inventoryKind := shared.NewInventoryKind(func() shared.Inventory {
		return &inventory.InventoryGrain{}
	}, 0)

	clusterConfig := cluster.Configure("game-cluster", provider, lookup, config, cluster.WithKinds(helloKind, inventoryKind))
	c := cluster.New(system, clusterConfig)
	c.StartMember()
	defer c.Shutdown(true)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		client := shared.GetHelloGrainClient(c, "mygrain1")
		res, err := client.SayHello(&shared.HelloRequest{
			Timestamp: timestamppb.Now(),
			Context: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"name": structpb.NewStringValue(name),
				},
			},
		})
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if res.Status != 0 {
			http.Error(w, res.Context.Fields[hello.KeyError].GetStringValue(), http.StatusInternalServerError)
			return
		}

		w.Write([]byte(res.Context.Fields[hello.KeyMessage].GetStringValue() + "\n"))
	})

	r.Get("/inventory", func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-User-Id")
		if user == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		id, err := uuid.Parse(user)
		if err != nil {
			log.Printf("uuid parse error: %v", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		inventoryID := shared.GenerateInventoryGrainID(id)
		client := shared.GetInventoryGrainClient(c, inventoryID.String())
		res, err := client.Describe(&shared.DescribeInventoryRequest{Timestamp: timestamppb.Now()})
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		context := res.Context.AsMap()
		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(context); err != nil {
			log.Printf("json encoding error: %v", err)
		}
	})

	r.Post("/inventory/building", func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-User-Id")
		if user == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		id, err := uuid.Parse(user)
		if err != nil {
			log.Printf("uuid parse error: %v", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		request := api.BuildRequest{}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if !registry.IsValidBlueprint(request.Blueprint) {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		buildpb := &shared.BuildRequest{
			Timestamp: timestamppb.Now(),
			Context: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					inventory.KeyBlueprint: structpb.NewStringValue(request.Blueprint),
				},
			},
		}

		inventoryID := shared.GenerateInventoryGrainID(id)
		client := shared.GetInventoryGrainClient(c, inventoryID.String())
		res, err := client.StartBuild(buildpb)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		context := res.Context.AsMap()
		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(context); err != nil {
			log.Printf("json encoding error: %v", err)
		}
	})

	s := &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: r,
	}

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP listener: %v", err)
		}
	}()

	defer s.Shutdown(context.Background()) // nolint

	<-sigchan

}
