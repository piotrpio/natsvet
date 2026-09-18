#!/usr/bin/env bash
# Proves the golangci-lint module plugin reports what the standalone binary
# reports: builds bin/custom-gcl from .custom-gcl.yml, runs it and natsvet
# (every opt-in rule enabled) over the testdata module, normalizes both
# outputs and diffs them. Usage: scripts/plugin.sh <natsvet binary>.
# Needs golangci-lint (any v2) on PATH to run `golangci-lint custom`, which
# fetches the golangci-lint version pinned in .custom-gcl.yml.
set -euo pipefail

BIN=${1:?usage: scripts/plugin.sh <natsvet binary>}
ROOT=$(cd "$(dirname "$0")/.." && pwd)
BIN=$(cd "$(dirname "$BIN")" && pwd)/$(basename "$BIN")
TESTDATA=$ROOT/testdata
PIN=$(sed -n 's/^version: *//p' "$ROOT/.custom-gcl.yml")
CUSTOM=$ROOT/bin/custom-gcl

command -v golangci-lint >/dev/null || {
	echo "plugin: golangci-lint not found on PATH; install v2 (https://golangci-lint.run/docs/welcome/install/)" >&2
	exit 1
}
(cd "$ROOT" && golangci-lint custom)
if ! "$CUSTOM" version 2>&1 | grep -q "$PIN"; then
	echo "plugin: bin/custom-gcl does not report the pinned $PIN:" >&2
	"$CUSTOM" version >&2
	exit 1
fi

# Every -<rule>.enable flag the binary exposes; testdata/.golangci.yml must
# enable the same rules, and the diff below shows when it does not.
OPTIN=$("$BIN" -h 2>&1 | grep -oE '^\s+-[a-z]+\.enable' | tr -d ' \t' | tr '\n' ' ')

PLUGIN=$(mktemp)
WANT=$(mktemp)
trap 'rm -f "$PLUGIN" "$WANT"' EXIT

(
	cd "$TESTDATA"
	# 0: no issues; 1: issues found; anything else is a failure.
	"$CUSTOM" run ./... || [ $? -eq 1 ]
) | sed -E 's/^([^:]+:[0-9]+:[0-9]+: )[a-z]+: /\1/' | sort >"$PLUGIN"

(
	cd "$TESTDATA"
	# shellcheck disable=SC2086
	"$BIN" $OPTIN ./... 2>&1 || true
) | sed "s|^$TESTDATA/||" | sort >"$WANT"

if ! diff -u "$WANT" "$PLUGIN" >&2; then
	echo "plugin: custom-gcl (+) and natsvet (-) disagree on testdata" >&2
	exit 1
fi
echo "plugin: $(wc -l <"$PLUGIN" | tr -d ' ') findings, identical through golangci-lint"
