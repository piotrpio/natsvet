#!/usr/bin/env bash
# Plans the legacy JetStream migration of corpus repositories and prints a
# summary of each plan: sites by class, components (blocked, skipped,
# safe in one commit), steps with machine edits, and pending decisions.
# Nothing is applied; the plans are reviewed by hand.
#
# Usage: scripts/migrate-corpus.sh <natsvet binary> [repository name...]
# (default: go-choria eventing-natss natscli). Needs jq.
# Environment: CORPUS_DIR caches clones (default ~/.cache/natsvet-corpus);
# PLAN_DIR keeps each plan's JSON and Markdown when set.
set -euo pipefail

BIN=$(cd "$(dirname "${1:?usage: migrate-corpus.sh <natsvet binary> [name...]}")" && pwd)/$(basename "$1")
shift
HERE=$(cd "$(dirname "$0")" && pwd)
CORPUS_DIR=${CORPUS_DIR:-$HOME/.cache/natsvet-corpus}
NAMES=("$@")
[ ${#NAMES[@]} -eq 0 ] && NAMES=(go-choria eventing-natss natscli)

# fetch <url> <commit>: prints the clone directory, at the pinned commit.
fetch() {
	local url=$1 commit=$2 name dir
	name=$(basename "$url" .git)
	dir=$CORPUS_DIR/$name
	if [ ! -d "$dir/.git" ]; then
		mkdir -p "$dir"
		git -C "$dir" init -q
		git -C "$dir" remote add origin "$url"
	fi
	if [ "$(git -C "$dir" rev-parse HEAD 2>/dev/null || true)" != "$commit" ]; then
		git -C "$dir" fetch -q --depth 1 origin "$commit"
		git -C "$dir" checkout -q -f --detach FETCH_HEAD
	fi
	echo "$dir"
}

PLAN=$(mktemp "${TMPDIR:-/tmp}/migrate-corpus.XXXXXX")
trap 'rm -f "$PLAN"' EXIT

for want in "${NAMES[@]}"; do
	line=$({ grep -v '^\s*#' "$HERE/corpus.txt" || true; } | grep -E "/$want(\.git)? " | head -1)
	if [ -z "$line" ]; then
		echo "migrate-corpus: $want is not in scripts/corpus.txt" >&2
		exit 1
	fi
	read -r -a fields <<<"$line"
	while [[ ${fields[0]} == *=* ]]; do fields=("${fields[@]:1}"); done
	url=${fields[0]}
	commit=${fields[1]}
	subdir=${fields[2]:-.}
	patterns=("${fields[@]:3}")
	[ ${#patterns[@]} -eq 0 ] && patterns=(./...)
	dir=$(fetch "$url" "$commit")
	echo "== $want ($commit)"
	(cd "$dir/$subdir" && go mod download >/dev/null 2>&1 || true)
	if ! (cd "$dir/$subdir" && "$BIN" migrate plan "${patterns[@]}") >"$PLAN"; then
		echo "migrate-corpus: planning $want failed" >&2
		exit 1
	fi
	if [ -n "${PLAN_DIR:-}" ]; then
		mkdir -p "$PLAN_DIR"
		cp "$PLAN" "$PLAN_DIR/$want.json"
		(cd "$dir/$subdir" && "$BIN" migrate plan -format markdown "${patterns[@]}") >"$PLAN_DIR/$want.md"
	fi
	jq -r '
		"nats.go \(.module_nats_version) (table \(.table_nats_version))",
		"legacy uses \(.counts.legacy_uses) in \(.counts.sites) sites: mechanical \(.counts.mechanical), guided \(.counts.guided), decision \(.counts.decision), unmapped \(.counts.unmapped), skipped \(.counts.skipped)",
		"components \(.components | length): blocked \([.components[] | select(.blocked)] | length), skipped \([.components[] | select(.skipped)] | length), one commit \([.components[] | select(.one_commit)] | length)",
		"steps \(.steps | length): machine \([.steps[] | select(.machine)] | length), waiting \([.steps[] | select(.waits_on)] | length)",
		"pending: \([.pending_decisions[] | "\(.pattern)@\(.scope)=\(.default)×\(.sites)"] | join(", "))",
		"guided or unmapped, by reason:",
		([.sites[] | select(.class == "guided" or .class == "unmapped") | "  \(.class): \((.facts // .notes // ["?"])[0])"] | group_by(.) | map("\(length)× \(.[0])") | .[]),
		"blocked at:",
		(.components[] | select(.blocked) | .blocked[] | "  \(.position.file):\(.position.line): \(.reason)")
	' "$PLAN"
done
