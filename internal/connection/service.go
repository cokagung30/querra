package connection

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/cokagung30/querra/internal/secrets"
	"github.com/cokagung30/querra/internal/store"
)

// Service manages saved connections and groups. Settings live in the local
// store and passwords in the keychain; Service keeps the two in step.
type Service struct {
	store   *store.Store
	secrets secrets.Store
}

// NewService returns a Service over an open store and a secrets store (the
// OS keychain in the app, secrets.NewMemory in tests).
func NewService(st *store.Store, sec secrets.Store) *Service {
	return &Service{store: st, secrets: sec}
}

// List returns every connection, sorted by sort order, then name. Grouping
// them is up to the caller.
func (s *Service) List(ctx context.Context) ([]Connection, error) {
	rows, err := s.store.Connections.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Connection, 0, len(rows))
	for _, row := range rows {
		c, err := fromStore(row)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Get returns one connection, or ErrNotFound.
func (s *Service) Get(ctx context.Context, id string) (Connection, error) {
	row, err := s.store.Connections.Get(ctx, id)
	if err != nil {
		return Connection{}, err
	}
	return fromStore(row)
}

// Create saves a new connection at the end of its group, and its password
// when pw sets one.
func (s *Service) Create(ctx context.Context, set Settings, pw PasswordChange) (Connection, error) {
	set, err := s.check(ctx, set, pw)
	if err != nil {
		return Connection{}, err
	}
	order, err := s.nextOrder(ctx, set.GroupID)
	if err != nil {
		return Connection{}, err
	}
	state := PasswordNone
	if pw.Action == PasswordSet {
		state = PasswordSaved
	}
	row, err := toStore("", set, order, state)
	if err != nil {
		return Connection{}, err
	}
	created, err := s.store.Connections.Create(ctx, row)
	if err != nil {
		return Connection{}, err
	}
	if pw.Action == PasswordSet {
		if err := s.secrets.Set(created.ID, secrets.Password, pw.Value); err != nil {
			_ = s.store.Connections.Delete(ctx, created.ID)
			return Connection{}, fmt.Errorf("save password: %w", err)
		}
	}
	return fromStore(created)
}

// Update saves new settings for a connection and applies pw. Moving it to
// another group puts it at the end of that group.
//
// The keychain changes first, so a failed keychain write changes nothing.
// If the store then fails, the password is already changed; the old settings
// stay and the user can save again.
func (s *Service) Update(ctx context.Context, id string, set Settings, pw PasswordChange) (Connection, error) {
	set, err := s.check(ctx, set, pw)
	if err != nil {
		return Connection{}, err
	}
	cur, err := s.store.Connections.Get(ctx, id)
	if err != nil {
		return Connection{}, err
	}
	order := cur.SortOrder
	if set.GroupID != cur.GroupID {
		if order, err = s.nextOrder(ctx, set.GroupID); err != nil {
			return Connection{}, err
		}
	}
	state := PasswordState(cur.PasswordState)
	switch pw.Action {
	case PasswordSet:
		if err := s.secrets.Set(id, secrets.Password, pw.Value); err != nil {
			return Connection{}, fmt.Errorf("save password: %w", err)
		}
		state = PasswordSaved
	case PasswordClear:
		if err := s.secrets.Delete(id, secrets.Password); err != nil {
			return Connection{}, fmt.Errorf("remove password: %w", err)
		}
		state = PasswordNone
	}
	row, err := toStore(id, set, order, state)
	if err != nil {
		return Connection{}, err
	}
	updated, err := s.store.Connections.Update(ctx, row)
	if err != nil {
		return Connection{}, err
	}
	return fromStore(updated)
}

// Delete removes a connection and every secret it has. The secrets go first:
// if the keychain fails, nothing is deleted, and no password outlives its
// connection.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.store.Connections.Get(ctx, id); err != nil {
		return err
	}
	if err := s.secrets.DeleteAll(id); err != nil {
		return fmt.Errorf("remove secrets of connection %s: %w", id, err)
	}
	return s.store.Connections.Delete(ctx, id)
}

// Duplicate copies a connection, its secrets included, under a new name and
// places the copy right after the original. The frontend picks the name, so
// it can be translated.
func (s *Service) Duplicate(ctx context.Context, id, name string) (Connection, error) {
	src, err := s.Get(ctx, id)
	if err != nil {
		return Connection{}, err
	}
	set := src.Settings
	set.Name = name
	set = normalize(set)
	if err := validate(set); err != nil {
		return Connection{}, err
	}
	row, err := toStore("", set, src.SortOrder, src.PasswordState)
	if err != nil {
		return Connection{}, err
	}
	created, err := s.store.Connections.Create(ctx, row)
	if err != nil {
		return Connection{}, err
	}
	undo := func() {
		_ = s.secrets.DeleteAll(created.ID)
		_ = s.store.Connections.Delete(ctx, created.ID)
	}
	if err := s.copySecrets(src.ID, created.ID); err != nil {
		undo()
		return Connection{}, err
	}

	// Place the copy after the original.
	ids, err := s.groupOrder(ctx, set.GroupID)
	if err != nil {
		undo()
		return Connection{}, err
	}
	ids = slices.DeleteFunc(ids, func(x string) bool { return x == created.ID })
	at := slices.Index(ids, src.ID) + 1
	if err := s.store.Connections.Arrange(ctx, set.GroupID, slices.Insert(ids, at, created.ID)); err != nil {
		undo()
		return Connection{}, err
	}
	return s.Get(ctx, created.ID)
}

func (s *Service) copySecrets(from, to string) error {
	for _, kind := range secrets.Kinds() {
		v, err := s.secrets.Get(from, kind)
		if errors.Is(err, secrets.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s to copy: %w", kind, err)
		}
		if err := s.secrets.Set(to, kind, v); err != nil {
			return fmt.Errorf("copy %s: %w", kind, err)
		}
	}
	return nil
}

// Arrange puts the connections ids, in that order, into group groupID (""
// for no group), e.g. after a drag and drop. Connections not listed keep
// their place.
func (s *Service) Arrange(ctx context.Context, groupID string, ids []string) error {
	if err := s.checkGroup(ctx, groupID); err != nil {
		return err
	}
	return s.store.Connections.Arrange(ctx, groupID, ids)
}

// Groups returns every group in order.
func (s *Service) Groups(ctx context.Context) ([]Group, error) {
	rows, err := s.store.Groups.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Group, 0, len(rows))
	for _, g := range rows {
		out = append(out, Group{ID: g.ID, Name: g.Name, SortOrder: g.SortOrder})
	}
	return out, nil
}

// CreateGroup adds a group at the end. Group names are unique, ignoring
// case, so an import can match groups by name.
func (s *Service) CreateGroup(ctx context.Context, name string) (Group, error) {
	groups, err := s.Groups(ctx)
	if err != nil {
		return Group{}, err
	}
	name, err = groupName(groups, "", name)
	if err != nil {
		return Group{}, err
	}
	order := 0
	if len(groups) > 0 {
		order = groups[len(groups)-1].SortOrder + 1
	}
	g, err := s.store.Groups.Create(ctx, store.Group{Name: name, SortOrder: order})
	if err != nil {
		return Group{}, err
	}
	return Group{ID: g.ID, Name: g.Name, SortOrder: g.SortOrder}, nil
}

// RenameGroup renames a group.
func (s *Service) RenameGroup(ctx context.Context, id, name string) (Group, error) {
	groups, err := s.Groups(ctx)
	if err != nil {
		return Group{}, err
	}
	i := slices.IndexFunc(groups, func(g Group) bool { return g.ID == id })
	if i < 0 {
		return Group{}, fmt.Errorf("group %s: %w", id, ErrNotFound)
	}
	if groups[i].Name, err = groupName(groups, id, name); err != nil {
		return Group{}, err
	}
	g := groups[i]
	if err := s.store.Groups.Update(ctx, store.Group{ID: g.ID, Name: g.Name, SortOrder: g.SortOrder}); err != nil {
		return Group{}, err
	}
	return g, nil
}

// DeleteGroup removes a group. Its connections stay, without a group.
func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	return s.store.Groups.Delete(ctx, id)
}

// ArrangeGroups sets the order of the groups.
func (s *Service) ArrangeGroups(ctx context.Context, ids []string) error {
	return s.store.Groups.Arrange(ctx, ids)
}

// groupName validates a new name for group id ("" for a new group).
func groupName(groups []Group, id, name string) (string, error) {
	name = normalize(Settings{Name: name}).Name
	if err := checkName("name", name); err != nil {
		return "", err
	}
	for _, g := range groups {
		if g.ID != id && sameName(g.Name, name) {
			return "", invalid("name", "a group named %q already exists", g.Name)
		}
	}
	return name, nil
}

// check normalizes and validates settings and a password change, and checks
// that the group exists.
func (s *Service) check(ctx context.Context, set Settings, pw PasswordChange) (Settings, error) {
	set = normalize(set)
	if err := validate(set); err != nil {
		return Settings{}, err
	}
	if err := checkPassword(pw); err != nil {
		return Settings{}, err
	}
	if err := s.checkGroup(ctx, set.GroupID); err != nil {
		return Settings{}, err
	}
	return set, nil
}

func (s *Service) checkGroup(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	groups, err := s.Groups(ctx)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(groups, func(g Group) bool { return g.ID == id }) {
		return invalid("groupId", "the group no longer exists")
	}
	return nil
}

// groupOrder returns the ids of a group's connections in order.
func (s *Service) groupOrder(ctx context.Context, groupID string) ([]string, error) {
	rows, err := s.store.Connections.List(ctx)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, c := range rows {
		if c.GroupID == groupID {
			ids = append(ids, c.ID)
		}
	}
	return ids, nil
}

// nextOrder is the sort order that puts a connection last in its group.
func (s *Service) nextOrder(ctx context.Context, groupID string) (int, error) {
	rows, err := s.store.Connections.List(ctx)
	if err != nil {
		return 0, err
	}
	order := 0
	for _, c := range rows {
		if c.GroupID == groupID && c.SortOrder >= order {
			order = c.SortOrder + 1
		}
	}
	return order, nil
}
