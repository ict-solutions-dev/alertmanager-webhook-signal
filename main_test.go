package main

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
	"time"
)

func TestHumanizeDuration(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"zero", time.Duration(0), "0s"},
		{"seconds", 90 * time.Second, "1m 30s"},
		{"hours and minutes", 2*time.Hour + 5*time.Minute, "2h 5m"},
		{"days", 25 * time.Hour, "1d 1h"},
		{"float seconds", float64(45), "45s"},
		{"duration string", "1h30m", "1h 30m"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := humanizeDuration(testCase.value); got != testCase.want {
				t.Errorf("humanizeDuration(%v) = %q, want %q", testCase.value, got, testCase.want)
			}
		})
	}
}

func TestDefaultTemplatesParse(t *testing.T) {
	if _, err := template.New("grafana").Funcs(templateFuncs).Parse(defaultGrafanaTemplate); err != nil {
		t.Fatalf("grafana template failed to parse: %v", err)
	}
	if _, err := template.New("alertmanager").Funcs(templateFuncs).Parse(defaultAlertmanageTemplate); err != nil {
		t.Fatalf("alertmanager template failed to parse: %v", err)
	}
}

type templateAlert struct {
	Status       string
	StartsAt     string
	GeneratorURL string
	Labels       map[string]any
}

type templateData struct {
	Alertname   string
	Alert       templateAlert
	Config      struct{ GeneratorURL bool }
	Annotations map[string]any
}

func TestDefaultAlertmanagerTemplateRender(t *testing.T) {
	tmpl := template.Must(template.New("alertmanager").Funcs(templateFuncs).Parse(defaultAlertmanageTemplate))

	var data templateData
	data.Alertname = "WindowsServerCollectorError"
	data.Alert = templateAlert{
		Status:       "firing",
		StartsAt:     time.Now().Add(-90 * time.Second).Format(time.RFC3339),
		GeneratorURL: "https://prometheus.example.com/graph",
		Labels:       map[string]any{"severity": "critical", "instance": "em.ictcloud.sk", "job": "windows_exporters"},
	}
	data.Config.GeneratorURL = true
	data.Annotations = map[string]any{"summary": "Windows Server collector Error"}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatalf("execute: %v", err)
	}

	rendered := out.String()
	for _, want := range []string{
		"🚨",
		"WindowsServerCollectorError",
		"Windows Server collector Error",
		"Severity: critical",
		"Instance: em.ictcloud.sk",
		"Job: windows_exporters",
		"Duration:",
		"prometheus.example.com",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered output missing %q\n--- output ---\n%s", want, rendered)
		}
	}

	data.Alert.Status = "resolved"
	out.Reset()
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatalf("execute (resolved): %v", err)
	}
	if !strings.Contains(out.String(), "RESOLVED") || !strings.Contains(out.String(), "✅") {
		t.Errorf("resolved output missing markers\n--- output ---\n%s", out.String())
	}
}
