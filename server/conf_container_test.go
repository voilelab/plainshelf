package server

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

const containerConfigPath = "../docker/config.yaml"

func TestContainerDefaultConfigProtectsWrites(t *testing.T) {
	conf := loadContainerConf(t)

	if conf.AppConf == nil || conf.AppConf.Security == nil {
		t.Fatal("container config has no app_conf.security block")
	}
	if got := conf.AppConf.Security.Mode; got != SecurityModeLocalToken {
		t.Fatalf("container config security mode = %q, want %q", got, SecurityModeLocalToken)
	}

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
