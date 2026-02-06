import json
from datetime import datetime
from collections import defaultdict

# Read monthly stats
monthly_data = {}
with open('/Users/xuanyu.wang/repos/knowledge/productivity-with-ai/monthly_stats.tsv', 'r') as f:
    for line in f:
        parts = line.strip().split('\t')
        month = parts[0]
        monthly_data[month] = {
            'additions': int(parts[1]),
            'deletions': int(parts[2]),
            'total_lines': int(parts[3]),
            'merged_prs': int(parts[4]),
            'total_prs': int(parts[5])
        }

# Exclude June-September 2025 (leave period)
leave_months = ['2025-06', '2025-07', '2025-08', '2025-09']

# Define periods for comparison
periods = {
    'Q1 2024 (Feb-Apr)': ['2024-02', '2024-03', '2024-04'],
    'Q2 2024 (May-Jun)': ['2024-05', '2024-06'],
    'Q3 2024 (Jul-Sep)': ['2024-07', '2024-08', '2024-09'],
    'Q4 2024 (Oct-Dec)': ['2024-10', '2024-11', '2024-12'],
    'Q1 2025 (Jan-Mar)': ['2025-01', '2025-02', '2025-03'],
    'Q2 2025 (Apr-May)': ['2025-04', '2025-05'],
    'Q4 2025 (Oct-Dec, post-leave)': ['2025-10', '2025-11', '2025-12'],
    'Q1 2026 (Jan-Feb)': ['2026-01', '2026-02'],
}

def calc_period_stats(months):
    total_lines = 0
    total_prs = 0
    merged_prs = 0
    valid_months = 0
    for m in months:
        if m in monthly_data and m not in leave_months:
            total_lines += monthly_data[m]['total_lines']
            total_prs += monthly_data[m]['total_prs']
            merged_prs += monthly_data[m]['merged_prs']
            valid_months += 1
    if valid_months == 0:
        return None
    return {
        'total_lines': total_lines,
        'avg_lines_per_month': total_lines / valid_months,
        'total_prs': total_prs,
        'avg_prs_per_month': total_prs / valid_months,
        'merged_prs': merged_prs,
        'valid_months': valid_months
    }

print("=" * 70)
print("PRODUCTIVITY ANALYSIS BY PERIOD")
print("=" * 70)
print()

for period_name, months in periods.items():
    stats = calc_period_stats(months)
    if stats:
        print(f"{period_name}:")
        print(f"  Lines changed: {stats['total_lines']:,} ({stats['avg_lines_per_month']:,.0f}/month)")
        print(f"  PRs: {stats['total_prs']} total, {stats['merged_prs']} merged ({stats['avg_prs_per_month']:.1f}/month)")
        print()

# Calculate pre-leave vs post-leave comparison
pre_leave_months = ['2024-02', '2024-03', '2024-04', '2024-05', '2024-06', '2024-07', '2024-08', 
                    '2024-09', '2024-10', '2024-11', '2024-12', '2025-01', '2025-02', '2025-03', 
                    '2025-04', '2025-05']
post_leave_months = ['2025-10', '2025-11', '2025-12', '2026-01', '2026-02']

pre_stats = calc_period_stats(pre_leave_months)
post_stats = calc_period_stats(post_leave_months)

print("=" * 70)
print("PRE-LEAVE vs POST-LEAVE COMPARISON")
print("(Excluding Jun-Sep 2025)")
print("=" * 70)
print()
print(f"Pre-leave (Feb 2024 - May 2025): {pre_stats['valid_months']} months")
print(f"  Avg lines/month: {pre_stats['avg_lines_per_month']:,.0f}")
print(f"  Avg PRs/month: {pre_stats['avg_prs_per_month']:.1f}")
print()
print(f"Post-leave (Oct 2025 - Feb 2026): {post_stats['valid_months']} months")
print(f"  Avg lines/month: {post_stats['avg_lines_per_month']:,.0f}")
print(f"  Avg PRs/month: {post_stats['avg_prs_per_month']:.1f}")
print()

lines_change = ((post_stats['avg_lines_per_month'] - pre_stats['avg_lines_per_month']) / pre_stats['avg_lines_per_month']) * 100
pr_change = ((post_stats['avg_prs_per_month'] - pre_stats['avg_prs_per_month']) / pre_stats['avg_prs_per_month']) * 100

print(f"Change:")
print(f"  Lines/month: {'+' if lines_change > 0 else ''}{lines_change:.1f}%")
print(f"  PRs/month: {'+' if pr_change > 0 else ''}{pr_change:.1f}%")

# Monthly breakdown
print()
print("=" * 70)
print("MONTHLY BREAKDOWN (Excluding leave period)")
print("=" * 70)
print()
print(f"{'Month':<10} {'Lines':>10} {'PRs':>6} {'Merged':>8} {'Additions':>12} {'Deletions':>12}")
print("-" * 70)

for month in sorted(monthly_data.keys()):
    if month in leave_months:
        continue
    d = monthly_data[month]
    print(f"{month:<10} {d['total_lines']:>10,} {d['total_prs']:>6} {d['merged_prs']:>8} {d['additions']:>12,} {d['deletions']:>12,}")

