package xmlpatch

import (
	"strings"
	"testing"
)

func TestApplyVariableAddsTZ(t *testing.T) {
	original := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <TailscaleStateDir/>
</Container>
`
	result, err := ApplyVariable(original, "TZ", "Europe/Prague", "TZ")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("expected changed XML")
	}
	if !strings.Contains(result.Modified, `Target="TZ"`) {
		t.Fatal("expected TZ variable")
	}
	if !strings.Contains(result.Modified, `Europe/Prague`) {
		t.Fatal("expected Europe/Prague value")
	}
}
