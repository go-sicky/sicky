#!/bin/sh
# govulncheck has no allowlist flag (checked against `govulncheck -h`: no
# -ignore, no -ignorevuln), so this diffs its findings against .vuln-allow.
#
# Rationale for each allowed entry lives in AGENTS.md 6.1. A new advisory must
# fail this script — that is the whole point of keeping the list in a file
# rather than in the Makefile.
set -eu

allow_file="${VULN_ALLOW_FILE:-.vuln-allow}"

log=$(mktemp)
allow=$(mktemp)
trap 'rm -f "$log" "$allow"' EXIT

if govulncheck ./... >"$log" 2>&1; then
	echo "vuln: clean, no findings"
	exit 0
fi

: >"$allow"
if [ -f "$allow_file" ]; then
	# Strip comments and blanks, keep the first whitespace-separated field.
	grep -vE '^[[:space:]]*(#|$)' "$allow_file" | awk 'NF { print $1 }' | sort -u >"$allow"
fi

found=$(grep -oE 'GO-[0-9]{4}-[0-9]+' "$log" | sort -u || true)
if [ -z "$found" ]; then
	# govulncheck failed for a reason other than a finding (network, parse).
	echo "vuln: govulncheck failed without reporting a vulnerability:"
	cat "$log"
	exit 1
fi

unexpected=$(printf '%s\n' "$found" | grep -vxF -f "$allow" | grep -v '^$' || true)
if [ -n "$unexpected" ]; then
	echo "vuln: findings not present in $allow_file:"
	printf '  %s\n' $unexpected
	echo
	echo "run 'govulncheck ./...' for the full trace, then either upgrade the"
	echo "dependency or add the ID to $allow_file with a one-line reason."
	exit 1
fi

echo "vuln: OK — $(printf '%s\n' "$found" | wc -l | tr -d ' ') known advisories allowed, none new"