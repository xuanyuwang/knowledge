#!/usr/bin/env python3
"""
Backfill scorecards for all customers across all clusters.

This script:
1. Creates k8s jobs from the cron-batch-reindex-conversations cronjob
2. Waits for job logs and parses temporal workflow info
3. Outputs tracking data to a JSON file for later status queries

Created: 2026-02-07
Updated: 2026-02-07

Usage:
    python3 backfill_all.py --config config.json
    python3 backfill_all.py --config config.json --dry-run
    python3 backfill_all.py --config config.json --cluster us-east-1-prod --customer sunbit
"""

import argparse
import json
import os
import re
import subprocess
import sys
import time
from dataclasses import dataclass, asdict
from datetime import datetime
from pathlib import Path
from typing import Optional


# Configuration
NAMESPACE = "cresta-cron"
CRONJOB_NAME = "cron-batch-reindex-conversations"
LOG_PATTERN = re.compile(
    r'Created reindex conversations job: name=([^,]+), execution_id=([^,]+), cluster=(\S+)'
)

# Default time range for backfill
DEFAULT_START_TIME = "2026-01-01T00:00:00Z"
DEFAULT_END_TIME = "2026-02-01T00:00:00Z"


@dataclass
class JobInfo:
    """Tracking info for a single backfill job."""
    customer: str
    profile: str
    cluster: str
    k8s_job_name: str
    job_resource_name: Optional[str] = None
    temporal_workflow_id: Optional[str] = None
    temporal_cluster: Optional[str] = None
    status: str = "pending"
    created_at: str = ""
    error: Optional[str] = None


def run_cmd(cmd: list[str], timeout: int = 60) -> tuple[int, str, str]:
    """Run a shell command and return (returncode, stdout, stderr)."""
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=timeout
        )
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired:
        return -1, "", f"Command timed out after {timeout}s"
    except Exception as e:
        return -1, "", str(e)


def create_job(
    cluster: str,
    customer: str,
    start_time: str,
    end_time: str,
    dry_run: bool = False
) -> tuple[bool, str, str]:
    """
    Create a k8s job for a customer.

    Returns: (success, job_name, error_message)
    """
    context = f"{cluster}_dev"
    timestamp = int(time.time())
    job_name = f"batch-reindex-conversations-{customer}-{timestamp}"

    # Step 1: Create job YAML from cronjob template
    create_cmd = [
        "kubectl", "create", "job",
        "--from", f"cronjob/{CRONJOB_NAME}",
        job_name,
        "-n", NAMESPACE,
        f"--context={context}",
        "--dry-run=client", "-o", "yaml"
    ]

    rc, stdout, stderr = run_cmd(create_cmd)
    if rc != 0:
        return False, job_name, f"Failed to create job template: {stderr}"

    # Write base job yaml to temp file
    tmp_base = Path("/tmp/reindex-job-base.yaml")
    tmp_base.write_text(stdout)

    # Step 2: Set environment variables
    env_cmd = [
        "kubectl", "set", "env",
        "--local", "-f", str(tmp_base),
        f"REINDEX_START_TIME={start_time}",
        f"REINDEX_END_TIME={end_time}",
        f"RUN_ONLY_FOR_CUSTOMER_IDS={customer}",
        "-o", "yaml"
    ]

    rc, stdout, stderr = run_cmd(env_cmd)
    if rc != 0:
        return False, job_name, f"Failed to set env vars: {stderr}"

    tmp_final = Path("/tmp/reindex-job-final.yaml")
    tmp_final.write_text(stdout)

    if dry_run:
        print(f"  [DRY-RUN] Would create job: {job_name}")
        return True, job_name, ""

    # Step 3: Apply the job
    apply_cmd = [
        "kubectl", "apply",
        "-f", str(tmp_final),
        f"--context={context}"
    ]

    rc, stdout, stderr = run_cmd(apply_cmd)
    if rc != 0:
        return False, job_name, f"Failed to apply job: {stderr}"

    print(f"  Created k8s job: {job_name}")
    return True, job_name, ""


def wait_for_job_logs(
    cluster: str,
    job_name: str,
    max_wait: int = 300,
    poll_interval: int = 10
) -> tuple[bool, str]:
    """
    Wait for a job's pod to start and return its logs.

    Returns: (success, logs_or_error)
    """
    context = f"{cluster}_dev"
    start_time = time.time()

    while time.time() - start_time < max_wait:
        # Get pods for this job
        get_pods_cmd = [
            "kubectl", "get", "pods",
            "-n", NAMESPACE,
            f"--context={context}",
            "-l", f"job-name={job_name}",
            "-o", "jsonpath={.items[0].metadata.name}"
        ]

        rc, pod_name, stderr = run_cmd(get_pods_cmd)
        if rc != 0 or not pod_name.strip():
            print(f"  Waiting for pod to be created... ({int(time.time() - start_time)}s)")
            time.sleep(poll_interval)
            continue

        pod_name = pod_name.strip()

        # Check if pod is running or completed
        get_status_cmd = [
            "kubectl", "get", "pod", pod_name,
            "-n", NAMESPACE,
            f"--context={context}",
            "-o", "jsonpath={.status.phase}"
        ]

        rc, phase, stderr = run_cmd(get_status_cmd)
        phase = phase.strip()

        if phase in ["Running", "Succeeded", "Failed"]:
            # Try to get logs
            logs_cmd = [
                "kubectl", "logs", pod_name,
                "-n", NAMESPACE,
                f"--context={context}"
            ]

            rc, logs, stderr = run_cmd(logs_cmd, timeout=120)
            if rc == 0 and logs:
                return True, logs

            if phase == "Failed":
                return False, f"Pod failed. stderr: {stderr}"

        print(f"  Pod status: {phase}, waiting... ({int(time.time() - start_time)}s)")
        time.sleep(poll_interval)

    return False, f"Timeout waiting for job logs after {max_wait}s"


def parse_job_logs(logs: str) -> tuple[Optional[str], Optional[str], Optional[str]]:
    """
    Parse job logs to extract temporal workflow info.

    Returns: (job_name, execution_id, cluster) or (None, None, None)
    """
    for line in logs.split('\n'):
        match = LOG_PATTERN.search(line)
        if match:
            return match.group(1), match.group(2), match.group(3)
    return None, None, None


def process_customer(
    cluster: str,
    customer: str,
    profile: str,
    start_time: str,
    end_time: str,
    dry_run: bool = False,
    skip_logs: bool = False
) -> JobInfo:
    """Process a single customer - create job and collect info."""
    job_info = JobInfo(
        customer=customer,
        profile=profile,
        cluster=cluster,
        k8s_job_name="",
        created_at=datetime.utcnow().isoformat() + "Z"
    )

    print(f"\nProcessing {customer}/{profile} on {cluster}...")

    # Create the job
    success, job_name, error = create_job(
        cluster, customer, start_time, end_time, dry_run
    )
    job_info.k8s_job_name = job_name

    if not success:
        job_info.status = "failed"
        job_info.error = error
        print(f"  ERROR: {error}")
        return job_info

    if dry_run:
        job_info.status = "dry-run"
        return job_info

    job_info.status = "created"

    if skip_logs:
        print("  Skipping log collection (--skip-logs)")
        return job_info

    # Wait for and parse logs
    print("  Waiting for job logs...")
    success, logs_or_error = wait_for_job_logs(cluster, job_name)

    if not success:
        job_info.error = logs_or_error
        print(f"  WARNING: Could not get logs: {logs_or_error}")
        return job_info

    # Parse temporal info from logs
    job_resource_name, execution_id, temporal_cluster = parse_job_logs(logs_or_error)

    if execution_id:
        job_info.job_resource_name = job_resource_name
        job_info.temporal_workflow_id = execution_id
        job_info.temporal_cluster = temporal_cluster
        job_info.status = "running"
        print(f"  Temporal workflow: {execution_id}")
    else:
        job_info.error = "Could not parse temporal workflow ID from logs"
        print(f"  WARNING: {job_info.error}")

    return job_info


def load_config(config_path: str) -> dict:
    """Load configuration from JSON file."""
    with open(config_path) as f:
        return json.load(f)


def save_results(results: list[JobInfo], output_path: str):
    """Save results to JSON file."""
    data = {
        "generated_at": datetime.utcnow().isoformat() + "Z",
        "jobs": [asdict(r) for r in results]
    }
    with open(output_path, 'w') as f:
        json.dump(data, f, indent=2)
    print(f"\nResults saved to: {output_path}")


def main():
    parser = argparse.ArgumentParser(
        description="Backfill scorecards for all customers across clusters"
    )
    parser.add_argument(
        "--config", "-c",
        required=True,
        help="Path to config JSON file with clusters and customers"
    )
    parser.add_argument(
        "--output", "-o",
        default="backfill_tracking.json",
        help="Output file for tracking info (default: backfill_tracking.json)"
    )
    parser.add_argument(
        "--start-time",
        default=DEFAULT_START_TIME,
        help=f"Reindex start time (default: {DEFAULT_START_TIME})"
    )
    parser.add_argument(
        "--end-time",
        default=DEFAULT_END_TIME,
        help=f"Reindex end time (default: {DEFAULT_END_TIME})"
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show what would be done without creating jobs"
    )
    parser.add_argument(
        "--skip-logs",
        action="store_true",
        help="Skip waiting for logs (faster, but no temporal workflow IDs)"
    )
    parser.add_argument(
        "--cluster",
        help="Only process a specific cluster"
    )
    parser.add_argument(
        "--customer",
        help="Only process a specific customer"
    )

    args = parser.parse_args()

    # Load config
    config = load_config(args.config)

    print("=" * 60)
    print("Backfill Scorecards - Batch Job Creator")
    print("=" * 60)
    print(f"Start time: {args.start_time}")
    print(f"End time:   {args.end_time}")
    print(f"Dry run:    {args.dry_run}")
    print(f"Skip logs:  {args.skip_logs}")

    results: list[JobInfo] = []

    for cluster_config in config.get("clusters", []):
        cluster = cluster_config["name"]

        # Filter by cluster if specified
        if args.cluster and cluster != args.cluster:
            continue

        print(f"\n{'=' * 60}")
        print(f"Cluster: {cluster}")
        print("=" * 60)

        for customer_config in cluster_config.get("customers", []):
            customer = customer_config["id"]
            profile = customer_config.get("profile", "default")

            # Filter by customer if specified
            if args.customer and customer != args.customer:
                continue

            job_info = process_customer(
                cluster=cluster,
                customer=customer,
                profile=profile,
                start_time=args.start_time,
                end_time=args.end_time,
                dry_run=args.dry_run,
                skip_logs=args.skip_logs
            )
            results.append(job_info)

    # Save results
    if results:
        save_results(results, args.output)
    else:
        print("\nNo jobs processed.")

    # Summary
    print("\n" + "=" * 60)
    print("Summary")
    print("=" * 60)
    status_counts = {}
    for r in results:
        status_counts[r.status] = status_counts.get(r.status, 0) + 1
    for status, count in sorted(status_counts.items()):
        print(f"  {status}: {count}")

    # Print any errors
    errors = [r for r in results if r.error]
    if errors:
        print("\nErrors:")
        for r in errors:
            print(f"  {r.customer}/{r.profile} on {r.cluster}: {r.error}")


if __name__ == "__main__":
    main()
