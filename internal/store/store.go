// Package store keeps hot state in Redis: latest telemetry per drone and command records.
//
// Postgres/Timescale (history, missions, audit) is added in a later milestone; see migrations/.
package store

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	controlv1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/control/v1"
	dronev1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/drone/v1"
)

var ErrNotFound = errors.New("not found")

const (
	keyLatest = "swarm:drones:latest" // hash: drone_id -> Telemetry
	keyCmd    = "swarm:cmd:"          // string: Command record
	keyIdem   = "swarm:idem:"         // string: idempotency key -> command_id
	recordTTL = 24 * time.Hour
)

type Store struct{ rdb *redis.Client }

func New(addr string) *Store {
	return &Store{rdb: redis.NewClient(&redis.Options{Addr: addr})}
}

func (s *Store) Ping(ctx context.Context) error { return s.rdb.Ping(ctx).Err() }

// ---------------------------------------------------------------- telemetry

func (s *Store) PutTelemetry(ctx context.Context, t *dronev1.Telemetry) error {
	b, err := proto.Marshal(t)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyLatest, t.GetDroneId(), b).Err()
}

func (s *Store) GetTelemetry(ctx context.Context, droneID string) (*dronev1.Telemetry, error) {
	b, err := s.rdb.HGet(ctx, keyLatest, droneID).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t := &dronev1.Telemetry{}
	return t, proto.Unmarshal(b, t)
}

func (s *Store) ListTelemetry(ctx context.Context) ([]*dronev1.Telemetry, error) {
	m, err := s.rdb.HGetAll(ctx, keyLatest).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*dronev1.Telemetry, 0, len(m))
	for _, v := range m {
		t := &dronev1.Telemetry{}
		if err := proto.Unmarshal([]byte(v), t); err == nil {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetDroneId() < out[j].GetDroneId() })
	return out, nil
}

// ---------------------------------------------------------------- commands

func (s *Store) PutCommand(ctx context.Context, rec *controlv1.CommandRecord) error {
	b, err := proto.Marshal(rec)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, keyCmd+rec.GetCommandId(), b, recordTTL).Err()
}

func (s *Store) GetCommand(ctx context.Context, id string) (*controlv1.CommandRecord, error) {
	b, err := s.rdb.Get(ctx, keyCmd+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rec := &controlv1.CommandRecord{}
	return rec, proto.Unmarshal(b, rec)
}

// ClaimIdempotencyKey associates key with commandID if unused. If the key was
// already claimed it returns the original command id and claimed=false.
func (s *Store) ClaimIdempotencyKey(ctx context.Context, key, commandID string) (existing string, claimed bool, err error) {
	ok, err := s.rdb.SetNX(ctx, keyIdem+key, commandID, recordTTL).Result()
	if err != nil || ok {
		return "", ok, err
	}
	existing, err = s.rdb.Get(ctx, keyIdem+key).Result()
	return existing, false, err
}
