//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cokagung30/querra/internal/driver"
)

// These tests need the querra_test database from `wails3 task db:reset`
// (dev/seed/postgres). They only read it.

func testConfig(t *testing.T) driver.Config {
	t.Helper()
	host := os.Getenv("QUERRA_DEV_POSTGRES_HOST")
	if host == "" {
		t.Skip("QUERRA_DEV_POSTGRES_HOST is not set (see dev/test.env.example)")
	}
	port, _ := strconv.Atoi(os.Getenv("QUERRA_DEV_POSTGRES_PORT"))
	return driver.Config{Host: host, Port: port, User: "querra_test", Password: "querra_test", Database: "querra_test"}
}

func openTest(t *testing.T) driver.Conn {
	t.Helper()
	c, err := Driver{}.Open(context.Background(), testConfig(t), nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestOpen(t *testing.T) {
	ctx := context.Background()
	c := openTest(t)
	var app string
	if err := c.Pool().QueryRowContext(ctx, "SHOW application_name").Scan(&app); err != nil || app != applicationName {
		t.Fatalf("application_name = %q, %v; want %q", app, err, applicationName)
	}

	// require must agree with what the server offers.
	var ssl string
	if err := c.Pool().QueryRowContext(ctx, "SHOW ssl").Scan(&ssl); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t)
	cfg.SSL.Mode = driver.SSLRequire
	tlsConn, err := Driver{}.Open(ctx, cfg, nil)
	if ssl == "on" && err != nil {
		t.Errorf("sslmode=require against a TLS server: %v", err)
	}
	if ssl == "off" && err == nil {
		t.Error("sslmode=require against a server without TLS succeeded")
	}
	if tlsConn != nil {
		_ = tlsConn.Close()
	}
}

func TestOpenWrongPasswordKeepsItOutOfTheError(t *testing.T) {
	cfg := testConfig(t)
	cfg.Password = "wrong-password-7f3a9c"
	c, err := Driver{}.Open(context.Background(), cfg, nil)
	if err == nil {
		_ = c.Close()
		t.Skip("the server trusts local connections, so any password works")
	}
	if strings.Contains(err.Error(), cfg.Password) {
		t.Fatalf("error %q contains the password", err)
	}
}

func TestOpenMarksLoginErrors(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	cfg.Database = "querra_test_no_such_db"
	if c, err := (Driver{}).Open(ctx, cfg, nil); !errors.Is(err, driver.ErrDatabaseNotFound) {
		if c != nil {
			_ = c.Close()
		}
		t.Errorf("Open(missing database) error = %v, want ErrDatabaseNotFound", err)
	}

	cfg = testConfig(t)
	cfg.Password = "wrong-password-7f3a9c"
	c, err := Driver{}.Open(ctx, cfg, nil)
	if err == nil {
		_ = c.Close()
		t.Skip("the server trusts local connections, so any password works")
	}
	if !errors.Is(err, driver.ErrAuthFailed) {
		t.Errorf("Open(wrong password) error = %v, want ErrAuthFailed", err)
	}
}

func TestServerInfo(t *testing.T) {
	ctx := context.Background()
	c := openTest(t)
	var version string
	if err := c.Pool().QueryRowContext(ctx, "SHOW server_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	info, err := c.ServerInfo(ctx)
	if err != nil || info.Version != version {
		t.Fatalf("ServerInfo() = %+v, %v; want version %q", info, err, version)
	}

	// Whatever sslmode=prefer chose, the server must agree.
	var ssl bool
	err = c.Pool().QueryRowContext(ctx,
		"SELECT ssl FROM pg_stat_ssl WHERE pid = pg_backend_pid()").Scan(&ssl)
	if err != nil {
		t.Fatal(err)
	}
	if info.TLS != ssl {
		t.Errorf("ServerInfo().TLS = %v, but pg_stat_ssl says %v", info.TLS, ssl)
	}

	cfg := testConfig(t)
	cfg.SSL.Mode = driver.SSLDisable
	plain, err := Driver{}.Open(ctx, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = plain.Close() }()
	if info, err := plain.ServerInfo(ctx); err != nil || info.TLS {
		t.Errorf("ServerInfo() with sslmode=disable = %+v, %v; want TLS false", info, err)
	}
}

func TestSessionSurvivesCancel(t *testing.T) {
	c := openTest(t)
	ctx := context.Background()
	s, err := c.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	pid := backendPID(t, s)
	cancelCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = s.Exec(cancelCtx, "SELECT pg_sleep(30)")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "57014" {
		t.Fatalf("cancelled pg_sleep error = %v, want SQLSTATE 57014 (query_canceled)", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("cancel took %v", elapsed)
	}
	// Same server session afterwards, so SET and transactions survive.
	if got := backendPID(t, s); got != pid {
		t.Fatalf("backend pid after cancel = %d, want %d (the session was replaced)", got, pid)
	}
}

func TestSessionKeepsItsState(t *testing.T) {
	c := openTest(t)
	ctx := context.Background()
	s, err := c.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if _, err := s.Exec(ctx, "SET search_path TO sales"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := queryRow(ctx, s, "SELECT count(*) FROM customers", &n); err != nil || n != 3 {
		t.Fatalf("customers via search_path = %d, %v; want 3", n, err)
	}
	// The shared pool never sees the session's SET.
	var path string
	if err := c.Pool().QueryRowContext(ctx, "SHOW search_path").Scan(&path); err != nil || strings.Contains(path, "sales") {
		t.Fatalf("pool search_path = %q, %v; the session's SET leaked", path, err)
	}

	if _, err := s.Exec(ctx, (Driver{}).Dialect().ReadOnlySessionSQL()); err != nil {
		t.Fatal(err)
	}
	_, err = s.Exec(ctx, "CREATE TEMP TABLE read_only_probe (a int)")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "25006" {
		t.Fatalf("write in a read-only session error = %v, want SQLSTATE 25006", err)
	}
}

func TestDatabasesAndSchemas(t *testing.T) {
	ctx := context.Background()
	in := openTest(t).Introspector()

	dbs, err := in.Databases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(dbs, driver.Database{Name: "querra_test"}) {
		t.Errorf("Databases() = %v, want querra_test among them", dbs)
	}

	schemas, err := in.Schemas(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []driver.Schema{
		{Name: "public"}, {Name: "sales"}, {Name: "pg_catalog", System: true}, {Name: "information_schema", System: true},
	} {
		if !slices.Contains(schemas, want) {
			t.Errorf("Schemas() = %v, want %+v among them", schemas, want)
		}
	}
	for _, s := range schemas {
		if strings.HasPrefix(s.Name, "pg_toast") || strings.HasPrefix(s.Name, "pg_temp_") {
			t.Errorf("Schemas() includes %s", s.Name)
		}
	}
}

func TestObjects(t *testing.T) {
	ctx := context.Background()
	in := openTest(t).Introspector()

	public, err := in.Objects(ctx, "public")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"all_types", "big_table", "no_pk", "Weird Table"} {
		if !slices.Contains(public, driver.Object{Name: name, Kind: driver.Table}) {
			t.Errorf("Objects(public) = %v, want table %q", public, name)
		}
	}

	sales, err := in.Objects(ctx, "sales")
	if err != nil {
		t.Fatal(err)
	}
	want := []driver.Object{
		{Name: "customer_order_counts", Kind: driver.MaterializedView},
		{Name: "customers", Kind: driver.Table},
		{Name: "order_count", Kind: driver.Function, Signature: "p_customer_id integer"},
		{Name: "order_items", Kind: driver.Table},
		{Name: "order_totals", Kind: driver.View},
		{Name: "orders", Kind: driver.Table},
		{Name: "products", Kind: driver.Table},
	}
	// The server sorts by its collation (en_US ignores "_"), so compare the
	// lists in byte order.
	byName := func(a, b driver.Object) int { return strings.Compare(a.Name, b.Name) }
	slices.SortFunc(sales, byName)
	if !slices.Equal(sales, want) {
		t.Errorf("Objects(sales) =\n%v\nwant\n%v", sales, want)
	}

	if none, err := in.Objects(ctx, "no_such_schema"); err != nil || len(none) != 0 || none == nil {
		t.Errorf("Objects(no_such_schema) = %#v, %v; want an empty, non-nil list", none, err)
	}
}

func TestTableWithCompositeKeyAndForeignKeys(t *testing.T) {
	info, err := openTest(t).Introspector().Table(context.Background(), driver.TableRef{Schema: "sales", Name: "order_items"})
	if err != nil {
		t.Fatal(err)
	}
	if info.Kind != driver.Table || !slices.Equal(info.PrimaryKey, []string{"order_id", "product_id"}) {
		t.Errorf("kind %s, primary key %v", info.Kind, info.PrimaryKey)
	}
	wantCols := []driver.TableColumn{
		{Name: "order_id", Type: "integer", Kind: driver.KindInteger, PrimaryKey: true},
		{Name: "product_id", Type: "integer", Kind: driver.KindInteger, PrimaryKey: true},
		{Name: "quantity", Type: "integer", Kind: driver.KindInteger},
	}
	if !slices.Equal(info.Columns, wantCols) {
		t.Errorf("Columns =\n%+v\nwant\n%+v", info.Columns, wantCols)
	}

	if len(info.ForeignKeys) != 2 {
		t.Fatalf("ForeignKeys = %+v, want 2", info.ForeignKeys)
	}
	orders, products := info.ForeignKeys[0], info.ForeignKeys[1]
	if orders.Name != "order_items_order_id_fkey" || !slices.Equal(orders.Columns, []string{"order_id"}) ||
		orders.RefTable != (driver.TableRef{Schema: "sales", Name: "orders"}) || !slices.Equal(orders.RefColumns, []string{"id"}) ||
		orders.OnDelete != "CASCADE" || orders.OnUpdate != "NO ACTION" {
		t.Errorf("FK to orders = %+v", orders)
	}
	if products.RefTable != (driver.TableRef{Schema: "sales", Name: "products"}) || products.OnDelete != "NO ACTION" {
		t.Errorf("FK to products = %+v", products)
	}

	if len(info.Constraints) != 1 || info.Constraints[0].Kind != driver.Check ||
		!slices.Equal(info.Constraints[0].Columns, []string{"quantity"}) || info.Constraints[0].Definition != "CHECK ((quantity > 0))" {
		t.Errorf("Constraints = %+v, want the quantity check", info.Constraints)
	}
	if len(info.Indexes) != 1 || info.Indexes[0].Name != "order_items_pkey" || !info.Indexes[0].Primary ||
		!info.Indexes[0].Unique || info.Indexes[0].Method != "btree" || !slices.Equal(info.Indexes[0].Columns, []string{"order_id", "product_id"}) {
		t.Errorf("Indexes = %+v, want the primary key index", info.Indexes)
	}
}

func TestTableDefaultsUniqueAndQuotedNames(t *testing.T) {
	ctx := context.Background()
	in := openTest(t).Introspector()

	customers, err := in.Table(ctx, driver.TableRef{Schema: "sales", Name: "customers"})
	if err != nil {
		t.Fatal(err)
	}
	id, name, email := customers.Columns[0], customers.Columns[1], customers.Columns[2]
	if id.Default != "nextval('sales.customers_id_seq'::regclass)" || id.Nullable || !id.PrimaryKey {
		t.Errorf("id = %+v", id)
	}
	if name.Nullable || !email.Nullable {
		t.Errorf("nullable: name %v, email %v; want false, true", name.Nullable, email.Nullable)
	}
	if !slices.ContainsFunc(customers.Constraints, func(c driver.Constraint) bool {
		return c.Kind == driver.Unique && slices.Equal(c.Columns, []string{"email"})
	}) {
		t.Errorf("Constraints = %+v, want UNIQUE (email)", customers.Constraints)
	}

	weird, err := in.Table(ctx, driver.TableRef{Schema: "public", Name: "Weird Table"})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range weird.Columns {
		names = append(names, c.Name)
	}
	if want := []string{"ID", "Mixed Case", "select", "spasi kolom", "ünïcödé", "dot.name"}; !slices.Equal(names, want) {
		t.Errorf("Weird Table columns = %q, want %q", names, want)
	}

	big, err := in.Table(ctx, driver.TableRef{Schema: "public", Name: "big_table"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(big.Indexes, func(ix driver.Index) bool {
		return ix.Name == "big_table_category_idx" && !ix.Unique && slices.Equal(ix.Columns, []string{"category"})
	}) {
		t.Errorf("big_table indexes = %+v, want big_table_category_idx (category)", big.Indexes)
	}
}

func TestTableKinds(t *testing.T) {
	ctx := context.Background()
	in := openTest(t).Introspector()

	info, err := in.Table(ctx, driver.TableRef{Schema: "public", Name: "all_types"})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]driver.Kind{}
	types := map[string]string{}
	for _, c := range info.Columns {
		kinds[c.Name], types[c.Name] = c.Kind, c.Type
	}
	for col, want := range map[string]driver.Kind{
		"c_bigint": driver.KindInteger, "c_numeric": driver.KindDecimal, "c_double": driver.KindDecimal,
		"c_boolean": driver.KindBool, "c_char": driver.KindText, "c_varchar": driver.KindText,
		"c_bytea": driver.KindBinary, "c_date": driver.KindDate, "c_timetz": driver.KindTime,
		"c_timestamptz": driver.KindDateTime, "c_uuid": driver.KindUUID, "c_jsonb": driver.KindJSON,
		"c_int_array": driver.KindArray, "c_mood": driver.KindText, "c_interval": driver.KindOther,
	} {
		if kinds[col] != want {
			t.Errorf("%s kind = %q, want %q", col, kinds[col], want)
		}
	}
	if types["c_numeric"] != "numeric(30,10)" || types["c_int_array"] != "integer[]" || types["c_timestamptz"] != "timestamp with time zone" {
		t.Errorf("types = %v", types)
	}

	view, err := in.Table(ctx, driver.TableRef{Schema: "sales", Name: "order_totals"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Kind != driver.View || len(view.Columns) != 3 || len(view.PrimaryKey) != 0 || view.PrimaryKey == nil {
		t.Errorf("order_totals = kind %s, %d columns, primary key %#v", view.Kind, len(view.Columns), view.PrimaryKey)
	}

	if _, err := in.Table(ctx, driver.TableRef{Schema: "public", Name: "no_such_table"}); !errors.Is(err, driver.ErrNotFound) {
		t.Errorf("Table(no_such_table) error = %v, want ErrNotFound", err)
	}
}

func TestEstimateRows(t *testing.T) {
	ctx := context.Background()
	in := openTest(t).Introspector()
	n, err := in.EstimateRows(ctx, driver.TableRef{Schema: "public", Name: "big_table"})
	if err != nil || n < 900_000 || n > 1_100_000 {
		t.Errorf("EstimateRows(big_table) = %d, %v; want about 1,000,000 (the seed runs ANALYZE)", n, err)
	}
	if n, err := in.EstimateRows(ctx, driver.TableRef{Schema: "sales", Name: "order_totals"}); err != nil || n != -1 {
		t.Errorf("EstimateRows(view) = %d, %v; want -1", n, err)
	}
	if _, err := in.EstimateRows(ctx, driver.TableRef{Schema: "public", Name: "no_such_table"}); !errors.Is(err, driver.ErrNotFound) {
		t.Errorf("EstimateRows(no_such_table) error = %v, want ErrNotFound", err)
	}
}

// TestValues checks every all_types cell against PostgreSQL's own text,
// except bool and bytea, which follow PLAN §3.5.
func TestValues(t *testing.T) {
	c := openTest(t)
	rows, err := c.Pool().QueryContext(context.Background(), "SELECT * FROM all_types ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()

	codec := Driver{}.Codec()
	cols, err := driver.Columns(rows, codec)
	if err != nil {
		t.Fatal(err)
	}
	var table []map[string]*string
	for rows.Next() {
		cells, err := driver.ScanRow(rows, codec, cols)
		if err != nil {
			t.Fatal(err)
		}
		row := map[string]*string{}
		for i, col := range cols {
			row[col.Name] = cells[i]
		}
		table = append(table, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(table) != 3 {
		t.Fatalf("got %d rows, want 3", len(table))
	}

	kinds := map[string]driver.Kind{}
	for _, col := range cols {
		kinds[col.Name] = col.Kind
	}
	for col, want := range map[string]driver.Kind{
		"c_bigint": driver.KindInteger, "c_numeric": driver.KindDecimal, "c_real": driver.KindDecimal,
		"c_boolean": driver.KindBool, "c_char": driver.KindText, "c_bytea": driver.KindBinary,
		"c_date": driver.KindDate, "c_time": driver.KindTime, "c_timestamptz": driver.KindDateTime,
		"c_json": driver.KindJSON, "c_uuid": driver.KindUUID, "c_int_array": driver.KindArray,
	} {
		if kinds[col] != want {
			t.Errorf("result column %s kind = %q, want %q", col, kinds[col], want)
		}
	}

	checkRow(t, "upper extremes", table[0], map[string]string{
		"c_smallint":   "32767",
		"c_integer":    "2147483647",
		"c_bigint":     "9223372036854775807",
		"c_numeric":    "12345678901234567890.0123456789",
		"c_real":       "3.4e+38",
		"c_double":     "1.7976931348623157e+308",
		"c_boolean":    "true",
		"c_char":       "char      ",
		"c_varchar":    "varchar",
		"c_text":       "multi-line\ntext with 'quotes' and unicode: ünïcödé 日本語 🚀",
		"c_bytea":      "0xdeadbeef",
		"c_date":       "2026-11-07",
		"c_time":       "23:59:59.999999",
		"c_timetz":     "12:00:00+07",
		"c_timestamp":  "2026-11-07 10:00:00.123456",
		"c_interval":   "1 year 2 mons 3 days 04:05:06",
		"c_uuid":       "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
		"c_json":       `{"a": 1, "big": 12345678901234567890}`,
		"c_jsonb":      `{"nested": {"list": [1, 2, 3]}}`,
		"c_inet":       "192.168.1.1",
		"c_cidr":       "10.0.0.0/8",
		"c_macaddr":    "08:00:2b:01:02:03",
		"c_int_array":  "{1,2,3}",
		"c_text_array": `{a,"b c",NULL}`,
		"c_mood":       "happy",
		"c_point":      "(1.5,2.5)",
		"c_xml":        "<root><child>x</child></root>",
		"c_tsvector":   "'brown' 'fox' 'quick' 'the'",
	})
	checkRow(t, "lower extremes", table[1], map[string]string{
		"c_smallint":  "-32768",
		"c_bigint":    "-9223372036854775808",
		"c_numeric":   "-0.0000000001",
		"c_real":      "-1.5",
		"c_boolean":   "false",
		"c_char":      "          ",
		"c_varchar":   "", // empty, not NULL
		"c_bytea":     "0x",
		"c_date":      "0001-01-01",
		"c_timestamp": "1970-01-01 00:00:00",
		"c_interval":  "-1 days",
		"c_json":      "[]",
		"c_int_array": "{}",
		"c_tsvector":  "",
	})

	// timestamptz is shown in the local time zone; check the instant.
	got, err := time.Parse("2006-01-02 15:04:05.999999-07", *table[0]["c_timestamptz"])
	if err != nil {
		got, err = time.Parse("2006-01-02 15:04:05.999999-07:00", *table[0]["c_timestamptz"])
	}
	if want := time.Date(2026, 11, 7, 3, 0, 0, 123456000, time.UTC); err != nil || !got.Equal(want) {
		t.Errorf("c_timestamptz = %q (%v), want the instant %v", *table[0]["c_timestamptz"], err, want)
	}
	if table[0]["c_money"] == nil {
		t.Error("c_money is NULL") // its text depends on lc_monetary, so only presence is checked
	}

	for name, cell := range table[2] {
		if name != "id" && cell != nil {
			t.Errorf("all-NULL row: %s = %q, want NULL", name, *cell)
		}
	}
}

func checkRow(t *testing.T, label string, row map[string]*string, want map[string]string) {
	t.Helper()
	for col, w := range want {
		switch got := row[col]; {
		case got == nil:
			t.Errorf("%s: %s = NULL, want %q", label, col, w)
		case *got != w:
			t.Errorf("%s: %s = %q, want %q", label, col, *got, w)
		}
	}
}

func backendPID(t *testing.T, s driver.Session) int {
	t.Helper()
	var pid int
	if err := queryRow(context.Background(), s, "SELECT pg_backend_pid()", &pid); err != nil {
		t.Fatal(err)
	}
	return pid
}

// queryRow scans the first row of a session query into dest.
func queryRow(ctx context.Context, s driver.Session, stmt string, dest ...any) error {
	rows, err := s.Query(ctx, stmt)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return errors.New("no rows")
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	return rows.Close()
}
