package server

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// containerConfigPath is the config the published image runs unmodified, so
// what it parses to is what a user who forgets a port mapping is protected by.
const containerConfigPath = "../docker/config.yaml"

// TestContainerDefaultConfigProtectsWrites pins the shipped container default.
// A regression to mode: "none" would silently answer anonymous writes on any
// exposed port, which is exactly what this default exists to prevent.
//
// Parsing a file and checking a pure guard asks nothing of the router, so it is
// pinned here. That the parsed block then actually gates a request is the half
// that needs one, and lives in
// server/contract/crosscut/security_test.go.
func TestContainerDefaultConfigProtectsWrites(t *testing.T) {
	conf := loadContainerConf(t)

	if conf.AppConf == nil || conf.AppConf.Security == nil {
		t.Fatal("container config has no app_conf.security block")
	}
	if got := conf.AppConf.Security.Mode; got != SecurityModeLocalToken {
		t.Fatalf("container config security mode = %q, want %q", got, SecurityModeLocalToken)
	}

	// addr stays 0.0.0.0:20000 (required inside the container) and the
	// listen-addr guard still passes because the mode is set explicitly.
	if conf.ServerConf == nil {
		t.Fatal("container config has no server_conf block")
	}
	if err := ValidateSecurityForListenAddr(conf.AppConf.Security, conf.ServerConf.Addr); err != nil {
		t.Fatalf("ValidateSecurityForListenAddr(%q) = %v, want nil", conf.ServerConf.Addr, err)
	}
}

func loadContainerConf(t *testing.T) SrvConf {
	t.Helper()

	bs, err := os.ReadFile(containerConfigPath)
	if err != nil {
		t.Fatalf("read %s: %v", containerConfigPath, err)
	}
	var conf SrvConf
	if err := yaml.Unmarshal(bs, &conf); err != nil {
		t.Fatalf("parse %s: %v", containerConfigPath, err)
	}
	return conf
}
