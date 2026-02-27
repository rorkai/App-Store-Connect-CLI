package screenshots

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func prepareKoubouVersionBenchmark(b *testing.B) {
	resetKoubouVersionCacheForTest()

	binDir := b.TempDir()
	kouPath := filepath.Join(binDir, "kou")
	script := `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then
  echo "kou 0.14.0"
  exit 0
fi
echo "unsupported args" >&2
exit 1
`
	if err := os.WriteFile(kouPath, []byte(script), 0o755); err != nil {
		b.Fatalf("WriteFile(%q) error: %v", kouPath, err)
	}
	b.Setenv("PATH", binDir)
}

func BenchmarkEnsurePinnedKoubouVersionCached(b *testing.B) {
	prepareKoubouVersionBenchmark(b)

	if _, err := ensurePinnedKoubouVersion(context.Background()); err != nil {
		b.Fatalf("ensurePinnedKoubouVersion() warmup error: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ensurePinnedKoubouVersion(context.Background()); err != nil {
			b.Fatalf("ensurePinnedKoubouVersion() error: %v", err)
		}
	}
}

func BenchmarkEnsurePinnedKoubouVersionUncached(b *testing.B) {
	prepareKoubouVersionBenchmark(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetKoubouVersionCacheForTest()
		if _, err := ensurePinnedKoubouVersion(context.Background()); err != nil {
			b.Fatalf("ensurePinnedKoubouVersion() error: %v", err)
		}
	}
}
