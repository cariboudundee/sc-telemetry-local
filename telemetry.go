package main

// Client du serveur de télémétrie local du jeu (gRPC, lecture seule).

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

const service = "sc.game.telemetry.v1.GameTelemetryService"

type Telemetry struct {
	mu        sync.RWMutex
	in, out   protoreflect.MessageDescriptor
	latest    proto.Message
	at        time.Time
	Connected bool
	Frames    uint64
	LastErr   string
}

func newTelemetry(set *descriptorpb.FileDescriptorSet) (*Telemetry, error) {
	files, err := protodesc.NewFiles(set)
	if err != nil {
		return nil, err
	}
	d, err := files.FindDescriptorByName(service)
	if err != nil {
		return nil, fmt.Errorf("service %s absent du schéma : %w", service, err)
	}
	m := d.(protoreflect.ServiceDescriptor).Methods().ByName("TelemetryStream")
	if m == nil {
		return nil, fmt.Errorf("méthode TelemetryStream absente du schéma")
	}
	return &Telemetry{in: m.Input(), out: m.Output()}, nil
}

// run se connecte au jeu et se reconnecte tout seul (jeu fermé, chargement, patch…).
func (t *Telemetry) run(port func() int) {
	for {
		addr := fmt.Sprintf("127.0.0.1:%d", port())
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			err = t.stream(conn)
			conn.Close()
		}
		t.mu.Lock()
		t.Connected, t.latest = false, nil
		if err != nil {
			t.LastErr = err.Error()
		}
		t.mu.Unlock()
		time.Sleep(3 * time.Second)
	}
}

func (t *Telemetry) stream(conn *grpc.ClientConn) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	st, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, "/"+service+"/TelemetryStream")
	if err != nil {
		return err
	}
	if err := st.SendMsg(dynamicpb.NewMessage(t.in)); err != nil {
		return err
	}
	st.CloseSend()
	for {
		msg := dynamicpb.NewMessage(t.out)
		if err := st.RecvMsg(msg); err != nil {
			return err
		}
		t.mu.Lock()
		t.latest, t.at, t.Connected, t.LastErr = msg, time.Now(), true, ""
		t.Frames++
		t.mu.Unlock()
	}
}

// JSON renvoie la dernière trame (noms de champs du jeu), ou nil.
func (t *Telemetry) JSON() []byte {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.latest == nil {
		return nil
	}
	b, _ := protojson.MarshalOptions{UseProtoNames: true}.Marshal(t.latest)
	return b
}

// JSONFull : dernière trame avec tous les champs (y compris à zéro / faux), énumérations en nombres
// (enums=true) ou en noms. Sert au pont Virpil, qui doit voir passer les valeurs qui retombent à 0.
func (t *Telemetry) JSONFull(enums bool) []byte {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.latest == nil {
		return nil
	}
	b, _ := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true, UseEnumNumbers: enums}.Marshal(t.latest)
	return b
}