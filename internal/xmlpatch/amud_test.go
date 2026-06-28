package xmlpatch

import (
	"strings"
	"testing"
)

func TestApplyAMUDLabelsAddsBeforeTailscaleStateDir(t *testing.T) {
	original := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <TailscaleStateDir/>
</Container>
`
	result, err := ApplyAMUDLabels(original, map[string]string{
		"amud.enable": "true",
		"amud.url":    "http://192.0.2.10:8080",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("expected changed XML")
	}
	if !strings.Contains(result.Modified, `Target="amud.enable"`) {
		t.Fatal("expected amud.enable label")
	}
	if strings.Index(result.Modified, `Target="amud.url"`) > strings.Index(result.Modified, `<TailscaleStateDir/>`) {
		t.Fatal("expected labels before TailscaleStateDir")
	}
}

func TestApplyAMUDLabelsUpdatesExistingLabel(t *testing.T) {
	original := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Config Name="Old" Target="amud.url" Default="old" Mode="" Description="" Type="Label" Display="advanced" Required="false" Mask="false">old</Config>
</Container>
`
	result, err := ApplyAMUDLabels(original, map[string]string{
		"amud.url": "http://192.0.2.10:8080",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Modified, `Default="http://192.0.2.10:8080"`) {
		t.Fatal("expected updated URL")
	}
	if strings.Count(result.Modified, `Target="amud.url"`) != 1 {
		t.Fatal("expected exactly one amud.url label")
	}
}
