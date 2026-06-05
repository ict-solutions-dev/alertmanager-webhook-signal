package main

import (
	"alertmanager-webhook-signal/internal/alerts"
	"alertmanager-webhook-signal/internal/config"
	"alertmanager-webhook-signal/internal/util"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"text/template"
	"time"

	"github.com/lmittmann/tint"
	"github.com/schlauerlauer/go-middleware"
)

// appVersion is injected at build time via -ldflags "-X main.appVersion=..."
// (release tag or `git describe` in CI). It defaults to "dev" otherwise.
var appVersion = "dev"

// version is the effective version reported at runtime. For local builds with
// no injected version, it falls back to the VCS revision Go embeds automatically
// (e.g. "dev-1a2b3c4d5e6f" or "dev-1a2b3c4d5e6f-dirty").
var version = resolveVersion()

func resolveVersion() string {
	if appVersion != "dev" {
		return appVersion
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return appVersion
	}
	var revision, suffix string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			if setting.Value == "true" {
				suffix = "-dirty"
			}
		}
	}
	if revision == "" {
		return appVersion
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	return "dev-" + revision + suffix
}

// templateFuncs are made available to both message templates.
var templateFuncs = template.FuncMap{
	"toUpper":          strings.ToUpper,
	"toLower":          strings.ToLower,
	"since":            since,
	"humanizeDuration": humanizeDuration,
}

// since returns the elapsed time since an RFC3339 timestamp (e.g. Alert.StartsAt).
func since(timestamp string) time.Duration {
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return 0
	}
	return time.Since(parsed)
}

// humanizeDuration formats a duration (a time.Duration, a number of seconds, or
// a parseable duration string) as a compact "1d 2h 3m 4s".
func humanizeDuration(value any) string {
	var duration time.Duration
	switch typed := value.(type) {
	case time.Duration:
		duration = typed
	case float64:
		duration = time.Duration(typed * float64(time.Second))
	case int:
		duration = time.Duration(typed) * time.Second
	case int64:
		duration = time.Duration(typed) * time.Second
	case string:
		parsed, err := time.ParseDuration(typed)
		if err != nil {
			return typed
		}
		duration = parsed
	default:
		return fmt.Sprint(value)
	}

	if duration < 0 {
		duration = -duration
	}
	duration = duration.Round(time.Second)

	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	duration -= minutes * time.Minute
	seconds := duration / time.Second

	parts := make([]string, 0, 4)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, " ")
}

const defaultGrafanaTemplate = `{{ if eq .State "alerting" }}❗{{ else }}✅{{ end }} {{ .Title }}
{{ .RuleName }}
{{ .Message }}
{{ .RuleUrl }}`

const defaultAlertmanageTemplate = `{{ if eq .Alert.Status "firing" }}🚨{{ else }}✅{{ end }} **{{ .Alertname }}**{{ if ne .Alert.Status "firing" }} — RESOLVED{{ end }}
{{- with .Annotations.summary }}

📋 {{ . }}{{ end }}
{{- with .Annotations.description }}
ℹ️ {{ . }}{{ else }}{{ with $.Annotations.message }}
💬 {{ . }}{{ end }}{{ end }}
{{- with .Alert.Labels.severity }}

⚠️ Severity: {{ . }}{{ end }}
{{- with .Alert.Labels.instance }}
🖥️ Instance: {{ . }}{{ else }}{{ with $.Alert.Labels.node }}
🖥️ Instance: {{ . }}{{ else }}{{ with $.Alert.Labels.host }}
🖥️ Instance: {{ . }}{{ end }}{{ end }}{{ end }}
{{- with .Alert.Labels.job }}
🔧 Job: {{ . }}{{ end }}
{{- with .Alert.Labels.environment }}
🌍 Environment: {{ . }}{{ end }}
{{- if eq .Alert.Status "firing" }}{{ with .Alert.StartsAt }}
⏱️ Duration: {{ humanizeDuration (since .) }}{{ end }}{{ end }}
{{- if and .Config.GeneratorURL .Alert.GeneratorURL }}

🔗 {{ .Alert.GeneratorURL }}{{ end }}
`

func main() {
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelInfo,
			TimeFormat: time.TimeOnly,
		}),
	))

	configPath := util.StringDefault(os.Getenv("CONFIG_PATH"), "/config.yaml")

	cfg, err := config.NewConfig(configPath)
	if err != nil {
		slog.Error("Error reading config", "err", err)
		os.Exit(1)
	}

	grafanaTemplate, err := template.New("grafana").Funcs(templateFuncs).Parse(util.StringDefault(cfg.Templates.Grafana, defaultGrafanaTemplate))
	if err != nil {
		slog.Error("error parsing grafana template", "err", err)
		os.Exit(1)
	}

	alertmanagerTemplate, err := template.New("alertmanager").Funcs(templateFuncs).Parse(util.StringDefault(cfg.Templates.Alertmanager, defaultAlertmanageTemplate))
	if err != nil {
		slog.Error("error parsing alertmanager template", "err", err)
		os.Exit(1)
	}

	alert := alerts.NewAlert(
		cfg,
		grafanaTemplate,
		alertmanagerTemplate,
	)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pong"))
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(version))
	})

	mux.HandleFunc("POST /alertmanager", alert.Alertmanager)
	mux.HandleFunc("POST /api/v3/alertmanager", alert.Alertmanager)

	mux.HandleFunc("POST /grafana", alert.Grafana)
	mux.HandleFunc("POST /api/v2/alert/grafana", alert.Grafana)
	mux.HandleFunc("POST /api/v3/grafana", alert.Grafana)

	listenInterface := util.StringDefault(cfg.Server.Interface, "0.0.0.0")
	listenPort := util.StringDefault(cfg.Server.Port, "10000")

	slog.Info("Server starting", "version", version, "interface", listenInterface, "port", listenPort)
	if err := http.ListenAndServe(fmt.Sprint(listenInterface, ":", listenPort), middleware.Logging(mux)); err != nil {
		slog.Error("error starting server", "err", err)
		os.Exit(1)
	}
}
