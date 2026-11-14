// Package connection manages saved connections (PLAN §3.6, FR-01, FR-03 to
// FR-05): their settings in the local store, their passwords in the OS
// keychain, Test Connection, and export and import as JSON without secrets.
// Values that cross to the frontend never include a secret, only
// PasswordState.
package connection

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cokagung30/querra/internal/driver"
	"github.com/cokagung30/querra/internal/store"
)

// Env is a connection's environment label (FR-03).
type Env string

const (
	EnvLocal      Env = "local"
	EnvStaging    Env = "staging"
	EnvProduction Env = "production"
)

// Color is a named color from the theme's palette, so it works in light and
// dark mode. ColorDefault uses the environment's color.
type Color string

const (
	ColorDefault Color = ""
	ColorRed     Color = "red"
	ColorOrange  Color = "orange"
	ColorYellow  Color = "yellow"
	ColorGreen   Color = "green"
	ColorTeal    Color = "teal"
	ColorBlue    Color = "blue"
	ColorPurple  Color = "purple"
	ColorGray    Color = "gray"
)

// PasswordState says whether the keychain holds a connection's database
// password.
type PasswordState string

const (
	// PasswordNone means no password is saved, and none is expected.
	PasswordNone PasswordState = "none"
	// PasswordSaved means the keychain holds the password.
	PasswordSaved PasswordState = "saved"
	// PasswordRequired means a password is needed but not saved, as after an
	// import; ask for it before connecting.
	PasswordRequired PasswordState = "required"
)

// SSL is a connection's TLS settings. Certificates and keys are file paths;
// their contents are never stored.
type SSL struct {
	// Mode follows libpq's sslmode; "" means prefer.
	Mode     driver.SSLMode `json:"mode"`
	CAFile   string         `json:"caFile"`
	CertFile string         `json:"certFile"`
	KeyFile  string         `json:"keyFile"`
}

// Settings is what the connection form edits: everything about a connection
// except its identity, its place in the list, and its password.
type Settings struct {
	// GroupID is the group the connection is in; "" means none.
	GroupID string        `json:"groupId"`
	Name    string        `json:"name"`
	Engine  driver.Engine `json:"engine"`
	Env     Env           `json:"env"`
	Color   Color         `json:"color"`
	Host    string        `json:"host"`
	// Port 0 means the engine's default port.
	Port     int    `json:"port"`
	Username string `json:"username"`
	Database string `json:"database"`
	SSL      SSL    `json:"ssl"`
	// ReadOnly applies read-only mode to every session (PLAN §3.7).
	ReadOnly bool `json:"readOnly"`
}

// Connection is a saved connection.
type Connection struct {
	ID            string        `json:"id"`
	Settings      Settings      `json:"settings"`
	SortOrder     int           `json:"sortOrder"`
	PasswordState PasswordState `json:"passwordState"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

// Group is a folder of connections in the sidebar.
type Group struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}

// PasswordAction says what saving or testing does with the password.
type PasswordAction string

const (
	// PasswordKeep leaves the saved password as it is (or uses it, when
	// testing). For a new connection it means no password.
	PasswordKeep PasswordAction = "keep"
	// PasswordSet saves Value as the password (or tests with it).
	PasswordSet PasswordAction = "set"
	// PasswordClear removes the saved password (or tests without one).
	PasswordClear PasswordAction = "clear"
)

// PasswordChange comes with the form's settings. Value is only read for
// PasswordSet and never stored anywhere but the keychain.
type PasswordChange struct {
	Action PasswordAction `json:"action"`
	Value  string         `json:"value"`
}

// normalize trims what the user is unlikely to mean literally and fills in
// the default SSL mode.
func normalize(s Settings) Settings {
	s.Name = strings.TrimSpace(s.Name)
	s.Host = strings.TrimSpace(s.Host)
	if s.SSL.Mode == "" {
		s.SSL.Mode = driver.SSLPrefer
	}
	return s
}

// driverConfig is what Open needs to reach the connection's database.
func (s Settings) driverConfig(password string) driver.Config {
	return driver.Config{
		Host:     s.Host,
		Port:     s.Port,
		User:     s.Username,
		Password: password,
		Database: s.Database,
		SSL: driver.SSL{
			Mode:     s.SSL.Mode,
			CAFile:   s.SSL.CAFile,
			CertFile: s.SSL.CertFile,
			KeyFile:  s.SSL.KeyFile,
		},
	}
}

func toStore(id string, s Settings, order int, state PasswordState) (store.Connection, error) {
	ssl, err := json.Marshal(s.SSL)
	if err != nil {
		return store.Connection{}, fmt.Errorf("encode SSL settings: %w", err)
	}
	return store.Connection{
		ID:            id,
		GroupID:       s.GroupID,
		Name:          s.Name,
		Engine:        string(s.Engine),
		Env:           string(s.Env),
		Color:         string(s.Color),
		Host:          s.Host,
		Port:          s.Port,
		Username:      s.Username,
		Database:      s.Database,
		SSL:           ssl,
		ReadOnly:      s.ReadOnly,
		SortOrder:     order,
		PasswordState: string(state),
	}, nil
}

func fromStore(c store.Connection) (Connection, error) {
	var ssl SSL
	if err := json.Unmarshal(c.SSL, &ssl); err != nil {
		return Connection{}, fmt.Errorf("decode SSL settings of connection %s: %w", c.ID, err)
	}
	return Connection{
		ID: c.ID,
		Settings: Settings{
			GroupID:  c.GroupID,
			Name:     c.Name,
			Engine:   driver.Engine(c.Engine),
			Env:      Env(c.Env),
			Color:    Color(c.Color),
			Host:     c.Host,
			Port:     c.Port,
			Username: c.Username,
			Database: c.Database,
			SSL:      ssl,
			ReadOnly: c.ReadOnly,
		},
		SortOrder:     c.SortOrder,
		PasswordState: PasswordState(c.PasswordState),
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}, nil
}

// EngineInfo describes an engine Querra can connect to, for the form.
type EngineInfo struct {
	Engine driver.Engine `json:"engine"`
	// DefaultPort is 0 for an engine without a network server, whose
	// Database is a file path.
	DefaultPort  int                 `json:"defaultPort"`
	Capabilities driver.Capabilities `json:"capabilities"`
}

// Engines lists the engines that have a driver, sorted.
func Engines() []EngineInfo {
	out := []EngineInfo{}
	for _, e := range driver.Engines() {
		d, err := driver.Lookup(e)
		if err != nil {
			continue // drivers are never unregistered, so a listed engine has one
		}
		out = append(out, EngineInfo{Engine: e, DefaultPort: d.DefaultPort(), Capabilities: d.Capabilities()})
	}
	return out
}
