package driver

// Introspection results. Slices are never nil, so they reach the frontend as
// [] rather than null.

type Database struct {
	Name string `json:"name"`
}

type Schema struct {
	Name string `json:"name"`
	// System marks the engine's own schemas, such as pg_catalog.
	System bool `json:"system"`
}

type ObjectKind string

const (
	Table            ObjectKind = "table"
	View             ObjectKind = "view"
	MaterializedView ObjectKind = "materialized_view"
	Function         ObjectKind = "function"
	Procedure        ObjectKind = "procedure"
)

type Object struct {
	Name string     `json:"name"`
	Kind ObjectKind `json:"kind"`
	// Signature tells overloaded routines apart, e.g. "p_customer_id
	// integer". It is empty for tables and views.
	Signature string `json:"signature"`
}

// TableRef names a table or view in the open database.
type TableRef struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
}

type TableInfo struct {
	Ref     TableRef      `json:"ref"`
	Kind    ObjectKind    `json:"kind"`
	Comment string        `json:"comment"`
	Columns []TableColumn `json:"columns"`
	// PrimaryKey holds column names in key order; empty when there is none.
	PrimaryKey  []string     `json:"primaryKey"`
	Indexes     []Index      `json:"indexes"`
	ForeignKeys []ForeignKey `json:"foreignKeys"`
	// Constraints holds unique, check and exclusion constraints. The primary
	// key and foreign keys have their own fields.
	Constraints []Constraint `json:"constraints"`
}

type TableColumn struct {
	Name string `json:"name"`
	// Type is the type as the database writes it, e.g. "numeric(12,2)".
	Type       string `json:"type"`
	Kind       Kind   `json:"kind"`
	Nullable   bool   `json:"nullable"`
	Default    string `json:"default"` // "" when there is none
	Comment    string `json:"comment"`
	PrimaryKey bool   `json:"primaryKey"`
}

type Index struct {
	Name string `json:"name"`
	// Columns holds key column names, or expressions, in key order.
	Columns    []string `json:"columns"`
	Unique     bool     `json:"unique"`
	Primary    bool     `json:"primary"`
	Method     string   `json:"method"`     // e.g. "btree"
	Definition string   `json:"definition"` // the full statement, including any predicate
}

type ForeignKey struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	RefTable   TableRef `json:"refTable"`
	RefColumns []string `json:"refColumns"`
	OnUpdate   string   `json:"onUpdate"` // e.g. "NO ACTION", "CASCADE"
	OnDelete   string   `json:"onDelete"`
}

type ConstraintKind string

const (
	Unique    ConstraintKind = "unique"
	Check     ConstraintKind = "check"
	Exclusion ConstraintKind = "exclusion"
)

type Constraint struct {
	Name       string         `json:"name"`
	Kind       ConstraintKind `json:"kind"`
	Columns    []string       `json:"columns"`
	Definition string         `json:"definition"` // e.g. "CHECK ((quantity > 0))"
}
