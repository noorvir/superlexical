#!/usr/bin/env bash
# Starts Ray for development and starts it again when a source file changes.
#
# The default model uses the login of the Codex CLI. For a Together model,
# such as RAY_MODEL=together/moonshotai/Kimi-K3, Ray needs TOGETHER_API_KEY.
# When it is not set, this script reads op://nos/togetherai-api-key/credential
# with the 1Password agent token. The key goes only to the Ray process.
set -euo pipefail
cd "$(dirname "$0")/.."

token_file="$HOME/.config/op/agent-token"
if [[ "${RAY_MODEL:-}" == together/* ]] && [ -z "${TOGETHER_API_KEY:-}" ] && command -v op >/dev/null && [ -f "$token_file" ]; then
	TOGETHER_API_KEY="$(
		OP_SERVICE_ACCOUNT_TOKEN="$(cat "$token_file")" \
			op read 'op://nos/togetherai-api-key/credential'
	)"
	export TOGETHER_API_KEY
fi

exec node --watch --disable-warning=ExperimentalWarning src/index.ts
