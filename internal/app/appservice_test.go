package app

import "testing"

func TestAppServiceInfo(t *testing.T) {
	got := NewAppService("1.2.3").Info()
	want := AppInfo{Name: "Querra", Version: "1.2.3"}
	if got != want {
		t.Fatalf("Info() = %+v, want %+v", got, want)
	}
}
