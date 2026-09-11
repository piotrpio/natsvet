#!/usr/bin/env bash
# Fails when a .go file does not start with the project license header.
set -euo pipefail
want='// Copyright 2026 Synadia Communications Inc.'
status=0
while IFS= read -r f; do
	if [ "$(head -n 1 "$f")" != "$want" ]; then
		echo "missing license header: $f"
		status=1
	fi
done < <(git ls-files --cached --others --exclude-standard '*.go')
exit $status
