# Price Transparency Explorer

A searchable view of the negotiated rates in a CMS Transparency in Coverage rate file: what a health plan agreed to pay doctors and hospitals for a service, and how one provider compares with the market.

It reads Centene's [2026-08-31 Fidelis index](https://www.centene.com/content/dam/centene/Centene%20Corporate/json/DOCUMENT/2026-08-31_fidelis_index.json), selects the in-network file for 37 of Fidelis Care's individual-market plans, loads it into memory, and serves a JSON API and a Vue app over it.

**Stack:** Go (standard library only), Vue 3 + TypeScript + Vite.

## Who it's for

- **Providers:** search a code with your NPI, EIN or organization name to see your contracted rates first, each compared with the market's middle price at the same place of service.
- **Payers:** search a code alone to see the market: lowest, middle, highest and most common price, broken down by place of service, with the highest-priced contracts one click away.
- **Builders:** every view is one API call. Each results page links to its JSON, and every row points to its exact location in the source file.
- **Anyone without the code:** type words from its name, such as `office visit`, and pick from the codes whose names match.

## Quick start

Requires Go 1.24+ and Node 22.12+ (22, 24 or 26 and up). In two terminals, from the repo root:

```sh
# API: http://localhost:8080/api/status
cd backend
go run ./cmd/server

# Web app: http://localhost:5173
cd frontend
npm install
npm run dev
```

The first start downloads about 92 MB into `data/` (the index, the rate file and CMS's HCPCS file); later starts reuse it. `go run ./cmd/server -h` lists the options, such as `-refresh` and `-index <URL>`.

**Tests:** `cd backend && go test ./...` and `cd frontend && npm test`.

### Examples

- `99213`: an office visit, with the market by place of service
- `99213` with NPI `1780644302`: one doctor's rates, each compared with the market
- `450`: emergency room facility fees (the leading zero is optional)
- `27447`: a procedure that appears only inside hospital fees
- `office visit` or `xray`: search by words when you don't know the code

## Architecture

```
index ─► select the rate file by name (fidelis-ex) ─► download to data/
      ─► stream the rate file ─► join provider references to prices
      ─► remove repeats, index by code ─► JSON API ─► Vue app
```

| Part | Role |
|---|---|
| `backend/cmd/server` | Flags and startup. Loads the data in the background; searches get a 503 with progress until it's ready. |
| `backend/internal/fetch` | Downloads with retries, saved to a `.part` file and renamed once complete |
| `backend/internal/tic` | Reads the index and streams the rate file; malformed items are skipped and counted |
| `backend/internal/rates` | Joins, removes repeats, names codes, searches and summarizes |
| `backend/internal/cms` | CMS's HCPCS names and modifier meanings |
| `backend/internal/api` | HTTP handlers and input validation |
| `frontend/src` | `App.vue` runs the search flow; components draw the page; tested logic lives in `lib/` |

The rate file's 179,948 prices expand to 201,907 price and provider-group listings, which reduce to 29,446 rows once repeats are removed: about 28 MB in memory, ready about half a second after start.

## API

| Endpoint | Returns |
|---|---|
| `GET /api/search` | One page of rates for a code, with counts, the market summary and modifier meanings |
| `GET /api/codes?q=` | Codes whose names contain the words |
| `GET /api/status` | Load progress and the data source |

`/api/search` parameters: `code` (required), `npi`, `ein`, `name`, `class` (`professional` or `institutional`), `place` (such as `11`), `order` (`lowest` or `highest`), `page`, `page_size` (10 to 200, default 50).

```sh
curl 'localhost:8080/api/search?code=99213&npi=1780644302'
```

Errors are JSON: a 400 for bad input, naming the field at fault, and a 503 while the data loads or after loading fails (with the reason, also in `/api/status`). Each row's `source` gives its location in the rate file.

## Assumptions

- **One in-network file.** The index lists two Fidelis files, both for individual-market plans. This loads `fidelis-ex` (37 Ambetter plans, 89 MB) rather than `fidelis-es` (Essential Plan, 6 plans, 174 MB): the smaller file, covering more plans. It's selected by file name rather than position in the list.
- **One issuer, one set of plans.** Every rate in the file applies to all 37 of its plans, so there's no plan or issuer filter.
- **Dollar amounts.** Every price in this file is a negotiated dollar amount with no end date, so percentage, per-diem and expired rates aren't handled.
- **No addresses.** The file identifies providers by NPI and tax ID only, so location means place of service.
- **Contracted rates, not claims.** These are what the plan agreed to pay, not what was paid.

## Design decisions

- **In memory, no database.** The 89 MB file fits comfortably (about 28 MB once loaded); at scale this would be a database loaded on a schedule.
- **Repeats removed.** The file repeats prices heavily (0490's price list appears under 162 items); each row keeps a count.
- **Hospital fees found by procedure.** Facility fees are also indexed by the procedure in `additional_information`, so a code like 27447 is findable.
- **Official names only.** Names come from the file and CMS's HCPCS file; no CPT code set is bundled.
- **Market summary.** Leaves out flagged prices (placeholder providers, CPT prices of $1 or less), needs at least 5 prices, and uses a nearest-rank middle, so the middle is always a real posted price.
- **Like-for-like comparisons.** Fees are compared within their place of service, but not for revenue codes (they mix procedures) or codes whose prices mix pricing modifiers (such as 26 and TC, or NU and RR).
- **Validation in one place.** The Go API checks every input and the app shows its messages; the app itself only notices an empty code box.
- **Forgiving input.** NPIs can have spaces or dashes and EINs spaces, with or without their dash; organization names match by word, in any order, with punctuation ignored ("Unity Hospital of Rochester Inc"); and words in the code box look up codes by name.

## What the data shows

- One rate dominates 99213: $51.45 is 56% of its 1,108 professional fees, and 86% of those are telehealth.
- Place drives price: 99213's middle is $51.45 for telehealth outside the home and $113.59 in an office.
- ER facility fees run backwards: at 17 of the 18 hospitals that list both, the more severe visit (99285) is priced below 99284. Search `450` with name `tisch`: NYU Langone Hospital Tisch lists $323.53 for 99285 and $418.63 for 99284. The market middles don't show it, since different hospitals list each code.
- The file needs care: leading zeros are dropped from all 117 revenue codes, and names are cut at 100 characters.

## Limitations and next steps

- Formats this file doesn't use aren't handled: inline provider groups, remote provider references and `.json.gz` files.
- Search is code-first, as the brief asks: a provider can't yet list every code they're paid for (see next steps).
- The summary pools pricing modifiers such as 26 and TC (the page says when it does), downloads don't resume, and the server is meant for localhost.
- Word search matches billing names only: "ER" won't find "Emergency", and "knee MRI" finds nothing, since the file calls that scan "MRI JNT OF LWR EXTRE W/O DYE".
- **Next steps:**
  - Ingest every file in the index on a schedule into a database, keeping each release so prices can be tracked over time.
  - A provider page listing every code for an NPI or EIN.
  - NPPES lookups for provider names, specialties and addresses.
  - Comparison with Medicare's fee schedule.
  - A summary split by pricing modifier.

## AI use

Built with Claude Code (Anthropic's AI coding tool) as a pair: I made the product decisions, and each step was tested (Go tests, Vitest, mutation checks, Chrome at desktop and phone sizes) and reviewed by AI agents.

## Data and licensing

Centene's files and CMS's HCPCS file are public and download at runtime; none are committed. CPT is a registered trademark of the American Medical Association; the CPT descriptions shown come from Centene's rate file.
