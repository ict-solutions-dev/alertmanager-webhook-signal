#!/bin/bash
set -euo pipefail

# Entrypoint for alertmanager-webhook-signal.
#
# Two modes:
#   1. Static config  — mount a ready config.yaml at /config.yaml (read-only) and
#                        it is used as-is.
#   2. Generated config — otherwise the config is generated from environment
#                         variables at startup. Every variable supports the
#                         "<NAME>__FILE" suffix to read its value from a file
#                         (Docker / Swarm secrets).

STATIC_CONFIG="/config.yaml"
GENERATED_CONFIG="/tmp/config.yaml"
BINARY="/usr/local/bin/alertmanager-webhook-signal"

log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*"
}

# Resolve "<NAME>__FILE" variables into "<NAME>" (Docker / Swarm secrets).
process_secret_files() {
    for var_name in $(env | grep '^[^=]\+__FILE=.\+' | sed -r 's/^([^=]*)__FILE=.*/\1/g' || true); do
        var_name_file="${var_name}__FILE"
        [ -n "${!var_name:-}" ] && {
            echo >&2 "ERROR: Both ${var_name} and ${var_name_file} are set but are exclusive"
            exit 1
        }

        var_filename="${!var_name_file}"
        log "Reading secret ${var_name} from ${var_filename}"

        [ ! -r "${var_filename}" ] && {
            echo >&2 "ERROR: ${var_filename} does not exist or is not readable"
            exit 1
        }

        export "${var_name}"="$(<"${var_filename}")"
        unset "${var_name_file}"
    done
}

# Normalize a boolean-ish value to "true" / "false".
normalize_bool() {
    case "$(echo "${1:-false}" | tr '[:upper:]' '[:lower:]')" in
        true|1|yes|on)   echo "true" ;;
        false|0|no|off)  echo "false" ;;
        *)
            echo >&2 "ERROR: '${1}' is not a valid boolean (use true/false)"
            exit 1
            ;;
    esac
}

validate_required_vars() {
    local required_vars=(
        "SIGNAL_NUMBER"
        "SIGNAL_SEND"
        "SIGNAL_RECIPIENTS"
    )
    for var in "${required_vars[@]}"; do
        [ -z "${!var:-}" ] && {
            log "ERROR: Required variable ${var} is not set (and no static ${STATIC_CONFIG} mounted)"
            exit 1
        }
    done
    return 0
}

# Append a YAML list (one quoted item per comma-separated value) at a given indent.
append_yaml_list() {
    local csv="$1" indent="$2" item
    local IFS=','
    for item in $csv; do
        item="$(echo "$item" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
        [ -z "$item" ] && continue
        printf '%s- "%s"\n' "$indent" "$item" >>"$GENERATED_CONFIG"
    done
}

generate_config() {
    local debug textmode generator_url server_port server_interface
    debug="$(normalize_bool "${SERVER_DEBUG:-false}")"
    textmode="$(normalize_bool "${SIGNAL_TEXTMODE_NORMAL:-false}")"
    generator_url="$(normalize_bool "${ALERT_GENERATOR_URL:-true}")"
    server_port="${SERVER_PORT:-10000}"
    server_interface="${SERVER_INTERFACE:-0.0.0.0}"

    {
        echo "server:"
        echo "  port: \"${server_port}\""
        echo "  interface: \"${server_interface}\""
        echo "  debug: ${debug}"
        echo "signal:"
        echo "  number: \"${SIGNAL_NUMBER}\""
        echo "  send: \"${SIGNAL_SEND}\""
        echo "  textmodeNormal: ${textmode}"
        echo "  recipients:"
    } >"$GENERATED_CONFIG"
    append_yaml_list "${SIGNAL_RECIPIENTS}" "    "

    {
        echo "alertmanager:"
        echo "  ignoreLabels:"
    } >>"$GENERATED_CONFIG"
    append_yaml_list "${ALERT_IGNORE_LABELS:-alertname,recipients}" "    "

    if [ -n "${ALERT_IGNORE_ANNOTATIONS:-}" ]; then
        echo "  ignoreAnnotations:" >>"$GENERATED_CONFIG"
        append_yaml_list "${ALERT_IGNORE_ANNOTATIONS}" "    "
    else
        echo "  ignoreAnnotations: []" >>"$GENERATED_CONFIG"
    fi
    echo "  generatorURL: ${generator_url}" >>"$GENERATED_CONFIG"

    # Recipient name -> Signal recipient map, from RECIPIENT_<NAME> variables.
    # e.g. RECIPIENT_PROXMOX="group.xxx" becomes  proxmox: "group.xxx"
    local has_recipients=false line name value
    for line in $(env | grep '^RECIPIENT_[^=]\+=.\+' | sed -r 's/^(RECIPIENT_[^=]*)=.*/\1/g' || true); do
        if [ "$has_recipients" = false ]; then
            echo "recipients:" >>"$GENERATED_CONFIG"
            has_recipients=true
        fi
        name="$(echo "${line#RECIPIENT_}" | tr '[:upper:]' '[:lower:]')"
        value="${!line}"
        printf '  %s: "%s"\n' "$name" "$value" >>"$GENERATED_CONFIG"
    done

    # Optional message templates (raw template text, indented as a YAML block scalar).
    if [ -n "${TEMPLATE_GRAFANA:-}" ] || [ -n "${TEMPLATE_ALERTMANAGER:-}" ]; then
        echo "templates:" >>"$GENERATED_CONFIG"
        if [ -n "${TEMPLATE_GRAFANA:-}" ]; then
            echo "  grafana: |-" >>"$GENERATED_CONFIG"
            printf '%s\n' "${TEMPLATE_GRAFANA}" | sed 's/^/    /' >>"$GENERATED_CONFIG"
        fi
        if [ -n "${TEMPLATE_ALERTMANAGER:-}" ]; then
            echo "  alertmanager: |-" >>"$GENERATED_CONFIG"
            printf '%s\n' "${TEMPLATE_ALERTMANAGER}" | sed 's/^/    /' >>"$GENERATED_CONFIG"
        fi
    fi
    return 0
}

print_redacted_config() {
    log "Generated configuration (number redacted):"
    sed -E 's/(number): ".+"/\1: "********"/' "$GENERATED_CONFIG"
}

main() {
    if [ -f "$STATIC_CONFIG" ]; then
        log "Using mounted static configuration at ${STATIC_CONFIG}"
        export CONFIG_PATH="$STATIC_CONFIG"
    else
        log "No static ${STATIC_CONFIG} found, generating configuration from environment..."
        process_secret_files
        validate_required_vars
        generate_config
        print_redacted_config
        export CONFIG_PATH="$GENERATED_CONFIG"
    fi

    log "Starting alertmanager-webhook-signal..."
    exec "$BINARY"
}

main "$@"
