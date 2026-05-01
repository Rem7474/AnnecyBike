package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/annecybike/backend/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  256,
	WriteBufferSize: 8192,
}

// Hub manages connected WebSocket clients and broadcasts snapshots.
type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]struct{})}
}

func (h *Hub) register(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	conn.Close()
}

func (h *Hub) broadcast(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.clients {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			slog.Warn("ws write failed, dropping client", "err", err)
			go h.unregister(conn)
		}
	}
}

// StartListener listens for PostgreSQL NOTIFY poll_done and broadcasts live snapshots.
func StartListener(ctx context.Context, pool *db.Pool, hub *Hub) {
	// Acquire a dedicated connection for LISTEN (pgxpool doesn't support LISTEN)
	rawCfg, err := pgxpool.ParseConfig(pool.Config().ConnString())
	if err != nil {
		slog.Error("ws listener: parse config failed", "err", err)
		return
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			conn, err := pool.Acquire(ctx)
			if err != nil {
				slog.Error("ws listener: acquire conn failed", "err", err)
				time.Sleep(5 * time.Second)
				continue
			}
			_ = rawCfg // suppress unused warning

			if _, err := conn.Exec(ctx, "LISTEN poll_done"); err != nil {
				conn.Release()
				time.Sleep(5 * time.Second)
				continue
			}
			slog.Info("ws listener: listening on poll_done")

			for {
				notif, err := conn.Conn().WaitForNotification(ctx)
				if err != nil {
					slog.Warn("ws listener: notification error", "err", err)
					conn.Release()
					break
				}
				_ = notif
				// Fetch fresh snapshot and broadcast
				msg := buildSnapshot(ctx, pool)
				hub.broadcast(msg)
			}
		}
	}()
}

type wsMessage struct {
	Type     string `json:"type"`
	Bikes    any    `json:"bikes"`
	Stations any    `json:"stations"`
}

func buildSnapshot(ctx context.Context, pool *db.Pool) []byte {
	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Bikes
	bikeRows, err := pool.Query(fetchCtx, `
		SELECT DISTINCT ON (bs.bike_id)
			bs.bike_id, b.vehicle_type_id,
			bs.lat, bs.lon, bs.station_id,
			bs.is_reserved, bs.is_disabled, bs.current_range_meters
		FROM bike_snapshots bs
		JOIN bikes b ON b.bike_id = bs.bike_id
		WHERE bs.time > NOW() - INTERVAL '2 minutes'
		ORDER BY bs.bike_id, bs.time DESC
	`)
	bikes := make([]map[string]any, 0)
	if err == nil {
		defer bikeRows.Close()
		for bikeRows.Next() {
			var bikeID, vtID string
			var lat, lon float64
			var stID *string
			var isRes, isDis bool
			var rng int
			if err := bikeRows.Scan(&bikeID, &vtID, &lat, &lon, &stID, &isRes, &isDis, &rng); err == nil {
				bikes = append(bikes, map[string]any{
					"bike_id":              bikeID,
					"vehicle_type_id":      vtID,
					"lat":                  lat,
					"lon":                  lon,
					"station_id":           stID,
					"is_reserved":          isRes,
					"is_disabled":          isDis,
					"current_range_meters": rng,
					"battery_pct":          batteryPct(rng),
				})
			}
		}
	}

	// Stations
	stRows, err := pool.Query(fetchCtx, `
		SELECT s.station_id, s.name, s.lat, s.lon, s.capacity,
			ss.num_bikes_available, ss.num_docks_available
		FROM stations s
		LEFT JOIN LATERAL (
			SELECT num_bikes_available, num_docks_available
			FROM station_snapshots
			WHERE station_id = s.station_id
			ORDER BY time DESC LIMIT 1
		) ss ON true
	`)
	stations := make([]map[string]any, 0)
	if err == nil {
		defer stRows.Close()
		for stRows.Next() {
			var id, name string
			var lat, lon float64
			var cap int
			var bikes, docks *int
			if err := stRows.Scan(&id, &name, &lat, &lon, &cap, &bikes, &docks); err == nil {
				stations = append(stations, map[string]any{
					"station_id":          id,
					"name":                name,
					"lat":                 lat,
					"lon":                 lon,
					"capacity":            cap,
					"num_bikes_available": bikes,
					"num_docks_available": docks,
				})
			}
		}
	}

	msg, _ := json.Marshal(wsMessage{Type: "snapshot", Bikes: bikes, Stations: stations})
	return msg
}

func WSHandler(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		hub.register(conn)

		// Keep connection alive, drain client pings
		go func() {
			defer hub.unregister(conn)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()
	}
}
