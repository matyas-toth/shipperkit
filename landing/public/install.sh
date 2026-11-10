#!/usr/bin/env bash
# Fallback for static hosts that cannot redirect /install.sh.
# Netlify and Cloudflare Pages use public/_redirects; Vercel uses vercel.json.
set -Eeuo pipefail
url="https://raw.githubusercontent.com/matyas-toth/shipperkit/latest/install.sh"
script=$(curl -fsSL --proto '=https' --retry 3 "$url")
exec bash -c "$script"
