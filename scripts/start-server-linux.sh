#!/usr/bin/env bash
set -euo pipefail
server_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cd -- "$server_dir"
exec "$server_dir/router-server" -repository-dir "$server_dir/data/repository" "$@"
