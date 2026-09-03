package memory

import (
	"context"
	"sort"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
)

// TraverseGraph implements entitygraph.DataManager via a breadth-first walk
// guarded by a visited set — unlike postgres.Backend's recursive-CTE
// traversal, an in-memory BFS can cheaply avoid re-expanding an already
// visited vertex, so this one has no cyclic-graph caveat.
func (b *Backend) TraverseGraph(ctx context.Context, req entitygraph.TraverseGraphRequest) (entitygraph.TraverseGraphResult, error) {
	direction := req.Direction
	if direction == "" {
		direction = "any"
	}
	depth := req.Depth
	if depth <= 0 {
		depth = 1
	}
	names := map[string]bool{}
	for _, n := range req.Names {
		names[n] = true
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	visited := map[string]bool{req.StartID: true}
	frontier := []string{req.StartID}
	result := entitygraph.TraverseGraphResult{}
	seenEdge := map[string]bool{}

	for d := 0; d < depth && len(frontier) > 0; d++ {
		var next []string
		for _, vertexID := range frontier {
			for _, r := range b.relationships {
				if r.AgencyID != req.AgencyID {
					continue
				}
				if len(names) > 0 && !names[r.Name] {
					continue
				}
				var neighbor string
				switch {
				case direction == "outbound" && r.FromID == vertexID:
					neighbor = r.ToID
				case direction == "inbound" && r.ToID == vertexID:
					neighbor = r.FromID
				case direction == "any" && r.FromID == vertexID:
					neighbor = r.ToID
				case direction == "any" && r.ToID == vertexID:
					neighbor = r.FromID
				default:
					continue
				}
				if !seenEdge[r.ID] {
					seenEdge[r.ID] = true
					result.Edges = append(result.Edges, r)
				}
				if !visited[neighbor] {
					visited[neighbor] = true
					next = append(next, neighbor)
				}
			}
		}
		frontier = next
	}

	for id := range visited {
		e, ok := b.entities[id]
		if !ok || e.Deleted {
			continue
		}
		result.Vertices = append(result.Vertices, e)
	}
	sort.Slice(result.Vertices, func(i, j int) bool { return result.Vertices[i].ID < result.Vertices[j].ID })

	return result, nil
}
