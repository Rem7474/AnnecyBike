package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
	*pgxpool.Pool
}

func Connect(ctx context.Context, dbURL string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, err
	}
	cfg.MinConns = 2
	cfg.MaxConns = 20
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Pool{pool}, nil
}

// FetchGeofencingZones returns the latest geofencing GeoJSON stored by the poller.
// Returns nil, nil if no zones have been fetched yet.
func (p *Pool) FetchGeofencingZones(ctx context.Context) ([]byte, error) {
	var raw []byte
	err := p.QueryRow(ctx, `SELECT geojson FROM geofencing_zones WHERE id = 1`).Scan(&raw)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
