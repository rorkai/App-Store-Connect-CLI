package cmd

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("ASC_TELEMETRY_DISABLED", "1")
	os.Exit(m.Run())
}
