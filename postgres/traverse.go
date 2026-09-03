package postgres

import (
	"context"
	"fmt"

	"github.com/lib/pq"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
)

// TraverseGraph implements entitygraph.DataManager via a recursive CTE walk
// over the relationships table, replacing the ArangoDB original's AQL graph
// traversal.
//
// Known limitation: the walk does not guard against revisiting a vertex, so
// a cyclic graph re-expands every cycle it finds at every depth up to
// req.Depth. Bounded by Depth as documented, but a caller passing a large
// Depth against a densely cyclic graph can make this slow. None of
// mwanachama-backend-git's or mwanachama-backend-taskmanager's schemas declare cyclic
// relationship sets today (commit ancestry and task dependencies are both
// DAGs), so this has not needed a "visited" guard yet.
func (b *Backend) TraverseGraph(ctx context.Context, req entitygraph.TraverseGraphRequest) (entitygraph.TraverseGraphResult, error) {
	direction := req.Direction
	if direction == "" {
		direction = "any"
	}
	depth := req.Depth
	if depth <= 0 {
		depth = 1
	}
	var names pq.StringArray
	if len(req.Names) > 0 {
		names = pq.StringArray(req.Names)
	}

	q := fmt.Sprintf(`
		WITH RECURSIVE walk AS (
			SELECT $1::text AS vertex_id, NULL::text AS edge_id, 0 AS depth
			UNION ALL
			SELECT
				CASE
					WHEN $3 = 'outbound' THEN r.to_id
					WHEN $3 = 'inbound'  THEN r.from_id
					ELSE CASE WHEN r.from_id = w.vertex_id THEN r.to_id ELSE r.from_id END
				END AS vertex_id,
				r.id AS edge_id,
				w.depth + 1 AS depth
			FROM walk w
			JOIN %s r ON
				r.agency_id = $2 AND
				(
					($3 = 'outbound' AND r.from_id = w.vertex_id) OR
					($3 = 'inbound'  AND r.to_id   = w.vertex_id) OR
					($3 = 'any'      AND (r.from_id = w.vertex_id OR r.to_id = w.vertex_id))
				) AND
				($4::text[] IS NULL OR r.name = ANY($4))
			WHERE w.depth < $5
		)
		SELECT vertex_id, edge_id, depth FROM walk ORDER BY depth
	`, b.tables.Relationships)

	rows, err := b.db.QueryContext(ctx, q, req.StartID, req.AgencyID, direction, names, depth)
	if err != nil {
		return entitygraph.TraverseGraphResult{}, err
	}
	vertexIDs := []string{}
	seenVertex := map[string]bool{}
	edgeIDs := []string{}
	seenEdge := map[string]bool{}
	for rows.Next() {
		var vertexID string
		var edgeID *string
		var d int
		if err := rows.Scan(&vertexID, &edgeID, &d); err != nil {
			rows.Close()
			return entitygraph.TraverseGraphResult{}, err
		}
		if !seenVertex[vertexID] {
			seenVertex[vertexID] = true
			vertexIDs = append(vertexIDs, vertexID)
		}
		if edgeID != nil && !seenEdge[*edgeID] {
			seenEdge[*edgeID] = true
			edgeIDs = append(edgeIDs, *edgeID)
		}
	}
	if err := rows.Err(); err != nil {
		return entitygraph.TraverseGraphResult{}, err
	}
	rows.Close()

	result := entitygraph.TraverseGraphResult{}
	if len(vertexIDs) > 0 {
		vq := fmt.Sprintf(`SELECT %s FROM %s WHERE agency_id = $1 AND id = ANY($2) AND NOT deleted`, entityCols, b.tables.Entities)
		vrows, err := b.db.QueryContext(ctx, vq, req.AgencyID, pq.StringArray(vertexIDs))
		if err != nil {
			return entitygraph.TraverseGraphResult{}, err
		}
		defer vrows.Close()
		for vrows.Next() {
			e, err := scanEntity(vrows)
			if err != nil {
				return entitygraph.TraverseGraphResult{}, err
			}
			result.Vertices = append(result.Vertices, e)
		}
		if err := vrows.Err(); err != nil {
			return entitygraph.TraverseGraphResult{}, err
		}
	}

	if len(edgeIDs) > 0 {
		byID := make(map[string]entitygraph.Relationship, len(edgeIDs))
		eq := fmt.Sprintf(`SELECT %s FROM %s WHERE agency_id = $1 AND id = ANY($2)`, relationshipCols, b.tables.Relationships)
		erows, err := b.db.QueryContext(ctx, eq, req.AgencyID, pq.StringArray(edgeIDs))
		if err != nil {
			return entitygraph.TraverseGraphResult{}, err
		}
		defer erows.Close()
		for erows.Next() {
			r, err := scanRelationship(erows)
			if err != nil {
				return entitygraph.TraverseGraphResult{}, err
			}
			byID[r.ID] = r
		}
		if err := erows.Err(); err != nil {
			return entitygraph.TraverseGraphResult{}, err
		}
		// Preserve discovery order (edgeIDs), not the second query's row order.
		for _, id := range edgeIDs {
			result.Edges = append(result.Edges, byID[id])
		}
	}

	return result, nil
}
