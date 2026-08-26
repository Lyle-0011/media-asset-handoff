#!/bin/sh
set -eu

base_url="${BASE_URL:-http://localhost:8080}"

curl --fail-with-body --request POST "$base_url/assets" \
  --header 'Content-Type: application/json' \
  --data '{"id":"asset-42","creator_id":"creator-7","filename":"trailer.mp4","content_type":"video/mp4","max_bytes":8388608}'

printf '\nUse upload_url with an HTTP PUT, then confirm the upload:\n'
printf 'curl --fail-with-body --request POST %s/assets/asset-42/uploaded\n' "$base_url"
