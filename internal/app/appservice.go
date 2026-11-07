// Package app holds the Wails services bound to the frontend. Services stay
// thin: they validate input, call domain packages, and map errors. Together
// with main.go, this is the only package allowed to import Wails.
package app

// AppInfo describes the running application.
type AppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// AppService exposes application metadata to the frontend.
type AppService struct {
	info AppInfo
}

// NewAppService returns an AppService that reports the given version.
func NewAppService(version string) *AppService {
	return &AppService{info: AppInfo{Name: "Querra", Version: version}}
}

// Info returns the application name and version.
func (s *AppService) Info() AppInfo {
	return s.info
}
