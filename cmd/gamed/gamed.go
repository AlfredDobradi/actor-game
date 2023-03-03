package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/alfreddobradi/actor-game/actor/inventory"
	"github.com/alfreddobradi/actor-game/actor/scheduler"
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
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	etcdEndpoints := os.Getenv("GAMED_ETCD_ENDPOINTS")
	if etcdEndpoints == "" {
		log.Fatalln("Please set GAMED_ETCD_ENDPOINTS env var")
	}
	listeningPort := os.Getenv("GAMED_LISTENING_PORT")
	if listeningPort == "" {
		listeningPort = "80"
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)

	endpoints := strings.Split(etcdEndpoints, ",")

	system := actor.NewActorSystem()

	provider, err := etcd.NewWithConfig("/actor-game", clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
	})
	if err != nil {
		log.Fatalf("error creating etcd provider: %v", err)
	}
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)

	schedulerKind := shared.NewSchedulerKind(func() shared.Scheduler {
		return &scheduler.SchedulerGrain{}
	}, 0)

	inventoryKind := shared.NewInventoryKind(func() shared.Inventory {
		return &inventory.InventoryGrain{}
	}, 0)

	clusterConfig := cluster.Configure(
		"game-cluster",
		provider,
		lookup,
		config,
		cluster.WithKinds(inventoryKind, schedulerKind),
	)
	c := cluster.New(system, clusterConfig)
	c.StartMember()
	defer c.Shutdown(true)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)

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

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	listenAddress := fmt.Sprintf("0.0.0.0:%s", listeningPort)
	s := &http.Server{
		Addr:    listenAddress,
		Handler: r,
	}

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP listener: %v", err)
		}
	}()

	log.Printf("Listening on %s", listenAddress)
	defer s.Shutdown(context.Background()) // nolint

	<-sigchan

}
