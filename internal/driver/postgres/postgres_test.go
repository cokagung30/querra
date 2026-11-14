package postgres

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cokagung30/querra/internal/driver"
)

func TestRegistered(t *testing.T) {
	d, err := driver.Lookup(driver.Postgres)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.(Driver); !ok {
		t.Fatalf("Lookup(postgres) = %T, want postgres.Driver", d)
	}
}

func TestMarkErr(t *testing.T) {
	refusedTLS := errors.New("server refused TLS connection")
	pgErr := func(code string) error { return &pgconn.PgError{Severity: "FATAL", Code: code, Message: "m"} }

	tests := []struct {
		name string
		err  error
		want []error
	}{
		{"wrong password", pgErr("28P01"), []error{driver.ErrAuthFailed}},
		{"no pg_hba entry", pgErr("28000"), []error{driver.ErrAuthFailed}},
		{"no database", pgErr("3D000"), []error{driver.ErrDatabaseNotFound}},
		{"no connect privilege", pgErr("42501"), []error{driver.ErrPermissionDenied}},
		{"TLS refused", refusedTLS, []error{driver.ErrTLSRefused}},
		{
			// sslmode=prefer: the TLS attempt was refused, the plain one failed to log in.
			"prefer fallback",
			fmt.Errorf("failed to connect: %w", errors.Join(refusedTLS, pgErr("28P01"))),
			[]error{driver.ErrAuthFailed, driver.ErrTLSRefused},
		},
		{"other server error", pgErr("53300"), nil},
		{"network error", &net.OpError{Op: "dial", Err: errors.New("boom")}, nil},
	}
	all := []error{driver.ErrAuthFailed, driver.ErrDatabaseNotFound, driver.ErrPermissionDenied, driver.ErrTLSRefused}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markErr(tt.err)
			if !errors.Is(got, tt.err) || got.Error() != tt.err.Error() {
				t.Fatalf("markErr() = %v, want it to wrap %v with the same message", got, tt.err)
			}
			for _, mark := range all {
				if want := slices.Contains(tt.want, mark); errors.Is(got, mark) != want {
					t.Errorf("errors.Is(markErr(), %v) = %v, want %v", mark, !want, want)
				}
			}
		})
	}
}

func TestConnConfigDefaults(t *testing.T) {
	t.Setenv("PGPASSWORD", "from-the-environment")
	cfg, err := connConfig(driver.Config{Host: "db.example.com", User: "app", Database: "shop"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != defaultPort || cfg.Host != "db.example.com" || cfg.User != "app" || cfg.Database != "shop" {
		t.Errorf("target = %s:%d user %s db %s", cfg.Host, cfg.Port, cfg.User, cfg.Database)
	}
	if cfg.Password != "" {
		t.Errorf("Password = %q; only the saved password may be used, not PGPASSWORD", cfg.Password)
	}
	if got := cfg.RuntimeParams["application_name"]; got != applicationName {
		t.Errorf("application_name = %q, want %q", got, applicationName)
	}
	if cfg.DefaultQueryExecMode != pgx.QueryExecModeDescribeExec {
		t.Errorf("DefaultQueryExecMode = %v, want DescribeExec", cfg.DefaultQueryExecMode)
	}
	if cfg.ConnectTimeout != connectTimeout || cfg.BuildContextWatcherHandler == nil {
		t.Errorf("ConnectTimeout = %v, BuildContextWatcherHandler set = %v", cfg.ConnectTimeout, cfg.BuildContextWatcherHandler != nil)
	}
	// prefer: TLS first without verification, then a plain fallback.
	if cfg.TLSConfig == nil || !cfg.TLSConfig.InsecureSkipVerify || len(cfg.Fallbacks) == 0 || cfg.Fallbacks[len(cfg.Fallbacks)-1].TLSConfig != nil {
		t.Errorf("prefer: TLSConfig = %+v, fallbacks = %d", cfg.TLSConfig, len(cfg.Fallbacks))
	}
}

func TestConnConfigQuotesValues(t *testing.T) {
	in := driver.Config{
		Host: "::1", Port: 6543, User: `o'brien \ x`, Password: `pa'ss\word`, Database: "my db",
		SSL: driver.SSL{Mode: driver.SSLDisable},
	}
	cfg, err := connConfig(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != in.Host || cfg.Port != 6543 || cfg.User != in.User || cfg.Password != in.Password || cfg.Database != in.Database {
		t.Errorf("got %s:%d user %q password %q db %q", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database)
	}
	if cfg.TLSConfig != nil || len(cfg.Fallbacks) != 0 {
		t.Errorf("disable: TLSConfig = %+v, fallbacks = %d; want plain only", cfg.TLSConfig, len(cfg.Fallbacks))
	}
}

func TestConnConfigSSLModes(t *testing.T) {
	// A CA file in a folder whose name needs quoting.
	caFile := writeCA(t, filepath.Join(t.TempDir(), "Bob's certs", "ca.pem"))

	require, err := connConfig(driver.Config{Host: "db", SSL: driver.SSL{Mode: driver.SSLRequire}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if require.TLSConfig == nil || !require.TLSConfig.InsecureSkipVerify || len(require.Fallbacks) != 0 {
		t.Errorf("require: TLSConfig = %+v, fallbacks = %d; want unverified TLS, no plain fallback", require.TLSConfig, len(require.Fallbacks))
	}

	full, err := connConfig(driver.Config{Host: "db.example.com", SSL: driver.SSL{Mode: driver.SSLVerifyFull, CAFile: caFile}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if full.TLSConfig == nil || full.TLSConfig.InsecureSkipVerify || full.TLSConfig.ServerName != "db.example.com" || full.TLSConfig.RootCAs == nil {
		t.Errorf("verify-full: TLSConfig = %+v; want verification against the CA and host name", full.TLSConfig)
	}

	_, err = connConfig(driver.Config{Host: "db", Password: "s3cret-value", SSL: driver.SSL{Mode: driver.SSLVerifyCA, CAFile: caFile + ".missing"}}, nil)
	if err == nil {
		t.Error("verify-ca with a missing CA file succeeded")
	} else if strings.Contains(err.Error(), "s3cret-value") {
		t.Errorf("error %q contains the password", err)
	}

	if _, err := connConfig(driver.Config{Host: "db", SSL: driver.SSL{Mode: "sometimes"}}, nil); err == nil {
		t.Error("an unknown SSL mode was accepted")
	}
}

func TestConnConfigDialFunc(t *testing.T) {
	dialed := errors.New("dialed")
	cfg, err := connConfig(driver.Config{Host: "db.internal"}, func(context.Context, string, string) (net.Conn, error) {
		return nil, dialed
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.DialFunc(context.Background(), "tcp", "db.internal:5432"); !errors.Is(err, dialed) {
		t.Errorf("DialFunc error = %v, want the custom dialer's", err)
	}
	// The tunnel resolves names itself, so the host must pass through as is.
	if addrs, err := cfg.LookupFunc(context.Background(), "db.internal"); err != nil || len(addrs) != 1 || addrs[0] != "db.internal" {
		t.Errorf("LookupFunc = %v, %v; want the host unchanged", addrs, err)
	}
}

func TestKindOf(t *testing.T) {
	for typname, want := range map[string]driver.Kind{
		"int2": driver.KindInteger, "int8": driver.KindInteger, "oid": driver.KindInteger,
		"numeric": driver.KindDecimal, "float4": driver.KindDecimal,
		"bool": driver.KindBool, "text": driver.KindText, "bpchar": driver.KindText, "varchar": driver.KindText,
		"date": driver.KindDate, "timetz": driver.KindTime, "timestamptz": driver.KindDateTime,
		"json": driver.KindJSON, "jsonb": driver.KindJSON, "uuid": driver.KindUUID, "bytea": driver.KindBinary,
		"_int4": driver.KindArray, "_text": driver.KindArray,
		"interval": driver.KindOther, "money": driver.KindOther, "inet": driver.KindOther, "16385": driver.KindOther,
	} {
		if got := kindOf(typname); got != want {
			t.Errorf("kindOf(%q) = %q, want %q", typname, got, want)
		}
	}
	if got := columnKind("email_address", "d", "citext"); got != driver.KindText {
		t.Errorf("domain over citext = %q, want text", got)
	}
	if got := columnKind("mood", "e", ""); got != driver.KindText {
		t.Errorf("enum = %q, want text", got)
	}
}

func TestFormat(t *testing.T) {
	jakarta := time.FixedZone("WIB", 7*3600)
	tests := []struct {
		typ  string
		v    any
		want string
	}{
		{"int8", int64(math.MaxInt64), "9223372036854775807"},
		{"int2", int64(-32768), "-32768"},
		{"numeric", "12345678901234567890.0123456789", "12345678901234567890.0123456789"},
		{"bool", true, "true"},
		{"bytea", []byte{0xde, 0xad, 0xbe, 0xef}, "0xdeadbeef"},
		{"bytea", []byte{}, "0x"},
		{"json", []byte(`{"big": 12345678901234567890}`), `{"big": 12345678901234567890}`},
		{"date", time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC), "2026-11-07"},
		{"date", time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC), "0001-01-01 BC"},
		{"date", "infinity", "infinity"},
		{"timestamp", time.Date(2026, 11, 7, 10, 0, 0, 123456000, time.UTC), "2026-11-07 10:00:00.123456"},
		{"timestamp", time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), "1970-01-01 00:00:00"},
		{"timestamptz", time.Date(2026, 11, 7, 10, 0, 0, 500000000, jakarta), "2026-11-07 10:00:00.5+07"},
		{"timestamptz", time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("IST", 5*3600+1800)), "2026-01-01 00:00:00+05:30"},
		{"timestamptz", time.Date(1900, 1, 1, 0, 0, 0, 0, time.FixedZone("LMT", 7*3600+7*60+12)), "1900-01-01 00:00:00+07:07:12"},
		{"timestamptz", time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("BRT", -3*3600)), "2026-01-01 00:00:00-03"},
	}
	for _, tt := range tests {
		if got := (codec{}).Format(driver.ResultColumn{DatabaseType: tt.typ}, tt.v); got != tt.want {
			t.Errorf("Format(%s, %#v) = %q, want %q", tt.typ, tt.v, got, tt.want)
		}
	}
}

// PostgreSQL 12+ prints floats as the shortest exact text, switching to an
// exponent below 1e-4 and from 1e15 (float8) or 1e6 (float4).
func TestFormatFloat(t *testing.T) {
	tests := []struct {
		typ  string
		v    float64
		want string
	}{
		{"float8", 0, "0"},
		{"float8", -2.5, "-2.5"},
		{"float8", 123456789, "123456789"},
		{"float8", 1e14, "100000000000000"},
		{"float8", 1e15, "1e+15"},
		{"float8", 0.0001, "0.0001"},
		{"float8", 0.00001, "1e-05"},
		{"float8", math.MaxFloat64, "1.7976931348623157e+308"},
		{"float8", math.Inf(1), "Infinity"},
		{"float8", math.Inf(-1), "-Infinity"},
		{"float8", math.NaN(), "NaN"},
		{"float4", float64(float32(3.4e38)), "3.4e+38"},
		{"float4", float64(float32(123456)), "123456"},
		{"float4", float64(float32(1234567)), "1.234567e+06"},
		{"float4", float64(float32(0.1)), "0.1"},
	}
	for _, tt := range tests {
		if got := (codec{}).Format(driver.ResultColumn{DatabaseType: tt.typ}, tt.v); got != tt.want {
			t.Errorf("Format(%s, %v) = %q, want %q", tt.typ, tt.v, got, tt.want)
		}
	}
}

func TestDialect(t *testing.T) {
	d := dialect{}
	for name, want := range map[string]string{
		"users":       `"users"`,
		"Weird Table": `"Weird Table"`,
		`a"b`:         `"a""b"`,
		"nul\x00byte": `"nulbyte"`,
		"ünïcödé":     `"ünïcödé"`,
	} {
		if got := d.QuoteIdent(name); got != want {
			t.Errorf("QuoteIdent(%q) = %s, want %s", name, got, want)
		}
	}
	if got := d.Placeholder(3); got != "$3" {
		t.Errorf("Placeholder(3) = %s", got)
	}
}

// writeCA writes a self-signed CA certificate to path and returns path.
func writeCA(t *testing.T, path string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Querra test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
