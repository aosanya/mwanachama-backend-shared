package dispatch

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestDispatchAndPlaintextDependOnNoPostgresDriver(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", ".", "../plaintext")
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, errOut.String())
	}
	forbidden := []string{"gorm.io/driver/postgres", "jackc/pgx", "lib/pq"}
	for _, importPath := range strings.Fields(out.String()) {
		for _, name := range forbidden {
			if strings.Contains(importPath, name) {
				t.Errorf("the packages depend on %s", importPath)
			}
		}
	}
}
