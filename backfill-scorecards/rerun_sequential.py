#!/usr/bin/env python3
"""
Sequential backfill for cvs and oportun - one day at a time.

Tracks progress in sequential_tracking.json so it can resume after interruption.

Usage:
    python3 rerun_sequential.py              # Run Jan 1-31, skipping completed days
    python3 rerun_sequential.py --status     # Show current progress
    python3 rerun_sequential.py --reset 15   # Reset day 15 to pending (to re-run it)

Prerequisites:
    - VPN connected
    - kubectl access to us-west-2-prod_dev
    - temporal CLI installed
    - Port-forward will be managed automatically

Created: 2026-02-09
"""

import argparse
import json
import os
import signal
import subprocess
import sys
import time
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from pathlib import Path

# ---- Config ----

CLUSTER = "us-west-2-prod"
CONTEXT = f"{CLUSTER}_dev"
CUSTOMERS = "cvs,oportun"
TEMPORAL_NS = "ingestion"
TEMPORAL_ADDR = "localhost:7233"
SCRIPT_DIR = Path(__file__).parent
TRACKING_FILE = SCRIPT_DIR / "sequential_tracking.json"

MAX_WAIT_PER_DAY = 3600  # 1 hour
POLL_INTERVAL = 30  # seconds
WORKFLOW_DISCOVERY_WAIT = 15  # seconds after job creation


# ---- Tracking ----

def load_tracking() -> dict:
    if TRACKING_FILE.exists():
        with open(TRACKING_FILE) as f:
            return json.load(f)
    return init_tracking()


def save_tracking(data: dict):
    with open(TRACKING_FILE, "w") as f:
        json.dump(data, f, indent=2)


def init_tracking() -> dict:
    tracking = {
        "cluster": CLUSTER,
        "customers": CUSTOMERS,
        "created_at": now_iso(),
        "days": {},
    }
    for day in range(1, 32):
        tracking["days"][str(day)] = {
            "date": f"2026-01-{day:02d}",
            "status": "completed" if day == 1 else "pending",
            "job_name": None,
            "workflow_ids": [],
            "started_at": "2026-02-09T14:00:00Z" if day == 1 else None,
            "completed_at": "2026-02-09T14:00:00Z" if day == 1 else None,
            "error": None,
        }
    save_tracking(tracking)
    print(f"Initialized tracking file: {TRACKING_FILE}")
    return tracking


def now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


# ---- Shell helpers ----

def run(cmd: list[str], timeout: int = 60) -> tuple[int, str, str]:
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired:
        return -1, "", f"Command timed out after {timeout}s"
    except Exception as e:
        return -1, "", str(e)


# ---- Port-forward management ----

class PortForward:
    def __init__(self):
        self.proc = None

    def start(self):
        self.stop()
        time.sleep(2)
        self.proc = subprocess.Popen(
            [
                "kubectl", f"--context={CONTEXT}", "-n", "temporal",
                "port-forward", "svc/temporal-frontend-headless", "7233:7233",
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        time.sleep(5)
        print(f"Port-forward started (pid {self.proc.pid})")

    def stop(self):
        # Kill any existing port-forwards on 7233
        subprocess.run(
            ["pkill", "-f", "port-forward.*7233"],
            capture_output=True,
        )
        if self.proc:
            self.proc.kill()
            self.proc = None

    def ensure_alive(self):
        if self.proc is None or self.proc.poll() is not None:
            print("  Port-forward died, restarting...")
            self.start()


# ---- Temporal helpers ----

def get_workflow_status(workflow_id: str) -> str:
    rc, stdout, _ = run([
        "temporal", "workflow", "describe",
        "--namespace", TEMPORAL_NS,
        "--address", TEMPORAL_ADDR,
        "--workflow-id", workflow_id,
        "--output", "json",
    ])
    if rc != 0 or not stdout:
        return "UNKNOWN"
    try:
        data = json.loads(stdout)
        return data.get("workflowExecutionInfo", {}).get("status", "UNKNOWN")
    except json.JSONDecodeError:
        return "UNKNOWN"


def find_recent_workflows(max_age_minutes: int = 2) -> list[str]:
    """Find running cvs/oportun workflows started within max_age_minutes."""
    rc, stdout, _ = run(
        [
            "temporal", "workflow", "list",
            "--namespace", TEMPORAL_NS,
            "--address", TEMPORAL_ADDR,
            "--query",
            'ExecutionStatus = "Running" AND '
            '(WorkflowId STARTS_WITH "reindexconversations-cvs-us-west-2" OR '
            'WorkflowId STARTS_WITH "reindexconversations-oportun-us-west-2")',
            "--output", "json",
        ],
        timeout=30,
    )
    if rc != 0 or not stdout:
        return []

    try:
        data = json.loads(stdout)
    except json.JSONDecodeError:
        return []

    cutoff = datetime.now(timezone.utc) - timedelta(minutes=max_age_minutes)
    result = []
    for wf in data:
        start_str = wf.get("startTime", "")
        if not start_str:
            continue
        start = datetime.fromisoformat(start_str.replace("Z", "+00:00"))
        if start > cutoff:
            wf_id = wf.get("execution", {}).get("workflowId", "")
            if wf_id:
                result.append(wf_id)
    return result


# ---- Job creation ----

def create_job(day: int) -> str:
    date_str = f"2026-01-{day:02d}"
    if day == 31:
        next_date = "2026-02-01"
    else:
        next_date = f"2026-01-{day + 1:02d}"

    start_time = f"{date_str}T00:00:00Z"
    end_time = f"{next_date}T00:00:00Z"
    suffix = f"jan{day:02d}"
    job_name = f"batch-reindex-seq-{suffix}-{int(time.time())}"

    yaml_raw = f"/tmp/reindex-job-{suffix}.yaml"
    yaml_final = f"/tmp/reindex-job-{suffix}-final.yaml"

    # Generate from cronjob template
    rc, _, stderr = run([
        "kubectl", "create", "job", f"--from=cronjob/cron-batch-reindex-conversations",
        job_name, "-n", "cresta-cron", f"--context={CONTEXT}",
        "--dry-run=client", "-o", "yaml",
    ])
    if rc != 0:
        raise RuntimeError(f"Failed to create job template: {stderr}")

    # Write raw yaml (captured from stdout would be better, but kubectl writes to stdout)
    rc, stdout, stderr = run([
        "kubectl", "create", "job", f"--from=cronjob/cron-batch-reindex-conversations",
        job_name, "-n", "cresta-cron", f"--context={CONTEXT}",
        "--dry-run=client", "-o", "yaml",
    ])
    if rc != 0:
        raise RuntimeError(f"Failed to create job template: {stderr}")
    with open(yaml_raw, "w") as f:
        f.write(stdout)

    # Set env vars
    rc, stdout, stderr = run([
        "kubectl", "set", "env", "--local", f"-f={yaml_raw}",
        f"REINDEX_START_TIME={start_time}",
        f"REINDEX_END_TIME={end_time}",
        f"RUN_ONLY_FOR_CUSTOMER_IDS={CUSTOMERS}",
        "-o", "yaml",
    ])
    if rc != 0:
        raise RuntimeError(f"Failed to set env: {stderr}")
    with open(yaml_final, "w") as f:
        f.write(stdout)

    # Apply
    rc, _, stderr = run([
        "kubectl", "apply", "-f", yaml_final, f"--context={CONTEXT}",
    ])
    if rc != 0:
        raise RuntimeError(f"Failed to apply job: {stderr}")

    print(f"  Job created: {job_name}")
    return job_name


# ---- Wait for completion ----

def wait_for_workflows(workflow_ids: list[str], pf: PortForward) -> tuple[bool, str]:
    """
    Poll workflows until all are done or timeout.
    Returns (success, error_message).
    """
    elapsed = 0

    while elapsed < MAX_WAIT_PER_DAY:
        pf.ensure_alive()

        all_done = True
        any_failed = False
        status_parts = []

        for wf_id in workflow_ids:
            status = get_workflow_status(wf_id)
            short = status.split("_")[-1] if "_" in status else status
            # Use last 8 chars of UUID for display
            wf_short = wf_id.rsplit("-", 1)[-1][:8] if "-" in wf_id else wf_id[:8]
            status_parts.append(f"{wf_short}={short}")

            if "RUNNING" in status:
                all_done = False
            elif "FAILED" in status or "TIMED_OUT" in status or "CANCELED" in status:
                any_failed = True

        if all_done:
            if any_failed:
                msg = f"Workflows finished with failures: {', '.join(status_parts)}"
                print(f"  WARNING: {msg}")
                return False, msg
            print(f"  All workflows completed!")
            return True, ""

        print(f"  [{elapsed}s] {' | '.join(status_parts)}")
        time.sleep(POLL_INTERVAL)
        elapsed += POLL_INTERVAL

    return False, f"Timeout after {MAX_WAIT_PER_DAY}s"


# ---- Commands ----

def cmd_status():
    tracking = load_tracking()
    days = tracking["days"]

    from collections import Counter
    counts = Counter(d["status"] for d in days.values())

    print("============================================================")
    print("Sequential Backfill Progress")
    print(f"Cluster:   {tracking['cluster']}")
    print(f"Customers: {tracking['customers']}")
    print("============================================================")

    for status in ["completed", "running", "failed", "pending"]:
        count = counts.get(status, 0)
        if count > 0:
            day_nums = sorted(int(k) for k, v in days.items() if v["status"] == status)
            # Compact range display
            ranges = _compact_ranges(day_nums)
            print(f"  {status.upper():12s}: {count:3d}  (Jan {ranges})")

    print(f"  {'TOTAL':12s}: {sum(counts.values()):3d}")
    print("============================================================")


def _compact_ranges(nums: list[int]) -> str:
    """Turn [1,2,3,5,7,8,9] into '1-3, 5, 7-9'."""
    if not nums:
        return ""
    ranges = []
    start = nums[0]
    end = nums[0]
    for n in nums[1:]:
        if n == end + 1:
            end = n
        else:
            ranges.append(f"{start}-{end}" if start != end else str(start))
            start = end = n
    ranges.append(f"{start}-{end}" if start != end else str(start))
    return ", ".join(ranges)


def cmd_reset(day: int):
    tracking = load_tracking()
    key = str(day)
    if key not in tracking["days"]:
        print(f"Invalid day: {day}")
        sys.exit(1)
    tracking["days"][key]["status"] = "pending"
    tracking["days"][key]["error"] = None
    tracking["days"][key]["job_name"] = None
    tracking["days"][key]["workflow_ids"] = []
    tracking["days"][key]["started_at"] = None
    tracking["days"][key]["completed_at"] = None
    save_tracking(tracking)
    print(f"Day {day} reset to pending")


def cmd_run():
    tracking = load_tracking()
    cmd_status()
    print()

    pf = PortForward()

    def cleanup(sig=None, frame=None):
        pf.stop()
        if sig:
            print(f"\nInterrupted. Progress saved to {TRACKING_FILE}")
            sys.exit(1)

    signal.signal(signal.SIGINT, cleanup)
    signal.signal(signal.SIGTERM, cleanup)

    pf.start()

    try:
        for day in range(1, 32):
            key = str(day)
            day_info = tracking["days"][key]

            if day_info["status"] == "completed":
                continue

            date_str = day_info["date"]
            print()
            print("============================================================")
            print(f"Day {day}: {date_str}")
            print("============================================================")

            # Update tracking: running
            day_info["status"] = "running"
            day_info["started_at"] = now_iso()
            day_info["error"] = None
            save_tracking(tracking)

            # Create k8s job
            try:
                job_name = create_job(day)
                day_info["job_name"] = job_name
                save_tracking(tracking)
            except RuntimeError as e:
                print(f"  ERROR creating job: {e}")
                day_info["status"] = "failed"
                day_info["error"] = str(e)
                save_tracking(tracking)
                continue

            # Wait for workflows to spawn
            print(f"  Waiting {WORKFLOW_DISCOVERY_WAIT}s for workflows to spawn...")
            time.sleep(WORKFLOW_DISCOVERY_WAIT)

            # Discover workflows
            pf.ensure_alive()
            workflow_ids = find_recent_workflows(max_age_minutes=2)

            if not workflow_ids:
                print("  No running workflows found. May have completed instantly.")
                day_info["status"] = "completed"
                day_info["completed_at"] = now_iso()
                save_tracking(tracking)
                continue

            print(f"  Found {len(workflow_ids)} workflow(s):")
            for wf_id in workflow_ids:
                print(f"    - {wf_id}")

            day_info["workflow_ids"] = workflow_ids
            save_tracking(tracking)

            # Wait for completion
            success, error = wait_for_workflows(workflow_ids, pf)

            if success:
                day_info["status"] = "completed"
                day_info["completed_at"] = now_iso()
            else:
                day_info["status"] = "failed"
                day_info["error"] = error

            save_tracking(tracking)
            print(f"  Day {day}: {day_info['status']}")

    finally:
        cleanup()

    print()
    print("============================================================")
    print("Sequential run finished!")
    print("============================================================")
    cmd_status()


# ---- Main ----

def main():
    parser = argparse.ArgumentParser(description="Sequential backfill for cvs/oportun")
    parser.add_argument("--status", action="store_true", help="Show progress")
    parser.add_argument("--reset", type=int, metavar="DAY", help="Reset a day to pending")
    args = parser.parse_args()

    if args.status:
        cmd_status()
    elif args.reset is not None:
        cmd_reset(args.reset)
    else:
        cmd_run()


if __name__ == "__main__":
    main()
