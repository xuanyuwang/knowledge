#!/usr/bin/env python3
"""
Appeal scorecard cleanup orchestration - per cluster.

Discovers all customer databases on a ClickHouse cluster, deletes appeal scorecard
data from ClickHouse, waits for mutations, then runs backfill for each customer.
Processes customers in batches of 10 with JSON-based progress tracking.

Usage:
    python3 cluster_cleanup.py <cluster> <ch_host> <ch_password>
    python3 cluster_cleanup.py <cluster> --status
    python3 cluster_cleanup.py <cluster> --reset <customer>

Prerequisites:
    - VPN connected
    - kubectl access to <cluster>_dev
    - temporal CLI installed
    - clickhouse client at /opt/homebrew/bin/clickhouse

Created: 2026-02-20
"""

import argparse
import json
import os
import signal
import subprocess
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timedelta, timezone
from pathlib import Path

# ---- Config ----

SCRIPT_DIR = Path(__file__).parent
TRACKING_DIR = SCRIPT_DIR / "tracking"
BACKFILL_SCRIPT = SCRIPT_DIR / "backfill.sh"

CH_CLIENT = "/opt/homebrew/bin/clickhouse"
CH_PORT = 9440
CH_USER = "admin"

DATE_START = "2026-01-01"
DATE_END = "2026-02-21"

BATCH_SIZE = 10
MUTATION_POLL_INTERVAL = 10  # seconds
MUTATION_TIMEOUT = 600  # 10 minutes
WORKFLOW_DISCOVERY_WAIT = 15  # seconds after job creation
WORKFLOW_POLL_INTERVAL = 30  # seconds
WORKFLOW_TIMEOUT = 3600  # 1 hour per customer

# Large customers that need 1-day sequential splits
LARGE_CUSTOMERS = {"cvs", "oportun"}

# Customers to skip (already completed)
SKIP_CUSTOMERS = {"mutualofomaha"}

TEMPORAL_NS = "ingestion"
TEMPORAL_ADDR = "localhost:7233"

# System databases to exclude
SYSTEM_DBS = {
    "system", "default", "INFORMATION_SCHEMA", "information_schema",
    "cresta_system", "_temporary_and_external_tables",
}


# ---- Helpers ----

def now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def run(cmd: list[str], timeout: int = 120) -> tuple[int, str, str]:
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired:
        return -1, "", f"Command timed out after {timeout}s"
    except Exception as e:
        return -1, "", str(e)


def ch_query(host: str, password: str, query: str, database: str = "") -> str:
    """Execute a ClickHouse query and return stdout."""
    cmd = [
        CH_CLIENT, "client",
        "-h", host, "--port", str(CH_PORT),
        "-u", CH_USER, "--password", password,
        "--secure",
        "--query", query,
    ]
    if database:
        cmd.extend(["-d", database])
    rc, stdout, stderr = run(cmd, timeout=300)
    if rc != 0:
        raise RuntimeError(f"ClickHouse query failed: {stderr.strip()}")
    return stdout.strip()


# ---- Tracking ----

def tracking_path(cluster: str) -> Path:
    return TRACKING_DIR / f"{cluster}.json"


def load_tracking(cluster: str) -> dict:
    path = tracking_path(cluster)
    if path.exists():
        with open(path) as f:
            return json.load(f)
    return None


def save_tracking(tracking: dict):
    cluster = tracking["cluster"]
    path = tracking_path(cluster)
    TRACKING_DIR.mkdir(parents=True, exist_ok=True)
    with open(path, "w") as f:
        json.dump(tracking, f, indent=2)


def init_tracking(cluster: str, ch_host: str, customers: dict[str, dict]) -> dict:
    """Initialize tracking for a cluster with discovered customers."""
    tracking = {
        "cluster": cluster,
        "ch_host": ch_host,
        "created_at": now_iso(),
        "date_range": [DATE_START, DATE_END],
        "customers": {},
    }
    for customer_id, info in customers.items():
        tracking["customers"][customer_id] = {
            "status": "pending",
            "databases": info["databases"],
            "before_counts": info["counts"],
            "after_counts": None,
            "started_at": None,
            "completed_at": None,
            "error": None,
        }
    save_tracking(tracking)
    return tracking


# ---- Discovery ----

def discover_databases(ch_host: str, ch_password: str) -> dict[str, dict]:
    """
    Discover all databases with scorecard data, grouped by customer ID.

    Returns: {customer_id: {"databases": [...], "counts": {"scorecard": N, "score": N}}}
    """
    print("Discovering databases with scorecard data...")

    # Get all databases with a scorecard table
    db_exclusion = ", ".join(f"'{db}'" for db in SYSTEM_DBS)
    result = ch_query(ch_host, ch_password,
        "SELECT DISTINCT database FROM system.tables "
        "WHERE name = 'scorecard' AND database NOT IN "
        f"({db_exclusion}) ORDER BY database"
    )
    if not result:
        return {}

    databases = [db.strip() for db in result.split("\n") if db.strip()]
    print(f"  Found {len(databases)} databases with scorecard tables")

    # Count rows in each database
    customers: dict[str, dict] = {}
    for db in databases:
        try:
            sc_count = int(ch_query(ch_host, ch_password,
                f"SELECT count() FROM {db}.scorecard "
                f"WHERE scorecard_time >= '{DATE_START}' AND scorecard_time < '{DATE_END}'"
            ) or "0")
        except (RuntimeError, ValueError):
            sc_count = 0

        try:
            score_count = int(ch_query(ch_host, ch_password,
                f"SELECT count() FROM {db}.score "
                f"WHERE scorecard_time >= '{DATE_START}' AND scorecard_time < '{DATE_END}'"
            ) or "0")
        except (RuntimeError, ValueError):
            score_count = 0

        if sc_count == 0 and score_count == 0:
            continue

        # Extract customer ID from database name: <customer>_<usecase>_... -> <customer>
        # Convention: database = customerid_usecase or customerid_usecase_environment
        # The customer ID is everything before the first recognized suffix
        customer_id = extract_customer_id(db)

        if customer_id not in customers:
            customers[customer_id] = {
                "databases": [],
                "counts": {"scorecard": 0, "score": 0},
            }
        customers[customer_id]["databases"].append(db)
        customers[customer_id]["counts"]["scorecard"] += sc_count
        customers[customer_id]["counts"]["score"] += score_count

        print(f"  {db}: scorecard={sc_count}, score={score_count} (customer={customer_id})")

    print(f"\n  {len(customers)} customers with data")
    return customers


def extract_customer_id(database_name: str) -> str:
    """
    Extract customer ID from ClickHouse database name.

    Database naming convention: <customerid>_<usecase>[_<env>]
    Examples:
        mutualofomaha_voice -> mutualofomaha
        mutualofomaha_medsupp_voice -> mutualofomaha
        mutualofomaha_sandbox_voice_sbx -> mutualofomaha-sandbox
        hilton_chat -> hilton
        cvs_voice -> cvs

    The customer ID for backfill.sh's RUN_ONLY_FOR_CUSTOMER_IDS is typically
    the part before _voice, _chat, _messaging, etc.

    For sandbox databases (*_sandbox_* or *_sbx), the customer ID includes "-sandbox".
    """
    parts = database_name.split("_")

    # Known usecase/environment suffixes to strip
    suffixes = {"voice", "chat", "messaging", "email", "sbx"}

    # Check if this is a sandbox database
    is_sandbox = "sandbox" in parts or "sbx" in parts

    # Find the customer ID by removing known suffixes from the end
    # Work backwards, stripping known suffixes
    customer_parts = []
    for part in parts:
        if part in suffixes:
            continue
        if part == "sandbox":
            continue
        customer_parts.append(part)

    customer_id = customer_parts[0] if customer_parts else parts[0]

    if is_sandbox:
        customer_id = f"{customer_id}-sandbox"

    return customer_id


# ---- ClickHouse Delete ----

def delete_ch_data(ch_host: str, ch_password: str, databases: list[str]) -> None:
    """Delete scorecard and score data from all databases for a customer."""
    for db in databases:
        for table in ["scorecard", "score"]:
            print(f"    Deleting from {db}.{table}...")
            try:
                ch_query(ch_host, ch_password,
                    f"ALTER TABLE {db}.{table} ON CLUSTER 'conversations' "
                    f"DELETE WHERE scorecard_time >= '{DATE_START}' "
                    f"AND scorecard_time < '{DATE_END}' "
                    f"SETTINGS replication_wait_for_inactive_replica_timeout = 0"
                )
            except RuntimeError as e:
                print(f"    WARNING: Delete failed for {db}.{table}: {e}")
                raise


def wait_for_mutations(ch_host: str, ch_password: str, databases: list[str]) -> None:
    """Wait for all pending mutations to complete in the given databases."""
    db_list = ", ".join(f"'{db}'" for db in databases)

    elapsed = 0
    while elapsed < MUTATION_TIMEOUT:
        result = ch_query(ch_host, ch_password,
            f"SELECT database, table, mutation_id "
            f"FROM system.mutations "
            f"WHERE database IN ({db_list}) AND is_done = 0"
        )
        if not result:
            print("    All mutations completed.")
            return

        pending = len(result.strip().split("\n"))
        print(f"    [{elapsed}s] {pending} mutation(s) still running...")
        time.sleep(MUTATION_POLL_INTERVAL)
        elapsed += MUTATION_POLL_INTERVAL

    raise RuntimeError(f"Mutations did not complete within {MUTATION_TIMEOUT}s")


def count_ch_data(ch_host: str, ch_password: str, databases: list[str]) -> dict[str, int]:
    """Count scorecard and score rows across all databases."""
    total_sc = 0
    total_score = 0
    for db in databases:
        try:
            sc = int(ch_query(ch_host, ch_password,
                f"SELECT count() FROM {db}.scorecard "
                f"WHERE scorecard_time >= '{DATE_START}' AND scorecard_time < '{DATE_END}'"
            ) or "0")
        except (RuntimeError, ValueError):
            sc = 0
        try:
            score = int(ch_query(ch_host, ch_password,
                f"SELECT count() FROM {db}.score "
                f"WHERE scorecard_time >= '{DATE_START}' AND scorecard_time < '{DATE_END}'"
            ) or "0")
        except (RuntimeError, ValueError):
            score = 0
        total_sc += sc
        total_score += score
    return {"scorecard": total_sc, "score": total_score}


# ---- Port-forward management ----

class PortForward:
    def __init__(self, cluster: str):
        self.cluster = cluster
        self.context = f"{cluster}_dev"
        self.proc = None

    def start(self):
        self.stop()
        time.sleep(2)
        self.proc = subprocess.Popen(
            [
                "kubectl", f"--context={self.context}", "-n", "temporal",
                "port-forward", "svc/temporal-frontend-headless", "7233:7233",
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        time.sleep(5)
        print(f"  Port-forward started (pid {self.proc.pid})")

    def stop(self):
        subprocess.run(["pkill", "-f", "port-forward.*7233"], capture_output=True)
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


def find_recent_workflows(customer_id: str, cluster: str, max_age_minutes: int = 3) -> list[str]:
    """Find running workflows for a customer started within max_age_minutes."""
    # Workflow IDs look like: reindexconversations-<customer>-<cluster>-...
    prefix = f"reindexconversations-{customer_id}-{cluster}"
    rc, stdout, _ = run(
        [
            "temporal", "workflow", "list",
            "--namespace", TEMPORAL_NS,
            "--address", TEMPORAL_ADDR,
            "--query",
            f'ExecutionStatus = "Running" AND WorkflowId STARTS_WITH "{prefix}"',
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


def wait_for_workflows(workflow_ids: list[str], pf: PortForward) -> tuple[bool, str]:
    """Poll workflows until all are done or timeout."""
    elapsed = 0
    while elapsed < WORKFLOW_TIMEOUT:
        pf.ensure_alive()

        all_done = True
        any_failed = False
        status_parts = []

        for wf_id in workflow_ids:
            status = get_workflow_status(wf_id)
            short = status.split("_")[-1] if "_" in status else status
            wf_short = wf_id.rsplit("-", 1)[-1][:8] if "-" in wf_id else wf_id[:8]
            status_parts.append(f"{wf_short}={short}")

            if "RUNNING" in status:
                all_done = False
            elif "FAILED" in status or "TIMED_OUT" in status or "CANCELED" in status:
                any_failed = True

        if all_done:
            if any_failed:
                msg = f"Workflows finished with failures: {', '.join(status_parts)}"
                return False, msg
            return True, ""

        print(f"    [{elapsed}s] {' | '.join(status_parts)}")
        time.sleep(WORKFLOW_POLL_INTERVAL)
        elapsed += WORKFLOW_POLL_INTERVAL

    return False, f"Timeout after {WORKFLOW_TIMEOUT}s"


# ---- Backfill ----

def run_backfill(cluster: str, customer_id: str, start_date: str, end_date: str) -> str:
    """Run backfill.sh and return the job name."""
    rc, stdout, stderr = run(
        [str(BACKFILL_SCRIPT), cluster, customer_id, start_date, end_date],
        timeout=60,
    )
    if rc != 0:
        raise RuntimeError(f"backfill.sh failed: {stderr}")

    # Extract job name from output
    for line in stdout.split("\n"):
        if "Job created:" in line:
            return line.split("Job created:")[-1].strip()
    # Fallback: return last non-empty line
    return stdout.strip().split("\n")[-1]


def run_backfill_single_customer(
    cluster: str, customer_id: str, pf: PortForward
) -> tuple[bool, str]:
    """Run backfill for a single customer and wait for completion."""
    is_large = customer_id in LARGE_CUSTOMERS

    if is_large:
        return run_backfill_sequential(cluster, customer_id, pf)
    else:
        return run_backfill_single(cluster, customer_id, pf)


def run_backfill_single(
    cluster: str, customer_id: str, pf: PortForward
) -> tuple[bool, str]:
    """Run a single backfill job for the full date range."""
    print(f"    Running backfill: {customer_id} ({DATE_START} to {DATE_END})...")
    try:
        job_name = run_backfill(cluster, customer_id, DATE_START, DATE_END)
        print(f"    Job: {job_name}")
    except RuntimeError as e:
        return False, str(e)

    # Wait for workflows to spawn
    time.sleep(WORKFLOW_DISCOVERY_WAIT)
    pf.ensure_alive()

    workflow_ids = find_recent_workflows(customer_id, cluster)
    if not workflow_ids:
        print("    No running workflows found (may have completed instantly).")
        return True, ""

    print(f"    Found {len(workflow_ids)} workflow(s)")
    return wait_for_workflows(workflow_ids, pf)


def run_backfill_sequential(
    cluster: str, customer_id: str, pf: PortForward
) -> tuple[bool, str]:
    """Run 1-day sequential backfill for large customers (cvs, oportun)."""
    start = datetime.strptime(DATE_START, "%Y-%m-%d")
    end = datetime.strptime(DATE_END, "%Y-%m-%d")

    total_days = (end - start).days
    print(f"    Running sequential backfill: {customer_id} ({total_days} days)...")

    current = start
    day_num = 0
    while current < end:
        day_num += 1
        next_day = current + timedelta(days=1)
        day_start = current.strftime("%Y-%m-%d")
        day_end = next_day.strftime("%Y-%m-%d")

        print(f"    [{day_num}/{total_days}] {day_start}...")
        try:
            job_name = run_backfill(cluster, customer_id, day_start, day_end)
        except RuntimeError as e:
            return False, f"Day {day_start}: {e}"

        # Wait for workflows
        time.sleep(WORKFLOW_DISCOVERY_WAIT)
        pf.ensure_alive()

        workflow_ids = find_recent_workflows(customer_id, cluster)
        if workflow_ids:
            success, error = wait_for_workflows(workflow_ids, pf)
            if not success:
                return False, f"Day {day_start}: {error}"

        current = next_day

    return True, ""


# ---- Process one customer ----

def process_customer(
    tracking: dict,
    customer_id: str,
    ch_host: str,
    ch_password: str,
    cluster: str,
    pf: PortForward,
) -> None:
    """Process a single customer: delete -> wait mutations -> backfill."""
    cust = tracking["customers"][customer_id]
    databases = cust["databases"]

    print(f"\n  === {customer_id} ({len(databases)} database(s)) ===")

    # Mark as deleting
    cust["status"] = "deleting"
    cust["started_at"] = now_iso()
    cust["error"] = None
    save_tracking(tracking)

    try:
        # Step 1: Delete from ClickHouse
        print(f"  Deleting ClickHouse data...")
        delete_ch_data(ch_host, ch_password, databases)

        # Step 2: Wait for mutations
        print(f"  Waiting for mutations...")
        wait_for_mutations(ch_host, ch_password, databases)

        # Step 3: Backfill
        cust["status"] = "backfilling"
        save_tracking(tracking)

        print(f"  Running backfill...")
        success, error = run_backfill_single_customer(cluster, customer_id, pf)

        if not success:
            raise RuntimeError(f"Backfill failed: {error}")

        # Step 4: Count after
        print(f"  Counting post-backfill data...")
        cust["after_counts"] = count_ch_data(ch_host, ch_password, databases)

        # Done
        cust["status"] = "completed"
        cust["completed_at"] = now_iso()
        save_tracking(tracking)

        before = cust["before_counts"]
        after = cust["after_counts"]
        sc_delta = before["scorecard"] - after["scorecard"]
        score_delta = before["score"] - after["score"]
        print(f"  DONE: scorecard {before['scorecard']} -> {after['scorecard']} (-{sc_delta}), "
              f"score {before['score']} -> {after['score']} (-{score_delta})")

    except Exception as e:
        cust["status"] = "failed"
        cust["error"] = str(e)
        cust["completed_at"] = now_iso()
        save_tracking(tracking)
        print(f"  FAILED: {e}")


# ---- Commands ----

def cmd_status(cluster: str):
    tracking = load_tracking(cluster)
    if not tracking:
        print(f"No tracking file for cluster: {cluster}")
        print(f"  Expected: {tracking_path(cluster)}")
        return

    customers = tracking["customers"]
    from collections import Counter
    counts = Counter(c["status"] for c in customers.values())

    print("============================================================")
    print(f"Appeal Scorecard Cleanup - {cluster}")
    print(f"============================================================")
    print(f"ClickHouse: {tracking['ch_host']}")
    print(f"Date range: {tracking['date_range'][0]} to {tracking['date_range'][1]}")
    print(f"Created:    {tracking['created_at']}")
    print(f"Customers:  {len(customers)}")
    print("------------------------------------------------------------")

    for status in ["completed", "backfilling", "deleting", "failed", "pending"]:
        count = counts.get(status, 0)
        if count > 0:
            names = sorted(k for k, v in customers.items() if v["status"] == status)
            # Show first few names
            display = ", ".join(names[:5])
            if len(names) > 5:
                display += f", ... (+{len(names) - 5} more)"
            print(f"  {status.upper():14s}: {count:3d}  [{display}]")

    print(f"  {'TOTAL':14s}: {sum(counts.values()):3d}")
    print("------------------------------------------------------------")

    # Show totals for completed
    total_before_sc = sum(
        c["before_counts"]["scorecard"] for c in customers.values()
        if c["status"] == "completed" and c["after_counts"]
    )
    total_after_sc = sum(
        c["after_counts"]["scorecard"] for c in customers.values()
        if c["status"] == "completed" and c["after_counts"]
    )
    total_before_score = sum(
        c["before_counts"]["score"] for c in customers.values()
        if c["status"] == "completed" and c["after_counts"]
    )
    total_after_score = sum(
        c["after_counts"]["score"] for c in customers.values()
        if c["status"] == "completed" and c["after_counts"]
    )

    if total_before_sc > 0:
        print(f"\n  Completed totals:")
        print(f"    scorecard: {total_before_sc} -> {total_after_sc} "
              f"(-{total_before_sc - total_after_sc})")
        print(f"    score:     {total_before_score} -> {total_after_score} "
              f"(-{total_before_score - total_after_score})")

    # Show failures
    failures = {k: v for k, v in customers.items() if v["status"] == "failed"}
    if failures:
        print(f"\n  Failures:")
        for name, info in sorted(failures.items()):
            print(f"    {name}: {info.get('error', 'unknown')}")

    print("============================================================")


def cmd_reset(cluster: str, customer_id: str):
    tracking = load_tracking(cluster)
    if not tracking:
        print(f"No tracking file for cluster: {cluster}")
        return

    if customer_id not in tracking["customers"]:
        print(f"Customer not found: {customer_id}")
        print(f"Available: {', '.join(sorted(tracking['customers'].keys()))}")
        return

    cust = tracking["customers"][customer_id]
    old_status = cust["status"]
    cust["status"] = "pending"
    cust["error"] = None
    cust["started_at"] = None
    cust["completed_at"] = None
    cust["after_counts"] = None
    save_tracking(tracking)
    print(f"Reset {customer_id}: {old_status} -> pending")


def cmd_run(cluster: str, ch_host: str, ch_password: str):
    # Check if we have an existing tracking file
    tracking = load_tracking(cluster)

    if tracking is None:
        # Discover databases and initialize tracking
        customers = discover_databases(ch_host, ch_password)
        if not customers:
            print("No customers with scorecard data found.")
            return

        # Mark skip customers
        for skip_id in SKIP_CUSTOMERS:
            if skip_id in customers:
                print(f"  Marking {skip_id} as completed (already done)")
                customers[skip_id]["status_override"] = "completed"

        tracking = init_tracking(cluster, ch_host, customers)

        # Apply skip overrides
        for skip_id in SKIP_CUSTOMERS:
            if skip_id in tracking["customers"]:
                tracking["customers"][skip_id]["status"] = "completed"
                tracking["customers"][skip_id]["completed_at"] = "previously completed"
        save_tracking(tracking)

        print(f"\nTracking initialized: {tracking_path(cluster)}")
    else:
        print(f"Resuming from existing tracking: {tracking_path(cluster)}")

    # Show current status
    cmd_status(cluster)
    print()

    # Get pending customers
    pending = sorted(
        k for k, v in tracking["customers"].items()
        if v["status"] in ("pending", "failed")
    )

    if not pending:
        print("All customers are completed or in progress. Nothing to do.")
        return

    print(f"{len(pending)} customers to process")
    print()

    # Set up port-forward
    pf = PortForward(cluster)

    def cleanup(sig=None, frame=None):
        pf.stop()
        if sig:
            print(f"\nInterrupted. Progress saved to {tracking_path(cluster)}")
            sys.exit(1)

    signal.signal(signal.SIGINT, cleanup)
    signal.signal(signal.SIGTERM, cleanup)

    pf.start()

    try:
        # Process in batches
        for batch_start in range(0, len(pending), BATCH_SIZE):
            batch = pending[batch_start:batch_start + BATCH_SIZE]
            batch_num = batch_start // BATCH_SIZE + 1
            total_batches = (len(pending) + BATCH_SIZE - 1) // BATCH_SIZE

            print("============================================================")
            print(f"Batch {batch_num}/{total_batches}: {', '.join(batch)}")
            print("============================================================")

            for customer_id in batch:
                process_customer(
                    tracking, customer_id,
                    ch_host, ch_password, cluster, pf,
                )

            # Refresh pending list in case we want to re-check
            remaining = sum(
                1 for v in tracking["customers"].values()
                if v["status"] in ("pending", "failed")
            )
            completed = sum(
                1 for v in tracking["customers"].values()
                if v["status"] == "completed"
            )
            print(f"\n  Batch {batch_num} done. "
                  f"Completed: {completed}, Remaining: {remaining}")

    finally:
        cleanup()

    print()
    print("============================================================")
    print("Cluster cleanup finished!")
    print("============================================================")
    cmd_status(cluster)


# ---- Main ----

def main():
    parser = argparse.ArgumentParser(
        description="Appeal scorecard cleanup - per cluster orchestration"
    )
    parser.add_argument("cluster", help="Cluster name (e.g., voice-prod)")
    parser.add_argument("ch_host", nargs="?",
                        help="ClickHouse host")
    parser.add_argument("ch_password", nargs="?",
                        help="ClickHouse admin password")
    parser.add_argument("--status", action="store_true",
                        help="Show progress for this cluster")
    parser.add_argument("--reset", metavar="CUSTOMER",
                        help="Reset a customer to pending")

    args = parser.parse_args()

    if args.status:
        cmd_status(args.cluster)
    elif args.reset:
        cmd_reset(args.cluster, args.reset)
    else:
        if not args.ch_host or not args.ch_password:
            parser.error("ch_host and ch_password are required for running cleanup")
        cmd_run(args.cluster, args.ch_host, args.ch_password)


if __name__ == "__main__":
    main()
