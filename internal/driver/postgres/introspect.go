package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cokagung30/querra/internal/driver"
)

// introspector reads pg_catalog. Names are always bind parameters, never
// quoted into the SQL.
type introspector struct{ db *sql.DB }

func (in introspector) Databases(ctx context.Context) ([]driver.Database, error) {
	dbs, err := queryList(ctx, in.db, func(rows *sql.Rows) (d driver.Database, err error) {
		return d, rows.Scan(&d.Name)
	}, `SELECT datname FROM pg_database WHERE datallowconn AND NOT datistemplate ORDER BY datname`)
	if err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}
	return dbs, nil
}

// Schemas leaves out TOAST and temporary-table schemas, which hold nothing to
// browse, and marks pg_catalog and information_schema as system schemas.
func (in introspector) Schemas(ctx context.Context) ([]driver.Schema, error) {
	schemas, err := queryList(ctx, in.db, func(rows *sql.Rows) (s driver.Schema, err error) {
		return s, rows.Scan(&s.Name, &s.System)
	}, `SELECT nspname, nspname IN ('pg_catalog', 'information_schema')
		FROM pg_namespace
		WHERE nspname NOT LIKE 'pg\_toast%' AND nspname NOT LIKE 'pg\_temp\_%'
		ORDER BY 2, 1`)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	return schemas, nil
}

// Objects lists tables (including partitioned and foreign tables, but not
// partitions, which are reached through their parent), views, materialized
// views, functions and procedures. Aggregates and window functions are left
// out.
func (in introspector) Objects(ctx context.Context, schema string) ([]driver.Object, error) {
	objects, err := queryList(ctx, in.db, func(rows *sql.Rows) (o driver.Object, err error) {
		return o, rows.Scan(&o.Name, &o.Kind, &o.Signature)
	}, `SELECT c.relname,
			CASE c.relkind WHEN 'v' THEN 'view' WHEN 'm' THEN 'materialized_view' ELSE 'table' END,
			''
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind IN ('r', 'p', 'f', 'v', 'm') AND NOT c.relispartition
		UNION ALL
		SELECT p.proname,
			CASE p.prokind WHEN 'p' THEN 'procedure' ELSE 'function' END,
			pg_get_function_identity_arguments(p.oid)
		FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = $1 AND p.prokind IN ('f', 'p')
		ORDER BY 1, 3`, schema)
	if err != nil {
		return nil, fmt.Errorf("list objects in %s: %w", schema, err)
	}
	return objects, nil
}

func (in introspector) Table(ctx context.Context, ref driver.TableRef) (*driver.TableInfo, error) {
	info, err := in.table(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("describe %s.%s: %w", ref.Schema, ref.Name, err)
	}
	return info, nil
}

func (in introspector) table(ctx context.Context, ref driver.TableRef) (*driver.TableInfo, error) {
	var (
		oid     int64
		relkind string
		comment sql.NullString
	)
	err := in.db.QueryRowContext(ctx, `SELECT c.oid, c.relkind, obj_description(c.oid, 'pg_class')
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2 AND c.relkind IN ('r', 'p', 'f', 'v', 'm')`,
		ref.Schema, ref.Name).Scan(&oid, &relkind, &comment)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, driver.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	info := &driver.TableInfo{Ref: ref, Kind: relationKind(relkind), Comment: comment.String}
	if info.Columns, err = in.columns(ctx, oid); err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}
	if info.Indexes, err = in.indexes(ctx, oid); err != nil {
		return nil, fmt.Errorf("indexes: %w", err)
	}
	if err := in.constraints(ctx, oid, info); err != nil {
		return nil, fmt.Errorf("constraints: %w", err)
	}
	for i, col := range info.Columns {
		for _, pk := range info.PrimaryKey {
			if col.Name == pk {
				info.Columns[i].PrimaryKey = true
			}
		}
	}
	return info, nil
}

func relationKind(relkind string) driver.ObjectKind {
	switch relkind {
	case "v":
		return driver.View
	case "m":
		return driver.MaterializedView
	}
	return driver.Table
}

func (in introspector) columns(ctx context.Context, oid int64) ([]driver.TableColumn, error) {
	return queryList(ctx, in.db, func(rows *sql.Rows) (c driver.TableColumn, err error) {
		var typname, typtype string
		var baseTypname, def, comment sql.NullString
		err = rows.Scan(&c.Name, &c.Type, &typname, &typtype, &baseTypname, &c.Nullable, &def, &comment)
		c.Kind = columnKind(typname, typtype, baseTypname.String)
		c.Default, c.Comment = def.String, comment.String
		return c, err
	}, `SELECT a.attname, format_type(a.atttypid, a.atttypmod), t.typname, t.typtype, bt.typname,
			NOT a.attnotnull, pg_get_expr(d.adbin, d.adrelid), col_description(a.attrelid, a.attnum)
		FROM pg_attribute a
		JOIN pg_type t ON t.oid = a.atttypid
		LEFT JOIN pg_type bt ON t.typtype = 'd' AND bt.oid = t.typbasetype
		LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		WHERE a.attrelid = $1 AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`, oid)
}

// columnKind maps a column's type to a Kind, looking through domains to their
// base type. Enum values are shown as text.
func columnKind(typname, typtype, baseTypname string) driver.Kind {
	switch {
	case typtype == "d" && baseTypname != "":
		return kindOf(baseTypname)
	case typtype == "e":
		return driver.KindText
	}
	return kindOf(typname)
}

func (in introspector) indexes(ctx context.Context, oid int64) ([]driver.Index, error) {
	return queryList(ctx, in.db, func(rows *sql.Rows) (ix driver.Index, err error) {
		var columns []byte
		if err = rows.Scan(&ix.Name, &ix.Unique, &ix.Primary, &ix.Method, &ix.Definition, &columns); err != nil {
			return ix, err
		}
		return ix, json.Unmarshal(columns, &ix.Columns)
	}, `SELECT ic.relname, i.indisunique, i.indisprimary, am.amname, pg_get_indexdef(i.indexrelid),
			COALESCE((SELECT json_agg(pg_get_indexdef(i.indexrelid, k, true) ORDER BY k)
				FROM generate_series(1, i.indnkeyatts) AS k), '[]')
		FROM pg_index i
		JOIN pg_class ic ON ic.oid = i.indexrelid
		JOIN pg_am am ON am.oid = ic.relam
		WHERE i.indrelid = $1
		ORDER BY NOT i.indisprimary, ic.relname`, oid)
}

// constraints fills the primary key, foreign keys and other constraints.
func (in introspector) constraints(ctx context.Context, oid int64, info *driver.TableInfo) error {
	type row struct {
		name, contype, definition string
		columns, refColumns       []string
		refSchema, refTable       sql.NullString
		onUpdate, onDelete        string
	}
	rows, err := queryList(ctx, in.db, func(rows *sql.Rows) (r row, err error) {
		var columns, refColumns []byte
		err = rows.Scan(&r.name, &r.contype, &columns, &r.refSchema, &r.refTable, &refColumns,
			&r.onUpdate, &r.onDelete, &r.definition)
		if err == nil {
			err = errors.Join(json.Unmarshal(columns, &r.columns), json.Unmarshal(refColumns, &r.refColumns))
		}
		return r, err
	}, `SELECT con.conname, con.contype,
			COALESCE((SELECT json_agg(a.attname ORDER BY k.ord)
				FROM unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord)
				JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum), '[]'),
			rn.nspname, rc.relname,
			COALESCE((SELECT json_agg(a.attname ORDER BY k.ord)
				FROM unnest(con.confkey) WITH ORDINALITY AS k(attnum, ord)
				JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.attnum), '[]'),
			con.confupdtype, con.confdeltype, pg_get_constraintdef(con.oid)
		FROM pg_constraint con
		LEFT JOIN pg_class rc ON rc.oid = con.confrelid
		LEFT JOIN pg_namespace rn ON rn.oid = rc.relnamespace
		WHERE con.conrelid = $1 AND con.contype IN ('p', 'f', 'u', 'c', 'x')
		ORDER BY con.conname`, oid)
	if err != nil {
		return err
	}

	info.PrimaryKey, info.ForeignKeys, info.Constraints = []string{}, []driver.ForeignKey{}, []driver.Constraint{}
	for _, r := range rows {
		switch r.contype {
		case "p":
			info.PrimaryKey = r.columns
		case "f":
			info.ForeignKeys = append(info.ForeignKeys, driver.ForeignKey{
				Name:       r.name,
				Columns:    r.columns,
				RefTable:   driver.TableRef{Schema: r.refSchema.String, Name: r.refTable.String},
				RefColumns: r.refColumns,
				OnUpdate:   referentialAction(r.onUpdate),
				OnDelete:   referentialAction(r.onDelete),
			})
		default:
			kind := map[string]driver.ConstraintKind{"u": driver.Unique, "c": driver.Check, "x": driver.Exclusion}[r.contype]
			info.Constraints = append(info.Constraints, driver.Constraint{
				Name: r.name, Kind: kind, Columns: r.columns, Definition: r.definition,
			})
		}
	}
	return nil
}

// referentialAction spells out pg_constraint's confupdtype/confdeltype codes.
func referentialAction(code string) string {
	switch code {
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	}
	return "NO ACTION"
}

// EstimateRows reads pg_class.reltuples, which ANALYZE and VACUUM keep up to
// date. PostgreSQL 14+ stores -1 for a table that has never been analyzed;
// views have no estimate either.
func (in introspector) EstimateRows(ctx context.Context, ref driver.TableRef) (int64, error) {
	var estimate float64
	err := in.db.QueryRowContext(ctx, `SELECT CASE WHEN c.relkind = 'v' THEN -1 ELSE c.reltuples END
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2 AND c.relkind IN ('r', 'p', 'f', 'v', 'm')`,
		ref.Schema, ref.Name).Scan(&estimate)
	if errors.Is(err, sql.ErrNoRows) {
		err = driver.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("estimate rows of %s.%s: %w", ref.Schema, ref.Name, err)
	}
	if estimate < 0 {
		return -1, nil
	}
	return int64(estimate), nil
}

// queryList runs a query and scans every row with scan. It never returns a
// nil slice.
func queryList[T any](ctx context.Context, db *sql.DB, scan func(*sql.Rows) (T, error), query string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	list := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, rows.Err()
}
