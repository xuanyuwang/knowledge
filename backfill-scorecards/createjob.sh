#!/bin/zsh
CLUSTER=us-east-1-prod_dev
CUSTOMER=sunbit

kubectl create job --from=cronjob/cron-batch-reindex-conversations \
  batch-reindex-conversations-${CUSTOMER}-$(date +%s) \
  -n cresta-cron \
  --context=${CLUSTER} \
  --dry-run=client -o yaml > /tmp/reindex-job.yaml

kubectl set env --local -f /tmp/reindex-job.yaml \
  REINDEX_START_TIME="2025-12-01T00:00:00Z" \
  REINDEX_END_TIME="2026-02-05T00:00:00Z" \
  RUN_ONLY_FOR_CUSTOMER_IDS="${CUSTOMER}" \
  -o yaml > /tmp/reindex-job-with-env.yaml

kubectl apply -f /tmp/reindex-job-with-env.yaml --context=${CLUSTER}
