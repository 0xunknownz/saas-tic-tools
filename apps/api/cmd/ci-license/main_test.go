package main

import (
	"context"
	"strings"
	"testing"
)

func TestRefusesOutsideDevelopmentCI(t *testing.T) {
	for _, config := range []struct{ actions, environment string }{{"", "development"}, {"true", "production"}, {"", "production"}} {
		t.Run(config.actions+"/"+config.environment, func(t *testing.T) {
			t.Setenv("GITHUB_ACTIONS", config.actions)
			t.Setenv("APP_ENV", config.environment)
			t.Setenv("DATABASE_URL", "must-not-connect")
			err := install(context.Background())
			if err == nil || !strings.Contains(err.Error(), "requires GITHUB_ACTIONS=true and APP_ENV=development") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRequiresRunnerEnvironmentBeforeConnecting(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("APP_ENV", "development")
	t.Setenv("GITHUB_ENV", "")
	t.Setenv("DATABASE_URL", "must-not-connect")
	if err := install(context.Background()); err == nil || !strings.Contains(err.Error(), "GITHUB_ENV is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
