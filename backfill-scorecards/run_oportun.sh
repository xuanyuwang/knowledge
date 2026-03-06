#!/bin/bash
set -e
export AWS_REGION=us-west-2
export PG_CONN="$(cresta-cli connstring -i --read-only us-west-2-prod us-west-2-prod oportun)"
export CH_HOST=clickhouse-conversations.us-west-2-prod.internal.cresta.ai
export CH_PASSWORD="${CH_PASSWORD:?Set CH_PASSWORD env var}"
export CH_DATABASE=oportun_us_west_2
source .venv/bin/activate
exec python3 backfill_process_scorecards.py --customer oportun --profile us-west-2 "$@"
