package screenshots

import (
	"fmt"
	"strings"
	"testing"
)

func TestMaestroProvider_FlowTemplateFormat(t *testing.T) {
	content := fmt.Sprintf(maestroFlowTemplate, "com.example.app", "home")
	if !strings.Contains(content, "appId: com.example.app") {
		t.Errorf("flow should contain appId, got: %s", content)
	}
	if !strings.Contains(content, "launchApp") {
		t.Errorf("flow should contain launchApp, got: %s", content)
	}
	if !strings.Contains(content, "takeScreenshot: home") {
		t.Errorf("flow should contain takeScreenshot: home, got: %s", content)
	}
}
