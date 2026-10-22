# recur — repeating schedules

S66, for member app decisions 335 and 336 (2026-10-22). Consumers: forms
F87, taskmanager W72, comm CM102, wakala-api WK203. Standard library only.

## The shape

A `Schedule` is a start, an optional end, a time zone, and exactly one of:

- a `Rule` — the subset of an iCalendar RRULE (RFC 5545) the decision names;
- `Dates` — a list of calendar dates (`YYYY-MM-DD`), for school terms and
  anything else no rule describes.

Every occurrence lands at the start's wall-clock time of day in the
schedule's zone, so a 09:00 meeting stays at 09:00 across a daylight-saving
change. A date that a zone skips is normalised by `time.Date`.

`Validate` refuses a schedule with no start, no zone, both or neither of a
rule and dates, an end before the start, an impossible date or an invalid
rule. An invalid schedule yields no occurrences rather than panicking.

## The rule subset

`ParseRule` reads, and `Rule.String` writes, these parts and no others:

| Part | Meaning |
| --- | --- |
| `FREQ` | `DAILY`, `WEEKLY`, `MONTHLY` or `YEARLY` — required |
| `INTERVAL` | every N periods, default 1 |
| `BYDAY` | weekdays; with a position `1`–`4` or `-1` (last) under `MONTHLY`/`YEARLY` |
| `BYMONTHDAY` | `1`–`31`, or `-1`–`-31` counted from the month's end |
| `BYMONTH` | `1`–`12` |
| `WKST` | accepted only as `MO`; weeks start on Monday |

An optional `RRULE:` prefix is accepted. Anything else — `COUNT`, `UNTIL`,
`BYSETPOS`, hourly frequencies, a fifth weekday — is refused with
`ErrInvalidRule`, because the decision does not ask for it and silently
ignoring a part would schedule something other than what was written. The
end lives on the `Schedule`, not as `UNTIL`, so stopping a repeat
(decision 336) is setting `End`, never rewriting the rule.

Expansion follows RFC 5545 for this subset:

- `DAILY` steps N days; any `BY*` part filters.
- `WEEKLY` steps N Monday-started weeks; `BYDAY` picks the days in each week,
  else the start's weekday.
- `MONTHLY` steps N months; `BYMONTHDAY` and `BYDAY` pick days (their
  intersection when both are given), else the start's day of the month. A
  month without that day is skipped, as RFC 5545 requires — a rule started
  on the 31st does not fire in April.
- `YEARLY` steps N years over `BYMONTH`, else the start's month; days are
  chosen within each month as for `MONTHLY`. A yearly rule naming days must
  name its months, because a position like `20MO` within a whole year is
  outside the subset. February 29 fires only in leap years.

Occurrences before the start are dropped. The end is inclusive.

## Reading a schedule

- `Next(after)` — the first occurrence strictly after a moment.
- `Between(from, to)` — the occurrences with `from <= t < to`.
- `Period(at)` — the occurrence at or before `at` and the next occurrence;
  when there is none, the schedule's end; when there is no end either, a zero
  end, which means the period is still open. A moment before the first
  occurrence has no period. Past periods stay answerable after the end
  (decision 336 keeps them).

A query far from the start skips straight to the period containing it, so
reading a daily schedule years on does not walk every day since. A rule
that can never match (February 30) gives up after 4000 empty periods and
answers nothing.

## Not decided here

Storage of a schedule (a rule string plus start, end and zone columns) and
the Stop action are the consumers' rows, not this package's.
