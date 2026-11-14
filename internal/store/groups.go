package store

import (
	"context"
	"fmt"
)

// Group is a named folder of connections in the sidebar.
type Group struct {
	ID        string
	Name      string
	SortOrder int
}

// Groups stores connection groups.
type Groups struct{ s *Store }

// List returns all groups by sort order, then name.
func (r *Groups) List(ctx context.Context) ([]Group, error) {
	rows, err := r.s.db.QueryContext(ctx,
		`SELECT id, name, sort_order FROM connection_groups ORDER BY sort_order, name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	groups := []Group{}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.SortOrder); err != nil {
			return nil, fmt.Errorf("list groups: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	return groups, nil
}

// Create inserts g, assigning an ID when it has none, and returns the stored
// group.
func (r *Groups) Create(ctx context.Context, g Group) (Group, error) {
	if g.ID == "" {
		g.ID = newID()
	}
	_, err := r.s.db.ExecContext(ctx,
		`INSERT INTO connection_groups (id, name, sort_order) VALUES (?, ?, ?)`,
		g.ID, g.Name, g.SortOrder)
	if err != nil {
		return Group{}, fmt.Errorf("create group: %w", err)
	}
	return g, nil
}

// Update saves g's name and sort order.
func (r *Groups) Update(ctx context.Context, g Group) error {
	res, err := r.s.db.ExecContext(ctx,
		`UPDATE connection_groups SET name = ?, sort_order = ? WHERE id = ?`,
		g.Name, g.SortOrder, g.ID)
	if err != nil {
		return fmt.Errorf("update group %s: %w", g.ID, err)
	}
	return expectOneRow(res, "update group "+g.ID)
}

// Delete removes a group. Its connections stay, without a group.
func (r *Groups) Delete(ctx context.Context, id string) error {
	res, err := r.s.db.ExecContext(ctx, `DELETE FROM connection_groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete group %s: %w", id, err)
	}
	return expectOneRow(res, "delete group "+id)
}
