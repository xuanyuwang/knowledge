#!/usr/bin/env python3
"""
Sequential backfill for conversation_with_labels — one day at a time.

Handles the full workflow: delete existing data → wait for mutation → create
k8s job → wait for job completion → verify. Tracks progress in JSON for
resume after interruption.

Usage:
    python3 backfill_sequential.py --cluster us-east-1-prod --customer alaska-air
    python3 backfill_sequential.py --cluster us-east-1-prod --customer alaska-air --status
    python3 backfill_sequential.py --cluster us-east-1-prod --customer alaska-air --reset 2026-01-15
    python3 backfill_sequential.py --cluster us-east-1-prod --customer alaska-air --start 2026-01-01 --end 2026-02-19

Prerequisites:
    - VPN connected
    - kubectl access to <cluster>_dev
    - PR go-servers#25706 deployed (ended_at filter fix)

Created: 2026-02-18
"""

import argparse
import json
import os
import signal
import subprocess
import sys
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path

# ---- Config ----

SCRIPT_DIR = Path(__file__).parent
CH_NAMESPACE = "clickhouse"
CH_DATABASE = "conversations"
CRON_NAMESPACE = "cresta-cron"
CRONJOB_NAME = "cron-label-conversations"

MAX_WAIT_PER_DAY = 3600      # 1 hour max wait for k8s job
POLL_INTERVAL = 30            # seconds between job status checks
MUTATION_POLL_INTERVAL = 10   # seconds between mutation checks
MUTATION_TIMEOUT = 600        # 10 minutes max wait for mutation


# ---- Helpers ----

def now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def run(cmd: list[str], timeout: int = 60) -> tuple[int, str, str]:
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired:
        return -1, "", f"Command timed out after {timeout}s"
    except Exception as e:
        return -1, "", str(e)


def date_range(start: str, end: str) -> list[str]:
    """Generate list of date strings from start (inclusive) to end (exclusive)."""
    current = datetime.strptime(start, "%Y-%m-%d")
    end_dt = datetime.strptime(end, "%Y-%m-%d")
    dates = []
    while current < end_dt:
        dates.append(current.strftime("%Y-%m-%d"))
        current += timedelta(days=1)
    return dates


# ---- Tracking ----

def tracking_path(cluster: str, customer: str) -> Path:
    safe_name = f"{cluster}_{customer}" if customer else f"{cluster}_all"
    return SCRIPT_DIR / f"tracking_{safe_name}.json"


def load_tracking(cluster: str, customer: str, start: str, end: str) -> dict:
    path = tracking_path(cluster, customer)
    if path.exists():
        with open(path) as f:
            return json.load(f)
    return init_tracking(cluster, customer, start, end)


def save_tracking(tracking: dict):
    cluster = tracking["cluster"]
    customer = tracking["customer"]
    path = tracking_path(cluster, customer)
    with open(path, "w") as f:
        json.dump(tracking, f, indent=2)


def init_tracking(cluster: str, customer: str, start: str, end: str) -> dict:
    dates = date_range(start, end)
    tracking = {
        "cluster": cluster,
        "customer": customer or "all",
        "start_date": start,
        "end_date": end,
        "created_at": now_iso(),
        "days": {},
    }
    for date_str in dates:
        tracking["days"][date_str] = {
            "status": "pending",
            "delete_done": False,
            "job_name": None,
            "started_at": None,
            "completed_at": None,
            "error": None,
        }
    path = tracking_path(cluster, customer)
    save_tracking(tracking)
    print(f"Initialized tracking: {path}")
    return tracking


# ---- ClickHouse helpers ----

def find_ch_pod(cluster: str) -> str:
    """Discover a ClickHouse pod in the cluster."""
    context = f"{cluster}_dev"
    rc, stdout, _ = run([
        "kubectl", "get", "pods", "-n", CH_NAMESPACE, f"--context={context}",
        "-l", "clickhouse.altinity.com/app=chop",
        "-o", "jsonpath={.items[0].metadata.name}",
    ])
    if rc == 0 and stdout.strip():
        return stdout.strip()

    # Fallback: first pod in clickhouse namespace
    rc, stdout, _ = run([
        "kubectl", "get", "pods", "-n", CH_NAMESPACE, f"--context={context}",
        "-o", "jsonpath={.items[0].metadata.name}",
    ])
    if rc == 0 and stdout.strip():
        return stdout.strip()

    raise RuntimeError(f"No ClickHouse pod found in {CH_NAMESPACE} namespace")


def ch_query(ch_pod: str, context: str, sql: str, timeout: int = 120) -> str:
    """Run a ClickHouse query via kubectl exec."""
    rc, stdout, stderr = run([
        "kubectl", "exec", "-n", CH_NAMESPACE, f"--context={context}", ch_pod,
        "--", "clickhouse-client", f"--query={sql}",
    ], timeout=timeout)
    if rc != 0:
        raise RuntimeError(f"ClickHouse query failed: {stderr}")
    return stdout.strip()


def delete_day(ch_pod: str, context: str, customer: str, date_str: str):
    """Delete existing rows for a single day."""
    next_date = (datetime.strptime(date_str, "%Y-%m-%d") + timedelta(days=1)).strftime("%Y-%m-%d")

    if customer and customer != "all":
        where = (
            f"customer_id = '{customer}' AND "
            f"conversation_end_time >= '{date_str} 00:00:00' AND "
            f"conversation_end_time < '{next_date} 00:00:00'"
        )
    else:
        where = (
            f"conversation_end_time >= '{date_str} 00:00:00' AND "
            f"conversation_end_time < '{next_date} 00:00:00'"
        )

    # Count first
    count = ch_query(ch_pod, context, (
        f"SELECT count() FROM {CH_DATABASE}.conversation_with_labels_d "
        f"WHERE {where}"
    ))
    print(f"    Rows to delete: {count}")

    if count == "0":
        return

    # Execute DELETE
    delete_sql = (
        f"ALTER TABLE {CH_DATABASE}.conversation_with_labels ON CLUSTER 'conversations' "
        f"DELETE WHERE {where} "
        f"SETTINGS replication_wait_for_inactive_replica_timeout = 0"
    )
    ch_query(ch_pod, context, delete_sql, timeout=300)
    print(f"    DELETE mutation submitted")


def wait_for_mutations(ch_pod: str, context: str) -> bool:
    """Wait until all conversation_with_labels mutations complete."""
    elapsed = 0
    while elapsed < MUTATION_TIMEOUT:
        result = ch_query(ch_pod, context, (
            "SELECT count() FROM system.mutations "
            "WHERE table = 'conversation_with_labels' AND is_done = 0"
        ))
        pending = int(result) if result.isdigit() else 0
        if pending == 0:
            return True
        print(f"    Waiting for {pending} mutation(s)... [{elapsed}s]")
        time.sleep(MUTATION_POLL_INTERVAL)
        elapsed += MUTATION_POLL_INTERVAL
    return False


# ---- K8s job helpers ----

def create_job(cluster: str, customer: str, date_str: str) -> str:
    """Create a k8s backfill job for a single day."""
    context = f"{cluster}_dev"
    next_date = (datetime.strptime(date_str, "%Y-%m-%d") + timedelta(days=1)).strftime("%Y-%m-%d")
    start_time = f"{date_str}T00:00:00Z"
    end_time = f"{next_date}T00:00:00Z"

    # Date suffix for job name
    suffix = date_str.replace("-", "")
    customer_tag = customer if customer and customer != "all" else "all"
    job_name = f"backfill-labels-{customer_tag}-{suffix}-{int(time.time())}"

    yaml_raw = f"/tmp/label-job-{suffix}.yaml"
    yaml_final = f"/tmp/label-job-{suffix}-final.yaml"

    # Step 1: Create template
    rc, stdout, stderr = run([
        "kubectl", "create", "job", f"--from=cronjob/{CRONJOB_NAME}",
        job_name, "-n", CRON_NAMESPACE, f"--context={context}",
        "--dry-run=client", "-o", "yaml",
    ])
    if rc != 0:
        raise RuntimeError(f"Failed to create job template: {stderr}")
    with open(yaml_raw, "w") as f:
        f.write(stdout)

    # Step 2: Set env vars
    env_vars = [
        f"ENABLE_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE=true",
        f"LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_START_AT_RANGE_START={start_time}",
        f"LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_END_AT_RANGE_END={end_time}",
    ]
    if customer and customer != "all":
        env_vars.append(
            f"FILTER_CUSTOMER_IN_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE={customer}"
        )

    rc, stdout, stderr = run([
        "kubectl", "set", "env", "--local", f"-f={yaml_raw}",
        *env_vars,
        "-o", "yaml",
    ])
    if rc != 0:
        raise RuntimeError(f"Failed to set env vars: {stderr}")
    with open(yaml_final, "w") as f:
        f.write(stdout)

    # Step 3: Apply
    rc, _, stderr = run([
        "kubectl", "apply", "-f", yaml_final, f"--context={context}",
    ])
    if rc != 0:
        raise RuntimeError(f"Failed to apply job: {stderr}")

    print(f"    Job created: {job_name}")
    return job_name


def wait_for_job(cluster: str, job_name: str) -> tuple[bool, str]:
    """Wait for a k8s job to complete."""
    context = f"{cluster}_dev"
    elapsed = 0

    while elapsed < MAX_WAIT_PER_DAY:
        # Check job status
        rc, stdout, _ = run([
            "kubectl", "get", "job", job_name,
            "-n", CRON_NAMESPACE, f"--context={context}",
            "-o", "jsonpath={.status.conditions[0].type}",
        ])

        if rc == 0:
            condition = stdout.strip()
            if condition == "Complete":
                return True, ""
            elif condition == "Failed":
                # Get failure reason from logs
                _, logs, _ = run([
                    "kubectl", "logs", "-n", CRON_NAMESPACE, f"--context={context}",
                    "-l", f"job-name={job_name}", "--tail=20",
                ], timeout=30)
                return False, f"Job failed. Last logs:\n{logs[-500:]}"

        # Also check succeeded/failed counts
        rc, stdout, _ = run([
            "kubectl", "get", "job", job_name,
            "-n", CRON_NAMESPACE, f"--context={context}",
            "-o", "jsonpath={.status.succeeded},{.status.failed}",
        ])
        if rc == 0:
            parts = stdout.strip().split(",")
            succeeded = int(parts[0]) if parts[0] else 0
            failed = int(parts[1]) if len(parts) > 1 and parts[1] else 0
            if succeeded > 0:
                return True, ""
            if failed > 0:
                return False, f"Job has {failed} failed pod(s)"

        print(f"    Waiting for job... [{elapsed}s]")
        time.sleep(POLL_INTERVAL)
        elapsed += POLL_INTERVAL

    return False, f"Timeout after {MAX_WAIT_PER_DAY}s"


# ---- Commands ----

def cmd_status(cluster: str, customer: str, start: str, end: str):
    tracking = load_tracking(cluster, customer, start, end)
    days = tracking["days"]

    from collections import Counter
    counts = Counter(d["status"] for d in days.values())

    print("============================================================")
    print("Backfill Progress: conversation_with_labels")
    print(f"Cluster:   {tracking['cluster']}")
    print(f"Customer:  {tracking['customer']}")
    print(f"Range:     {tracking['start_date']} to {tracking['end_date']}")
    print("============================================================")

    for status in ["completed", "running", "failed", "pending"]:
        count = counts.get(status, 0)
        if count > 0:
            day_list = sorted(k for k, v in days.items() if v["status"] == status)
            # Show first/last
            if len(day_list) <= 5:
                display = ", ".join(day_list)
            else:
                display = f"{day_list[0]} ... {day_list[-1]}"
            print(f"  {status.upper():12s}: {count:3d}  ({display})")

    print(f"  {'TOTAL':12s}: {sum(counts.values()):3d}")
    print("============================================================")


def cmd_reset(cluster: str, customer: str, start: str, end: str, reset_date: str):
    tracking = load_tracking(cluster, customer, start, end)
    if reset_date not in tracking["days"]:
        print(f"Date {reset_date} not in tracking")
        sys.exit(1)
    tracking["days"][reset_date] = {
        "status": "pending",
        "delete_done": False,
        "job_name": None,
        "started_at": None,
        "completed_at": None,
        "error": None,
    }
    save_tracking(tracking)
    print(f"Day {reset_date} reset to pending")


def cmd_run(cluster: str, customer: str, start: str, end: str, skip_delete: bool):
    tracking = load_tracking(cluster, customer, start, end)
    context = f"{cluster}_dev"

    cmd_status(cluster, customer, start, end)
    print()

    # Discover ClickHouse pod
    if not skip_delete:
        print("Discovering ClickHouse pod...")
        try:
            ch_pod = find_ch_pod(cluster)
            print(f"Using ClickHouse pod: {ch_pod}")
        except RuntimeError as e:
            print(f"ERROR: {e}")
            print("Use --skip-delete if deletion was already done externally.")
            sys.exit(1)
    else:
        ch_pod = None
        print("Skipping delete step (--skip-delete)")

    # Signal handling
    def cleanup(sig=None, frame=None):
        if sig:
            print(f"\nInterrupted. Progress saved to {tracking_path(cluster, customer)}")
            sys.exit(1)

    signal.signal(signal.SIGINT, cleanup)
    signal.signal(signal.SIGTERM, cleanup)

    # Process each day
    sorted_dates = sorted(tracking["days"].keys())
    for date_str in sorted_dates:
        day_info = tracking["days"][date_str]

        if day_info["status"] == "completed":
            continue

        print()
        print("============================================================")
        print(f"Processing: {date_str}")
        print("============================================================")

        # Mark as running
        day_info["status"] = "running"
        day_info["started_at"] = now_iso()
        day_info["error"] = None
        save_tracking(tracking)

        # Step 1: Delete existing data
        if not skip_delete and not day_info.get("delete_done"):
            print("  Step 1: Deleting existing data...")
            try:
                delete_day(ch_pod, context, customer, date_str)
                # Wait for mutation
                print("  Waiting for mutation to complete...")
                if not wait_for_mutations(ch_pod, context):
                    day_info["status"] = "failed"
                    day_info["error"] = "Mutation timeout"
                    save_tracking(tracking)
                    continue
                day_info["delete_done"] = True
                save_tracking(tracking)
                print("    Mutation complete.")
            except RuntimeError as e:
                print(f"    ERROR: {e}")
                day_info["status"] = "failed"
                day_info["error"] = str(e)
                save_tracking(tracking)
                continue
        else:
            print("  Step 1: Delete skipped (already done or --skip-delete)")

        # Step 2: Create k8s job
        print("  Step 2: Creating backfill job...")
        try:
            job_name = create_job(cluster, customer, date_str)
            day_info["job_name"] = job_name
            save_tracking(tracking)
        except RuntimeError as e:
            print(f"    ERROR: {e}")
            day_info["status"] = "failed"
            day_info["error"] = str(e)
            save_tracking(tracking)
            continue

        # Step 3: Wait for completion
        print("  Step 3: Waiting for job completion...")
        success, error = wait_for_job(cluster, job_name)

        if success:
            day_info["status"] = "completed"
            day_info["completed_at"] = now_iso()
            print(f"  COMPLETED: {date_str}")
        else:
            day_info["status"] = "failed"
            day_info["error"] = error
            print(f"  FAILED: {date_str} — {error}")

        save_tracking(tracking)

    print()
    print("============================================================")
    print("Sequential backfill finished!")
    print("============================================================")
    cmd_status(cluster, customer, start, end)


# ---- Main ----

def main():
    parser = argparse.ArgumentParser(
        description="Sequential backfill for conversation_with_labels"
    )
    parser.add_argument("--cluster", required=True, help="Kubernetes cluster (e.g., us-east-1-prod)")
    parser.add_argument("--customer", default="", help="Customer ID (empty = all customers)")
    parser.add_argument("--start", default="2026-01-01", help="Start date YYYY-MM-DD (default: 2026-01-01)")
    parser.add_argument("--end", default="2026-02-19", help="End date YYYY-MM-DD exclusive (default: 2026-02-19)")
    parser.add_argument("--status", action="store_true", help="Show progress")
    parser.add_argument("--reset", metavar="DATE", help="Reset a date to pending (YYYY-MM-DD)")
    parser.add_argument("--skip-delete", action="store_true",
                        help="Skip the ClickHouse delete step (if already done externally)")

    args = parser.parse_args()

    if args.status:
        cmd_status(args.cluster, args.customer, args.start, args.end)
    elif args.reset:
        cmd_reset(args.cluster, args.customer, args.start, args.end, args.reset)
    else:
        cmd_run(args.cluster, args.customer, args.start, args.end, args.skip_delete)


if __name__ == "__main__":
    main()
