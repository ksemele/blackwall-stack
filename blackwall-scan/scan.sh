#!/usr/bin/env bash
#
# blackwall-scan — external attack surface pipeline
# Runs INSIDE the container. Reads /targets/targets.txt, writes /output/<ts>/.
#
# Pipeline: subfinder -> (+direct IPs) -> dnsx -> naabu -> confirm -> httpx -> tlsx -> nuclei
#
# Env knobs:
#   PORTS           naabu port spec (default: "-top-ports 1000"; e.g. "-p -" for all 65535)
#   RUN_NUCLEI      "1" to run nuclei vuln/exposure checks (default: 0)
#   NUCLEI_TAGS     nuclei -tags (default: exposure,misconfig,takeover,default-login)
#   RATE            naabu -rate (default: 1000)
#   RUN_CONFIRM     "0" to skip the confirmation pass (default: 1)
#   CONFIRM_TIMEOUT per-pair probe timeout in the confirm pass, seconds (default: 3)
#
set -euo pipefail

TARGETS_FILE="${TARGETS_FILE:-/targets/targets.txt}"
PORTS="${PORTS:--top-ports 1000}"
RUN_NUCLEI="${RUN_NUCLEI:-0}"
NUCLEI_TAGS="${NUCLEI_TAGS:-exposure,misconfig,takeover,default-login}"
RATE="${RATE:-1000}"
RUN_CONFIRM="${RUN_CONFIRM:-1}"
CONFIRM_TIMEOUT="${CONFIRM_TIMEOUT:-3}"

# confirm.py sits next to scan.sh both in the image and in the repo
CONFIRM_SCRIPT="$(dirname "$0")/confirm.py"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="/output/${TS}"
mkdir -p "$OUT"

log() { printf '\033[36m[scan %s]\033[0m %s\n' "$(date -u +%H:%M:%S)" "$*"; }

if [[ ! -s "$TARGETS_FILE" ]]; then
  echo "ERROR: no targets at $TARGETS_FILE" >&2
  exit 1
fi

# --- split targets into domains vs IPs (v4) ---
grep -vE '^\s*(#|$)' "$TARGETS_FILE" | tr -d ' ' > "$OUT/targets.raw"
grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' "$OUT/targets.raw" > "$OUT/ips.txt" || true
grep -vE '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' "$OUT/targets.raw" > "$OUT/domains.txt" || true

log "targets: $(wc -l < "$OUT/targets.raw") ($(wc -l < "$OUT/domains.txt") domains, $(wc -l < "$OUT/ips.txt") IPs)"

# --- 1. subdomain discovery (passive) ---
: > "$OUT/subs.txt"
if [[ -s "$OUT/domains.txt" ]]; then
  log "subfinder: passive subdomain enumeration"
  subfinder -dL "$OUT/domains.txt" -all -silent -o "$OUT/subs.raw" || true
  cat "$OUT/domains.txt" "$OUT/subs.raw" 2>/dev/null | sort -u > "$OUT/subs.txt"
fi

# --- 2. hosts to probe = subs + direct IPs ---
cat "$OUT/subs.txt" "$OUT/ips.txt" 2>/dev/null | sort -u > "$OUT/hosts.txt"
log "hosts to scan: $(wc -l < "$OUT/hosts.txt")"

# --- 3. dnsx: resolve + record A/AAAA/CNAME (JSONL) ---
log "dnsx: resolving"
dnsx -l "$OUT/hosts.txt" -a -aaaa -cname -resp -json -silent -o "$OUT/dns.jsonl" || true

# --- 4. naabu: port scan (connect scan, no NET_RAW needed) ---
log "naabu: port scan ($PORTS, rate $RATE, connect-scan)"
# shellcheck disable=SC2086
naabu -list "$OUT/hosts.txt" $PORTS -s c -rate "$RATE" -json -silent -o "$OUT/ports.jsonl" || true

# build host:port list for httpx
if [[ -s "$OUT/ports.jsonl" ]]; then
  jq -r '"\(.host // .ip):\(.port)"' "$OUT/ports.jsonl" | sort -u > "$OUT/hostports.txt"
else
  cp "$OUT/hosts.txt" "$OUT/hostports.txt"
fi
log "open host:port pairs: $(wc -l < "$OUT/hostports.txt")"

# --- 4b. confirm: second pass to drop connect-scan false positives ---
# Some NAT/ISP middleboxes answer SYNs for closed ports: naabu reports the
# pair open, but the connection never carries a byte. confirm.py probes every
# pair at the application level (banner grab, then a plain HTTP GET) and
# splits them into ports.confirmed.jsonl / ports.unconfirmed.jsonl.
: > "$OUT/ports.confirmed.jsonl"
: > "$OUT/ports.unconfirmed.jsonl"
if [[ "$RUN_CONFIRM" == "1" ]]; then
  if [[ -s "$OUT/ports.jsonl" ]]; then
    log "confirm: probing $(wc -l < "$OUT/hostports.txt") host:port pairs (timeout ${CONFIRM_TIMEOUT}s)"
    python3 "$CONFIRM_SCRIPT" "$OUT/hostports.txt" "$OUT/ports.confirmed.jsonl" \
      "$OUT/ports.unconfirmed.jsonl" "$CONFIRM_TIMEOUT" || true
  else
    log "confirm: nothing to confirm (naabu reported no open ports)"
  fi
else
  log "confirm: skipped (RUN_CONFIRM=0)"
fi

# --- 5. httpx: probe HTTP services ---
log "httpx: probing HTTP(S)"
httpx -l "$OUT/hostports.txt" \
  -sc -title -td -server -ip -cname -tls-grab -location -web-server \
  -json -silent -o "$OUT/http.jsonl" || true

# --- 6. tlsx: TLS certs (SAN, expiry, cipher) ---
log "tlsx: TLS certificate inspection"
tlsx -l "$OUT/hostports.txt" -expired -self-signed -tls-version -cipher \
  -json -silent -o "$OUT/tls.jsonl" || true

# feed SANs back into a candidate list (graph self-expansion, phase 2 seed)
if [[ -s "$OUT/tls.jsonl" ]]; then
  jq -r '.subject_an[]?' "$OUT/tls.jsonl" 2>/dev/null | sort -u > "$OUT/san_discovered.txt" || true
fi

# --- 7. nuclei: exposures / misconfig / takeover (opt-in) ---
if [[ "$RUN_NUCLEI" == "1" ]]; then
  log "nuclei: $NUCLEI_TAGS (this is heavier)"
  if [[ -s "$OUT/http.jsonl" ]]; then
    jq -r '.url' "$OUT/http.jsonl" > "$OUT/urls.txt"
    nuclei -l "$OUT/urls.txt" -tags "$NUCLEI_TAGS" -severity low,medium,high,critical \
      -json-export "$OUT/nuclei.jsonl" -silent || true
  fi
else
  log "nuclei: skipped (set RUN_NUCLEI=1 to enable)"
fi

# --- 8. summary ---
SUMMARY="$OUT/SUMMARY.txt"
{
  echo "blackwall-scan report — $TS"
  echo "================================================"
  echo "Targets:        $(wc -l < "$OUT/targets.raw")"
  echo "Subdomains:     $(wc -l < "$OUT/subs.txt")"
  echo "Hosts scanned:  $(wc -l < "$OUT/hosts.txt")"
  echo "Open host:port: $(wc -l < "$OUT/hostports.txt")"
  if [[ "$RUN_CONFIRM" == "1" ]]; then
    echo "Confirmed:      $(wc -l < "$OUT/ports.confirmed.jsonl")"
    echo "Unconfirmed:    $(wc -l < "$OUT/ports.unconfirmed.jsonl")"
    echo
    echo "--- Confirmed ports (naabu + application-level probe) ---"
    [[ -s "$OUT/ports.confirmed.jsonl" ]] && jq -r '"\(.host):\(.port)  [\(.method)] \(.evidence)"' "$OUT/ports.confirmed.jsonl" || echo "(none)"
    echo
    echo "--- Unconfirmed ports (connect OK, no application-level reply — NAT/ISP faking? check before trusting) ---"
    [[ -s "$OUT/ports.unconfirmed.jsonl" ]] && jq -r '"\(.host):\(.port)  \(.reason)"' "$OUT/ports.unconfirmed.jsonl" || echo "(none)"
  else
    echo
    echo "--- Open ports (unconfirmed — confirm pass skipped, RUN_CONFIRM=0) ---"
    [[ -s "$OUT/ports.jsonl" ]] && jq -r '"\(.host // .ip):\(.port)"' "$OUT/ports.jsonl" | sort -u || echo "(none)"
  fi
  echo
  echo "--- HTTP services (status | title | tech) ---"
  [[ -s "$OUT/http.jsonl" ]] && jq -r '"\(.url)  [\(.status_code // "?")]  \(.title // "")  \((.tech // [])|join(","))"' "$OUT/http.jsonl" || echo "(none)"
  echo
  echo "--- TLS certs (host | CN | expiry) ---"
  [[ -s "$OUT/tls.jsonl" ]] && jq -r '"\(.host):\(.port)  CN=\(.subject_cn // "?")  expires=\(.not_after // "?")"' "$OUT/tls.jsonl" | sort -u || echo "(none)"
  if [[ "$RUN_NUCLEI" == "1" && -s "$OUT/nuclei.jsonl" ]]; then
    echo
    echo "--- nuclei findings ---"
    jq -r '"[\(.info.severity)] \(.info.name) — \(.matched_at // .host)"' "$OUT/nuclei.jsonl" || true
  fi
} > "$SUMMARY"

log "DONE. Report: $OUT"
echo
cat "$SUMMARY"
