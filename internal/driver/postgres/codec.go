package postgres

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cokagung30/querra/internal/driver"
)

// codec formats values as PostgreSQL's own text output where it can. pgx's
// database/sql layer already hands most types over as that text (numeric,
// uuid, interval, json, arrays, enums, …); only bool, bytea, integers,
// floats, dates and timestamps arrive decoded and are formatted here.
type codec struct{}

func (codec) Column(ct *sql.ColumnType) driver.ResultColumn {
	// pgx reports built-in types by name ("INT8") and others by OID.
	typ := strings.ToLower(ct.DatabaseTypeName())
	return driver.ResultColumn{Name: ct.Name(), DatabaseType: typ, Kind: kindOf(typ)}
}

// kindOf maps a type name as pg_type.typname has it ("int8", "_text") to a
// Kind.
func kindOf(typname string) driver.Kind {
	if strings.HasPrefix(typname, "_") {
		return driver.KindArray
	}
	switch typname {
	case "int2", "int4", "int8", "oid", "xid", "cid":
		return driver.KindInteger
	case "numeric", "float4", "float8":
		return driver.KindDecimal
	case "bool":
		return driver.KindBool
	case "text", "varchar", "bpchar", "char", "name", "citext", "xml":
		return driver.KindText
	case "date":
		return driver.KindDate
	case "time", "timetz":
		return driver.KindTime
	case "timestamp", "timestamptz":
		return driver.KindDateTime
	case "json", "jsonb":
		return driver.KindJSON
	case "uuid":
		return driver.KindUUID
	case "bytea":
		return driver.KindBinary
	}
	return driver.KindOther
}

func (codec) Format(col driver.ResultColumn, v any) string {
	switch v := v.(type) {
	case string: // text from PostgreSQL, or "infinity" for dates and timestamps
		return v
	case []byte:
		if col.DatabaseType == "bytea" {
			return "0x" + hex.EncodeToString(v)
		}
		return string(v) // json, jsonb, xml
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		if col.DatabaseType == "float4" {
			return formatFloat(v, 32)
		}
		return formatFloat(v, 64)
	case bool:
		return strconv.FormatBool(v)
	case time.Time:
		return formatTime(col.DatabaseType, v)
	}
	return fmt.Sprint(v)
}

// formatFloat writes a float the way PostgreSQL 12+ does: the shortest text
// that reads back exactly, in exponent form below 1e-4 or from 1e15 (float8)
// or 1e6 (float4) up.
func formatFloat(v float64, bits int) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	}
	e := strconv.FormatFloat(v, 'e', -1, bits)
	exp, _ := strconv.Atoi(e[strings.IndexByte(e, 'e')+1:])
	limit := 15
	if bits == 32 {
		limit = 6
	}
	if exp < -4 || exp >= limit {
		return e
	}
	return strconv.FormatFloat(v, 'f', -1, bits)
}

// formatTime writes dates and timestamps as ISO 8601 with a space, as
// PostgreSQL does. pgx gives timestamp and date values in UTC and timestamptz
// values in local time, which is the time zone the grid shows them in.
func formatTime(typ string, t time.Time) string {
	var s string
	switch typ {
	case "date":
		s = t.Format("01-02")
	case "timestamptz":
		s = t.Format("01-02 15:04:05.999999") + utcOffset(t)
	default:
		s = t.Format("01-02 15:04:05.999999")
	}
	// Years before 1 AD are written as BC, as PostgreSQL does: year 0 is 1 BC.
	if y := t.Year(); y < 1 {
		return fmt.Sprintf("%04d-%s BC", 1-y, s)
	}
	return fmt.Sprintf("%04d-%s", t.Year(), s)
}

// utcOffset formats t's zone offset as PostgreSQL does: "+07", "+05:30", or
// "+07:07:12" for the local mean time of old dates.
func utcOffset(t time.Time) string {
	_, offset := t.Zone()
	switch {
	case offset%60 != 0:
		return t.Format("-07:00:00")
	case offset%3600 != 0:
		return t.Format("-07:00")
	}
	return t.Format("-07")
}
