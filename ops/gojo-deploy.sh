#!/bin/bash
set -euo pipefail
export PATH=/usr/local/bin:/usr/bin:/bin

current_user="$(id -un)"
if [[ "$current_user" == gojo-deploy && $# -eq 1 && "$1" == --ssh ]]; then
  revision="${SSH_ORIGINAL_COMMAND:-}"
  [[ "$revision" =~ ^[0-9a-f]{40}$ ]] || { echo 'Expected one commit SHA.' >&2; exit 1; }
  printf '%s\n' "$revision" | /usr/bin/sudo -n -u ubuntu -- /usr/local/sbin/gojo-deploy
  exit
fi

if [[ "$current_user" != ubuntu || $# -ne 0 ]]; then
  echo 'Run the installed command as ubuntu and send one commit SHA on stdin.' >&2
  exit 1
fi
IFS= read -r revision
[[ "$revision" =~ ^[0-9a-f]{40}$ ]] || { echo 'Expected one commit SHA.' >&2; exit 1; }

cd /home/ubuntu/w0r_oj
state_dir=/home/ubuntu/.local/state/gojo-deploy
mkdir -p "$state_dir"
exec 9>"$state_dir/deploy.lock"
flock -w 60 9

git fetch --no-tags origin refs/heads/main
if [[ "$revision" != "$(git rev-parse --verify FETCH_HEAD)" ]]; then
  echo "Skipping $revision because it is no longer the tip of main."
  exit 0
fi
if [[ "$(git branch --show-current)" != main || -n "$(git status --porcelain)" ]]; then
  echo 'Expected a clean production checkout on main.' >&2
  exit 1
fi
previous_checkout="$(git rev-parse --verify HEAD)"
git merge --ff-only "$revision"
if [[ "$(git rev-parse --verify HEAD)" != "$revision" ]]; then
  echo 'Production checkout did not reach the requested commit.' >&2
  exit 1
fi

compose=(docker compose --project-name gojo-local-production --env-file .env.local-production
  -f docker-compose.prod.yml -f docker-compose.4g.yml)

echo "Deploying $revision from checkout $previous_checkout."
"${compose[@]}" config --quiet
"${compose[@]}" --parallel 1 build
"${compose[@]}" up --no-build --wait --wait-timeout 180
# Nginx resolves the API address when it starts. Refresh it after a rollout.
"${compose[@]}" restart --no-deps gateway
gateway_address="$("${compose[@]}" port gateway 80)"
if [[ ! "$gateway_address" =~ ^127\.0\.0\.1:[0-9]+$ ]]; then
  echo 'Expected a single loopback HTTP port for the Gojo gateway.' >&2
  exit 1
fi
curl --fail --silent --show-error --retry 12 --retry-all-errors --retry-delay 2 \
  --max-time 5 --output /dev/null "http://$gateway_address/ping"

printf '%s\n' "$revision" > "$state_dir/deployed-sha.tmp"
mv "$state_dir/deployed-sha.tmp" "$state_dir/deployed-sha"
echo "Deployed $revision."
