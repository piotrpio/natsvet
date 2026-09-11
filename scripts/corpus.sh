#!/usr/bin/env bash
# Runs natsvet over the pinned repositories in corpus.txt and diffs the
# findings against corpus.expected. Every opt-in rule except the inventory
# rules is enabled; inventory rules are counted, never diffed.
#
# Usage: scripts/corpus.sh <natsvet binary>
# Environment: CORPUS_DIR caches clones (default ~/.cache/natsvet-corpus).
set -euo pipefail

BIN=$(cd "$(dirname "${1:?usage: corpus.sh <natsvet binary>}")" && pwd)/$(basename "$1")
HERE=$(cd "$(dirname "$0")" && pwd)
CORPUS_DIR=${CORPUS_DIR:-$HOME/.cache/natsvet-corpus}
INVENTORY="legacyjs"

ACTUAL=$(mktemp)
WANT=$(mktemp)
trap 'rm -f "$ACTUAL" "$WANT"' EXIT

# Every -<rule>.enable flag the binary exposes, minus the inventory rules.
optin_flags() {
	"$BIN" -h 2>&1 | grep -oE '^\s+-[a-z]+\.enable' | tr -d ' \t' | while read -r f; do
		rule=${f#-}; rule=${rule%.enable}
		case " $INVENTORY " in *" $rule "*) ;; *) printf '%s ' "$f" ;; esac
	done
}

inventory_flags() {
	for rule in $INVENTORY; do printf -- '-%s.enable ' "$rule"; done
}

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

OPTIN=$(optin_flags)
INV=$(inventory_flags)

{ grep -v '^\s*#' "$HERE/corpus.txt" || true; } | { grep -v '^\s*$' || true; } | while read -r -a fields; do
	# Leading NAME=value tokens are options for this entry.
	modfile=""
	while [[ ${fields[0]} == *=* ]]; do
		case ${fields[0]} in
		modfile=*) modfile=${fields[0]#modfile=} ;;
		*) echo "corpus.txt: unknown option ${fields[0]}" >&2; exit 1 ;;
		esac
		fields=("${fields[@]:1}")
	done
	url=${fields[0]}
	commit=${fields[1]}
	subdir=${fields[2]:-.}
	patterns=("${fields[@]:3}")
	[ ${#patterns[@]} -eq 0 ] && patterns=(./...)
	dir=$(fetch "$url" "$commit")
	name=$(basename "$dir")
	if [ -n "$modfile" ]; then
		cp "$dir/$subdir/$modfile" "$dir/$subdir/go.mod"
		[ -f "$dir/$subdir/${modfile%.mod}.sum" ] && cp "$dir/$subdir/${modfile%.mod}.sum" "$dir/$subdir/go.sum"
	fi
	(
		cd "$dir/$subdir"
		go mod download >/dev/null 2>&1 || true
		# shellcheck disable=SC2086
		"$BIN" $OPTIN "${patterns[@]}" 2>&1 || true
	) | sed "s|^$dir/|$name/|" >>"$ACTUAL"
	n=$(
		cd "$dir/$subdir"
		# shellcheck disable=SC2086
		"$BIN" $INV "${patterns[@]}" 2>&1 | grep -c 'legacy JetStream API:' || true
	)
	echo "inventory: $name/$subdir legacyjs=$n" >&2
done

sort -o "$ACTUAL" "$ACTUAL"
{ grep -v '^\s*#' "$HERE/corpus.expected" || true; } | { grep -v '^\s*$' || true; } | sed -E 's/[[:space:]]+# (TP|FP)(:.*)?$//' | sort >"$WANT"

if ! diff -u "$WANT" "$ACTUAL" >&2; then
	echo "corpus: findings differ from scripts/corpus.expected; triage the lines above" >&2
	exit 1
fi
echo "corpus: $(wc -l <"$ACTUAL" | tr -d ' ') findings, all triaged"
