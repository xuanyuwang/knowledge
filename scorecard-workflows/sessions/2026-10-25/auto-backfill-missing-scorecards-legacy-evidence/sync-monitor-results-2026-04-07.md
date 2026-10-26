# Scorecard Sync Monitor Results — All Clusters

**Date**: 2026-04-07
**Tool**: `cron-scorecard-sync-monitor` (image: `main-20260407_212824z-69bc2f17`)
**Config**: `skipHistoric=true` (default), `CRON_ERROR_CHANNEL=""`, all customers
**Time Range**: March 2026 (2026-03-01 to 2026-04-01)

---

## us-east-1-prod

**90 tasks | 57,800 submitted | 1,601 missing (2.77%) | 22 with gaps | 55 zero-volume omitted**

| Customer/Profile | Total | Missing | Rate | Status |
|---|---|---|---|---|
| nclh/us-east-1 | 144 | 10 | 6.94% | WARNING |
| pure/us-east-1 | 59 | 4 | 6.78% | WARNING |
| aqua/us-east-1 | 592 | 36 | 6.08% | WARNING |
| abcbs/us-east-1 | 368 | 21 | 5.71% | WARNING |
| sunbit/us-east-1 | 4,363 | 236 | 5.41% | WARNING |
| acorns/us-east-1 | 989 | 51 | 5.16% | WARNING |
| rentokil/us-east-1 | 8,555 | 376 | 4.40% | WARNING |
| altice/us-east-1 | 137 | 6 | 4.38% | WARNING |
| alaska-air/us-east-1 | 1,587 | 69 | 4.35% | WARNING |
| comerica-east/us-east-1 | 408 | 17 | 4.17% | WARNING |
| iqcu/us-east-1 | 49 | 2 | 4.08% | WARNING |
| greenix/us-east-1 | 2,011 | 80 | 3.98% | WARNING |
| collegeboard/us-east-1 | 1,962 | 75 | 3.82% | WARNING |
| amba/us-east-1 | 126 | 4 | 3.17% | WARNING |
| united-east/us-east-1 | 11,009 | 330 | 3.00% | WARNING |
| home-care-delivered/us-east-1 | 1,515 | 41 | 2.71% | WARNING |
| guitar-center/us-east-1 | 1,166 | 22 | 1.89% | WARNING |
| fubotv/us-east-1 | 1,663 | 25 | 1.50% | WARNING |
| nrg/us-east-1 | 5,017 | 60 | 1.20% | WARNING |
| spirit/us-east-1 | 8,693 | 94 | 1.08% | WARNING |
| propel/us-east-1 | 1,452 | 14 | 0.96% | OK |
| marriott/us-east-1 | 4,303 | 28 | 0.65% | OK |
| bswift/us-east-1 | 403 | 0 | 0.00% | OK |
| care-access/us-east-1 | 304 | 0 | 0.00% | OK |
| cng/us-east-1 | 4 | 0 | 0.00% | OK |
| lending-club/us-east-1 | 53 | 0 | 0.00% | OK |
| opera-bootcamp/us-east-1 | 1 | 0 | 0.00% | OK |
| pack-rat/us-east-1 | 295 | 0 | 0.00% | OK |
| ralphlauren/us-east-1 | 329 | 0 | 0.00% | OK |
| rcg-sandbox/us-east-1 | 2 | 0 | 0.00% | OK |
| rcg/us-east-1 | 173 | 0 | 0.00% | OK |
| tailorcare/us-east-1 | 44 | 0 | 0.00% | OK |
| urgently/us-east-1 | 14 | 0 | 0.00% | OK |
| westernhealth/us-east-1 | 9 | 0 | 0.00% | OK |
| wheels/us-east-1 | 1 | 0 | 0.00% | OK |

**Findings**: Widespread sync gaps across 20 customers (1–7% missing). Largest absolute gaps: rentokil (376), united-east (330), sunbit (236). Highest rates: nclh (6.94%), pure (6.78%), aqua (6.08%).

---

## us-west-2-prod

**105 tasks | 33,968 submitted | 317 missing (0.93%) | 9 with gaps | 95 zero-volume omitted**

| Customer/Profile | Total | Missing | Rate | Status |
|---|---|---|---|---|
| aaa-life/us-west-2 | 766 | 14 | 1.83% | WARNING |
| xanterra/us-west-2 | 2,013 | 27 | 1.34% | WARNING |
| cox2/us-west-2 | 3,333 | 44 | 1.32% | WARNING |
| oportun/us-west-2 | 14,320 | 169 | 1.18% | WARNING |
| snapfinance/us-west-2 | 6,022 | 35 | 0.58% | OK |
| vivint/cx-voice | 3,108 | 13 | 0.42% | OK |
| frontdoor/us-west-2 | 765 | 3 | 0.39% | OK |
| care-oregon/us-west-2 | 2,036 | 7 | 0.34% | OK |
| achieve/us-west-2 | 1,603 | 5 | 0.31% | OK |
| bill-west/us-west-2 | 2 | 0 | 0.00% | OK |

**Findings**: 4 customers above 1% threshold. oportun largest by volume (169 missing). 95 of 105 profiles have 0 scorecards.

---

## voice-prod

**48 tasks | 17,147 submitted | 450 missing (2.62%) | 4 with gaps | 31 zero-volume omitted**

| Customer/Profile | Total | Missing | Rate | Status |
|---|---|---|---|---|
| **brinks/care-voice** | **1,676** | **436** | **26.01%** | **CRITICAL** |
| cox/voice | 1,991 | 7 | 0.35% | OK |
| vivint/voice | 4,936 | 6 | 0.12% | OK |
| holidayinn/transfers-voice | 1,176 | 1 | 0.09% | OK |
| aptive/voice | 157 | 0 | 0.00% | OK |
| cartier/voice | 386 | 0 | 0.00% | OK |
| cox/account-svc-voice | 322 | 0 | 0.00% | OK |
| cox/retention-voice | 1,770 | 0 | 0.00% | OK |
| cresta-sandbox-2/voice-sandbox-2 | 31 | 0 | 0.00% | OK |
| cresta/voice-sandbox | 3 | 0 | 0.00% | OK |
| hilton/voice | 1,161 | 0 | 0.00% | OK |
| holidayinn/owners-voice | 28 | 0 | 0.00% | OK |
| holidayinn/voice | 364 | 0 | 0.00% | OK |
| integrity-marketing/unifiedhealth-voice | 250 | 0 | 0.00% | OK |
| mutualofomaha/voice | 1,620 | 0 | 0.00% | OK |
| snapfinance-cs/bot | 644 | 0 | 0.00% | OK |
| usbank/voice | 632 | 0 | 0.00% | OK |

**Findings**: brinks/care-voice is CRITICAL at 26.01% (436/1,676) — 96.9% of all missing scorecards on this cluster.

---

## chat-prod

**26 tasks | 3,785 submitted | 0 missing (0.00%) | 0 with gaps | 24 zero-volume omitted**

| Customer/Profile | Total | Missing | Rate | Status |
|---|---|---|---|---|
| intuit/qbo | 3,780 | 0 | 0.00% | OK |
| sleepnumber/main | 5 | 0 | 0.00% | OK |

**Findings**: Clean. Only 2 profiles with scorecards, both fully synced.

---

## eu-west-2-prod

**6 tasks | 1 submitted | 0 missing (0.00%) | 0 with gaps | 5 zero-volume omitted**

| Customer/Profile | Total | Missing | Rate | Status |
|---|---|---|---|---|
| domestic-gen/eu-west-2 | 1 | 0 | 0.00% | OK |

**Findings**: Minimal scorecard usage in EU region.

---

## ap-southeast-2-prod

**4 tasks | 0 submitted | 0 missing | 0 with gaps | 4 zero-volume omitted**

No customers with scorecards in March.

---

## ca-central-1-prod

**2 tasks | 0 submitted | 0 missing | 0 with gaps | 2 zero-volume omitted**

No customers with scorecards in March.

---

## voice-prod — April 2026 (2026-04-06 to 2026-04-07, default 24h)

**48 tasks | 519 submitted | 1 missing (0.19%) | 1 with gaps | 33 zero-volume omitted**

| Customer/Profile | Total | Missing | Rate | Status |
|---|---|---|---|---|
| vivint/voice | 49 | 1 | 2.04% | WARNING |
| aptive/voice | 9 | 0 | 0.00% | OK |
| brinks/care-voice | 68 | 0 | 0.00% | OK |
| cartier/voice | 5 | 0 | 0.00% | OK |
| cox/account-svc-voice | 26 | 0 | 0.00% | OK |
| cox/retention-voice | 51 | 0 | 0.00% | OK |
| cox/voice | 126 | 0 | 0.00% | OK |
| cresta-sandbox-2/voice-sandbox-2 | 1 | 0 | 0.00% | OK |
| hilton/voice | 54 | 0 | 0.00% | OK |
| holidayinn/transfers-voice | 40 | 0 | 0.00% | OK |
| holidayinn/voice | 2 | 0 | 0.00% | OK |
| integrity-marketing/unifiedhealth-voice | 8 | 0 | 0.00% | OK |
| mutualofomaha/voice | 73 | 0 | 0.00% | OK |
| snapfinance-cs/bot | 4 | 0 | 0.00% | OK |
| usbank/voice | 3 | 0 | 0.00% | OK |

**Findings**: brinks/care-voice improved from 26.01% (March) to 0.00% (April). Only vivint/voice has 1 missing (2.04%, likely sync delay).

---

## Cross-Cluster Summary (March 2026)

| Cluster | Tasks | Total Submitted | Total Missing | Overall Rate | With Gaps |
|---|---|---|---|---|---|
| **us-east-1-prod** | 90 | 57,800 | 1,601 | **2.77%** | 22 |
| us-west-2-prod | 105 | 33,968 | 317 | 0.93% | 9 |
| **voice-prod** | 48 | 17,147 | 450 | **2.62%** | 4 |
| chat-prod | 26 | 3,785 | 0 | 0.00% | 0 |
| eu-west-2-prod | 6 | 1 | 0 | 0.00% | 0 |
| ap-southeast-2-prod | 4 | 0 | 0 | — | 0 |
| ca-central-1-prod | 2 | 0 | 0 | — | 0 |
| **Total** | **281** | **112,701** | **2,368** | **2.10%** | **35** |

**Key Findings**:
1. **us-east-1-prod is the most affected cluster** — 1,601 missing across 22 customers (2.77%), with rates up to 6.94%
2. **voice-prod**: brinks/care-voice single-handedly accounts for 436/450 missing (96.9%)
3. **us-west-2-prod**: 4 customers above 1%, largest gap is oportun (169 missing)
4. **chat-prod, eu-west-2, ap-southeast-2, ca-central-1**: Clean or minimal usage

## Context

- **ClickHouse sync check only** — historic schema check skipped (`SCORECARD_SYNC_MONITOR_SKIP_HISTORIC=true`)
- Slack notifications sent to `#scorecard-sync-monitor` (channel `C0ABYUBEYSG`)
- CLUSTER SUMMARY log line still subject to zap sampling on high-task clusters — Slack is the source of truth
- PRs: go-servers#26824 (uint64 fix), go-servers#26831 (skip-historic flag), go-servers#26838 (cluster summary), flux-deployments#269729 (CRON_ERROR_CHANNEL override)
