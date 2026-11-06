package settings

import (
	"reflect"
	"testing"
)

func TestMostSpecificLevelWins(t *testing.T) {
	host := Level{ID: "host", Settings: Settings{Port: ptr(2222)}}
	prod := Level{ID: "prod", Settings: Settings{Port: ptr(22022), User: ptr("deploy")}}
	root := Level{ID: "root", Settings: Settings{User: ptr("admin"), FontSize: ptr(16)}}

	r := Resolve(host, prod, root, Defaults())

	if *r.Settings.Port != 2222 || r.Source["port"] != "host" {
		t.Errorf("port = %d from %q, want 2222 from host", *r.Settings.Port, r.Source["port"])
	}
	if *r.Settings.User != "deploy" || r.Source["user"] != "prod" {
		t.Errorf("user = %q from %q, want deploy from prod", *r.Settings.User, r.Source["user"])
	}
	if *r.Settings.FontSize != 16 || r.Source["fontSize"] != "root" {
		t.Errorf("fontSize = %d from %q, want 16 from root", *r.Settings.FontSize, r.Source["fontSize"])
	}
	if *r.Settings.TermType != "xterm-256color" || r.Source["termType"] != "" {
		t.Errorf("termType should come from defaults")
	}
}

func TestExplicitZeroValueOverrides(t *testing.T) {
	host := Level{ID: "host", Settings: Settings{KeepAliveSeconds: ptr(0), ForwardAgent: ptr(false)}}
	group := Level{ID: "group", Settings: Settings{KeepAliveSeconds: ptr(60), ForwardAgent: ptr(true)}}

	r := Resolve(host, group, Defaults())

	if *r.Settings.KeepAliveSeconds != 0 || *r.Settings.ForwardAgent {
		t.Fatalf("explicit zero values must override inherited ones: %+v", r.Settings)
	}
}

func TestEmptySliceOverridesNilInherits(t *testing.T) {
	group := Level{ID: "group", Settings: Settings{StartupCommands: []string{"cd /srv"}}}

	inherited := Resolve(Level{ID: "a"}, group)
	if !reflect.DeepEqual(inherited.Settings.StartupCommands, []string{"cd /srv"}) {
		t.Errorf("nil slice should inherit, got %v", inherited.Settings.StartupCommands)
	}

	cleared := Resolve(Level{ID: "b", Settings: Settings{StartupCommands: []string{}}}, group)
	if cleared.Settings.StartupCommands == nil || len(cleared.Settings.StartupCommands) != 0 {
		t.Errorf("empty slice should override, got %v", cleared.Settings.StartupCommands)
	}
	if cleared.Source["startupCommands"] != "b" {
		t.Errorf("source = %q, want b", cleared.Source["startupCommands"])
	}
}

func TestEveryFieldIsNillable(t *testing.T) {
	typ := reflect.TypeFor[Settings]()
	for i := 0; i < typ.NumField(); i++ {
		switch k := typ.Field(i).Type.Kind(); k {
		case reflect.Pointer, reflect.Slice, reflect.Map:
		default:
			t.Errorf("field %s has kind %s; must be pointer, slice or map", typ.Field(i).Name, k)
		}
	}
}
