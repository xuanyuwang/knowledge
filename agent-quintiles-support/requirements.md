# Backend:
- for a given scorecard template, divide agents into quntiles based on each agent's QA score for the template
- only categorize agents when groups by agents, i.e. do NOT support team quintiles

# Frontend:

## Performance page

- on Performance page -> Leaderboard by criteria table (the 2nd table on this page) -> Agent tab, insert a column with title "Quintile Rank" right after "Average Performance" column. The new column should also be sticky as "Average Performance" column. Each cell of this column display a single number, one of 1 to 5, to represent 1st, 2nd, ..., 5th quintile.
- on the same table, there should be an icon following agent name text. The icon has 3 different colors: gold -> 1st quintile, silver -> 2nd quintile, and bronz -> 3rd quintile. In case of long names, also display the icon on tooltips of names.
- on Performance page -> Leaderboard per criteria table (the 3rd table) -> Agent tab, insert the same column as the last sticky column, just like what's on the Leaderboard by criteria table. There should be same icons for each agent with the same design.

## Leaderboard page

- on Leaderboard page -> Agent leaderboard table, insert quintile column after "Live Assist".
- display same icons on both Agent Leaderboard table and Agent Leaderboard per metric table in the "Name" column with same icon design

## Coaching hub page

- on Coaching Hub page -> "Recent Coaching Activities", display icon with same design on "Agent Name" column.
- when hover over the icons, display tooltips with "Xth quintile based on last 7 days"

## Coaching Plan page

To be determined

## Feature flag

- Add a frontend feature flag to guard all quintile UI (column + icons). When the flag is off, no quintile column or icon should appear.
- Flag is defined in the `config` repo (`config/src/CustomerConfig.ts` under `featureFlags`), then consumed in director via `useFeatureFlag('flagName')`.
