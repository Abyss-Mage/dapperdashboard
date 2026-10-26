package workspace

import (
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// The live staff map shows OpenStreetMap tiles. OpenStreetMap's servers refuse browsers that arrive without an identifying
// Referer, so tiles are fetched here instead: signed-in users only, with an identifying User-Agent, validated z/x/y, and a
// small in-memory cache so panning and refreshing does not hit the volunteer-run servers again. Tiles © OpenStreetMap contributors.
type tileCache struct {
	mu    sync.Mutex
	items map[string]cachedTile
	order []string
}
type cachedTile struct {
	body    []byte
	expires time.Time
}

const maxCachedTiles = 600

func (c *tileCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.items[key]
	if !ok || time.Now().After(t.expires) {
		return nil, false
	}
	return t.body, true
}
func (c *tileCache) put(key string, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[key]; !ok {
		c.order = append(c.order, key)
		if len(c.order) > maxCachedTiles {
			delete(c.items, c.order[0])
			c.order = c.order[1:]
		}
	}
	c.items[key] = cachedTile{body: body, expires: time.Now().Add(24 * time.Hour)}
}

func registerMapTileRoutes(mux *http.ServeMux, origin string) {
	cache := &tileCache{items: map[string]cachedTile{}}
	client := &http.Client{Timeout: 8 * time.Second}
	mux.HandleFunc("GET /api/v1/map-tiles/{z}/{x}/{y}", func(w http.ResponseWriter, r *http.Request) {
		z, ez := strconv.Atoi(r.PathValue("z"))
		x, ex := strconv.Atoi(r.PathValue("x"))
		y, ey := strconv.Atoi(r.PathValue("y"))
		if ez != nil || ex != nil || ey != nil || z < 0 || z > 19 || x < 0 || y < 0 || x >= 1<<z || y >= 1<<z {
			reply(w, 400, map[string]string{"error": "Invalid map tile"})
			return
		}
		key := r.PathValue("z") + "/" + r.PathValue("x") + "/" + r.PathValue("y")
		body, ok := cache.get(key)
		if !ok {
			req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://tile.openstreetmap.org/"+key+".png", nil)
			req.Header.Set("User-Agent", "DapperDetailingPlatform/1.0 (staff map; "+origin+")")
			req.Header.Set("Referer", origin+"/")
			resp, err := client.Do(req)
			if err != nil {
				reply(w, 502, map[string]string{"error": "Map tiles are unavailable"})
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				reply(w, 502, map[string]string{"error": "Map tiles are unavailable"})
				return
			}
			body, err = io.ReadAll(io.LimitReader(resp.Body, 512<<10))
			if err != nil {
				reply(w, 502, map[string]string{"error": "Map tiles are unavailable"})
				return
			}
			cache.put(key, body)
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "private, max-age=86400")
		_, _ = w.Write(body)
	})
}
