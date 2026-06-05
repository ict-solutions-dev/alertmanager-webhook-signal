package alerts

import (
	"strings"
	"testing"
	"text/template"

	"alertmanager-webhook-signal/internal/config"
	"alertmanager-webhook-signal/internal/model"
)

// newTestAlert builds an Alert with simple templates that expose the alert name
// and labels, so routing and label filtering can be asserted on the output.
func newTestAlert(cfg *config.ConfigData) *Alert {
	alertmanagerTemplate := template.Must(template.New("am").Parse(
		`{{ .Alertname }}|{{ range $key, $value := .Labels }}{{ $key }}={{ $value }};{{ end }}`,
	))
	grafanaTemplate := template.Must(template.New("grafana").Parse(`{{ .Title }}`))
	return NewAlert(cfg, grafanaTemplate, alertmanagerTemplate)
}

func baseConfig() *config.ConfigData {
	return &config.ConfigData{
		Signal: config.SignalConfig{
			Number:     "+421918533760",
			Recipients: []string{"group.DEFAULT"},
		},
		Recipients: map[string]string{
			"proxmox":  "group.PROXMOX",
			"critical": "group.CRITICAL",
		},
	}
}

func singleAlert(labels map[string]any) model.Alertmanager {
	return model.Alertmanager{
		Alerts: []model.AMAlert{
			{Status: "firing", Labels: labels, Annotations: map[string]any{}},
		},
	}
}

func recipientsOf(messages []model.SignalMessage) []string {
	var all []string
	for _, message := range messages {
		all = append(all, message.Recipients...)
	}
	return all
}

func TestAlertToSignalDefaultRecipient(t *testing.T) {
	alert := newTestAlert(baseConfig())

	messages := alert.alertToSignal(singleAlert(map[string]any{"alertname": "Watchdog"}))

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if got := recipientsOf(messages); len(got) != 1 || got[0] != "group.DEFAULT" {
		t.Errorf("recipients = %v, want [group.DEFAULT]", got)
	}
	if messages[0].Number != "+421918533760" {
		t.Errorf("Number = %q, want %q", messages[0].Number, "+421918533760")
	}
	if messages[0].TextMode != "styled" {
		t.Errorf("TextMode = %q, want styled", messages[0].TextMode)
	}
}

func TestAlertToSignalNamedRecipient(t *testing.T) {
	alert := newTestAlert(baseConfig())

	messages := alert.alertToSignal(singleAlert(map[string]any{
		"alertname":  "ProxmoxNodeDown",
		"recipients": "proxmox",
	}))

	if got := recipientsOf(messages); len(got) != 1 || got[0] != "group.PROXMOX" {
		t.Errorf("recipients = %v, want [group.PROXMOX]", got)
	}
}

func TestAlertToSignalMultipleRecipients(t *testing.T) {
	alert := newTestAlert(baseConfig())

	messages := alert.alertToSignal(singleAlert(map[string]any{
		"alertname":  "Boom",
		"recipients": "proxmox,critical",
	}))

	got := recipientsOf(messages)
	want := []string{"group.PROXMOX", "group.CRITICAL"}
	if len(got) != len(want) {
		t.Fatalf("recipients = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("recipients = %v, want %v", got, want)
		}
	}
}

func TestAlertToSignalDeduplicatesRecipients(t *testing.T) {
	alert := newTestAlert(baseConfig())

	messages := alert.alertToSignal(singleAlert(map[string]any{
		"alertname":  "Dup",
		"recipients": "proxmox, proxmox , proxmox",
	}))

	if got := recipientsOf(messages); len(got) != 1 || got[0] != "group.PROXMOX" {
		t.Errorf("recipients = %v, want a single [group.PROXMOX]", got)
	}
}

func TestAlertToSignalUnknownRecipientFallsBackToDefault(t *testing.T) {
	alert := newTestAlert(baseConfig())

	messages := alert.alertToSignal(singleAlert(map[string]any{
		"alertname":  "Mystery",
		"recipients": "does-not-exist",
	}))

	if got := recipientsOf(messages); len(got) != 1 || got[0] != "group.DEFAULT" {
		t.Errorf("recipients = %v, want [group.DEFAULT]", got)
	}
}

func TestAlertToSignalOneMessagePerDefaultRecipient(t *testing.T) {
	cfg := baseConfig()
	cfg.Signal.Recipients = []string{"group.A", "group.B"}
	alert := newTestAlert(cfg)

	messages := alert.alertToSignal(singleAlert(map[string]any{"alertname": "Fanout"}))

	got := recipientsOf(messages)
	want := []string{"group.A", "group.B"}
	if len(messages) != 2 || len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("recipients = %v, want %v (one message each)", got, want)
	}
}

func TestAlertToSignalIgnoreLabels(t *testing.T) {
	cfg := baseConfig()
	cfg.AMConfig.IgnoreLabels = []string{"recipients"}
	alert := newTestAlert(cfg)

	messages := alert.alertToSignal(singleAlert(map[string]any{
		"alertname":  "Filtered",
		"recipients": "proxmox",
	}))

	// Routing still works even though the label is filtered out of the message.
	if got := recipientsOf(messages); len(got) != 1 || got[0] != "group.PROXMOX" {
		t.Fatalf("recipients = %v, want [group.PROXMOX]", got)
	}
	if strings.Contains(messages[0].Message, "recipients=") {
		t.Errorf("message should not contain the ignored label, got %q", messages[0].Message)
	}
}

func TestAlertToSignalTextModeNormal(t *testing.T) {
	cfg := baseConfig()
	cfg.Signal.TextModeNormal = true
	alert := newTestAlert(cfg)

	messages := alert.alertToSignal(singleAlert(map[string]any{"alertname": "Plain"}))

	if messages[0].TextMode != "normal" {
		t.Errorf("TextMode = %q, want normal", messages[0].TextMode)
	}
}
