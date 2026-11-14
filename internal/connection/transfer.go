package connection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/cokagung30/querra/internal/driver"
)

// ErrInvalidFile means an import file cannot be used: it is not a Querra
// connections file, comes from a newer Querra, or has invalid settings.
var ErrInvalidFile = errors.New("invalid connections file")

// The export format (FR-05, PLAN §3.6). It never holds a secret:
// passwordRequired only says that the connection had a password, so the
// importer asks for it.
const (
	exportFormat  = "querra-connections"
	exportVersion = 1
	// maxImportBytes bounds what Import reads; a real file is a few KB.
	maxImportBytes = 10 << 20
)

type exportFile struct {
	Format      string             `json:"format"`
	Version     int                `json:"version"`
	Groups      []exportGroup      `json:"groups"`
	Connections []exportConnection `json:"connections"`
}

type exportGroup struct {
	Name string `json:"name"`
}

type exportConnection struct {
	// Group is the group's name; "" means none.
	Group            string        `json:"group,omitempty"`
	Name             string        `json:"name"`
	Engine           driver.Engine `json:"engine"`
	Env              Env           `json:"env"`
	Color            Color         `json:"color,omitempty"`
	Host             string        `json:"host,omitempty"`
	Port             int           `json:"port,omitempty"`
	Username         string        `json:"username,omitempty"`
	Database         string        `json:"database,omitempty"`
	SSL              SSL           `json:"ssl"`
	ReadOnly         bool          `json:"readOnly"`
	PasswordRequired bool          `json:"passwordRequired"`
}

// Export writes the connections ids (all of them when ids is empty), with
// their groups, as JSON. Exporting everything also keeps empty groups.
func (s *Service) Export(ctx context.Context, ids []string) ([]byte, error) {
	conns, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	groups, err := s.Groups(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		for _, id := range ids {
			if !slices.ContainsFunc(conns, func(c Connection) bool { return c.ID == id }) {
				return nil, fmt.Errorf("export connection %s: %w", id, ErrNotFound)
			}
		}
		conns = slices.DeleteFunc(conns, func(c Connection) bool { return !slices.Contains(ids, c.ID) })
	}

	names := map[string]string{} // group id → name
	for _, g := range groups {
		names[g.ID] = g.Name
	}
	file := exportFile{Format: exportFormat, Version: exportVersion, Groups: []exportGroup{}, Connections: []exportConnection{}}
	for _, g := range groups {
		used := slices.ContainsFunc(conns, func(c Connection) bool { return c.Settings.GroupID == g.ID })
		if used || len(ids) == 0 {
			file.Groups = append(file.Groups, exportGroup{Name: g.Name})
		}
	}
	// Connections follow their groups' order, ungrouped ones last, as in the
	// sidebar.
	rank := func(c Connection) int {
		i := slices.IndexFunc(groups, func(g Group) bool { return g.ID == c.Settings.GroupID })
		if i < 0 {
			return len(groups)
		}
		return i
	}
	slices.SortStableFunc(conns, func(a, b Connection) int { return rank(a) - rank(b) })
	for _, c := range conns {
		set := c.Settings
		file.Connections = append(file.Connections, exportConnection{
			Group:            names[set.GroupID],
			Name:             set.Name,
			Engine:           set.Engine,
			Env:              set.Env,
			Color:            set.Color,
			Host:             set.Host,
			Port:             set.Port,
			Username:         set.Username,
			Database:         set.Database,
			SSL:              set.SSL,
			ReadOnly:         set.ReadOnly,
			PasswordRequired: c.PasswordState != PasswordNone,
		})
	}

	out, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode export: %w", err)
	}
	return append(out, '\n'), nil
}

// ImportResult counts what Import added.
type ImportResult struct {
	Connections int `json:"connections"`
	Groups      int `json:"groups"`
}

// Import adds every connection in an export file as a new connection, at the
// end of its group. Groups are matched by name, ignoring case, and created
// when missing. Nothing existing is changed or replaced. A connection that had
// a password comes in as PasswordRequired.
//
// The whole file is checked before anything is added. A store failure
// halfway leaves the connections added so far.
func (s *Service) Import(ctx context.Context, data []byte) (ImportResult, error) {
	file, err := parseExport(data)
	if err != nil {
		return ImportResult{}, err
	}

	type pending struct {
		set   Settings
		group string
		state PasswordState
	}
	for _, g := range file.Groups {
		if err := checkName("name", normalize(Settings{Name: g.Name}).Name); err != nil {
			return ImportResult{}, fmt.Errorf("%w: group %q: %w", ErrInvalidFile, g.Name, err)
		}
	}
	var todo []pending
	for i, ec := range file.Connections {
		set := normalize(Settings{
			Name:     ec.Name,
			Engine:   ec.Engine,
			Env:      ec.Env,
			Color:    ec.Color,
			Host:     ec.Host,
			Port:     ec.Port,
			Username: ec.Username,
			Database: ec.Database,
			SSL:      ec.SSL,
			ReadOnly: ec.ReadOnly,
		})
		if err := validate(set); err != nil {
			return ImportResult{}, fmt.Errorf("%w: connection %d (%q): %w", ErrInvalidFile, i+1, ec.Name, err)
		}
		if group := normalize(Settings{Name: ec.Group}).Name; group != "" {
			if err := checkName("group", group); err != nil {
				return ImportResult{}, fmt.Errorf("%w: connection %d (%q): %w", ErrInvalidFile, i+1, ec.Name, err)
			}
		}
		state := PasswordNone
		if ec.PasswordRequired {
			state = PasswordRequired
		}
		todo = append(todo, pending{set: set, group: ec.Group, state: state})
	}

	var res ImportResult
	groups, err := s.Groups(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	groupID := func(name string) (string, error) {
		name = normalize(Settings{Name: name}).Name
		if name == "" {
			return "", nil
		}
		if i := slices.IndexFunc(groups, func(g Group) bool { return sameName(g.Name, name) }); i >= 0 {
			return groups[i].ID, nil
		}
		g, err := s.CreateGroup(ctx, name)
		if err != nil {
			return "", err
		}
		groups = append(groups, g)
		res.Groups++
		return g.ID, nil
	}
	for _, g := range file.Groups {
		if _, err := groupID(g.Name); err != nil {
			return res, fmt.Errorf("import group %q: %w", g.Name, err)
		}
	}
	for _, p := range todo {
		if p.set.GroupID, err = groupID(p.group); err != nil {
			return res, fmt.Errorf("import group %q: %w", p.group, err)
		}
		order, err := s.nextOrder(ctx, p.set.GroupID)
		if err != nil {
			return res, err
		}
		row, err := toStore("", p.set, order, p.state)
		if err != nil {
			return res, err
		}
		if _, err := s.store.Connections.Create(ctx, row); err != nil {
			return res, fmt.Errorf("import connection %q: %w", p.set.Name, err)
		}
		res.Connections++
	}
	return res, nil
}

func parseExport(data []byte) (exportFile, error) {
	if len(data) > maxImportBytes {
		return exportFile{}, fmt.Errorf("%w: the file is larger than %d MB", ErrInvalidFile, maxImportBytes>>20)
	}
	var head struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil || head.Format != exportFormat {
		return exportFile{}, fmt.Errorf("%w: not a Querra connections file", ErrInvalidFile)
	}
	if head.Version > exportVersion {
		return exportFile{}, fmt.Errorf("%w: made by a newer version of Querra (format version %d)", ErrInvalidFile, head.Version)
	}
	if head.Version < 1 {
		return exportFile{}, fmt.Errorf("%w: unknown format version %d", ErrInvalidFile, head.Version)
	}

	var file exportFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&file); err != nil {
		return exportFile{}, fmt.Errorf("%w: %w", ErrInvalidFile, err)
	}
	return file, nil
}
