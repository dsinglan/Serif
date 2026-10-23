// Types for the Go API's answers, and the calls that fetch them.

// Status is GET /api/status: where loading is, and once ready, where the
// data came from.
export interface Status {
  phase: 'starting' | 'fetching_index' | 'downloading' | 'fetching_cms_names' | 'reading' | 'ready' | 'failed'
  error?: string
  bytes_done?: number
  bytes_total?: number // -1 when the server doesn't say
  source?: Source
}

// Source is where the data came from and what loading counted. The page
// uses it for its ready line, the saved file's name in the jq commands, and
// whether CMS's names loaded. A count's name ends in _count; a name like
// plan_names is the list itself.
export interface Source {
  index_url: string
  index_last_updated_on: string
  reporting_entity_name: string
  reporting_entity_type: string // who posted the file, such as "Third Party Administrator"
  file_url: string
  file_version: string
  file_last_updated_on: string
  file_bytes: number
  issuer_names: string[] // the plans' issuers, such as "Fidelis Care"
  plan_market_types: string[] // such as "individual"
  plan_names: string[]
  reporting_plan_count: number
  row_count: number
  listing_count: number
  skipped_item_count: number
  unlinked_listing_count: number
  missing_rate_count: number
  // CMS's HCPCS file: official names for HCPCS codes and modifiers (optional)
  hcpcs_url: string
  hcpcs_file?: string
  hcpcs_code_count: number
  hcpcs_modifier_count: number
  hcpcs_error?: string
}

// getStatus asks the server where loading is.
export async function getStatus(): Promise<Status> {
  const res = await fetch('/api/status')
  if (!res.ok) {
    throw new Error(`The server answered ${res.status} for /api/status.`)
  }
  return res.json()
}

// BillingClass is the file's kind of fee: a doctor's (professional) or a
// facility's (institutional). The page shows one at a time, with a switch
// between them.
export type BillingClass = 'professional' | 'institutional'

// SearchQuery is the search as the server read it: a box left empty is "",
// and a page or page size nobody asked for is 0.
export interface SearchQuery {
  code: string
  npi: string
  ein: string
  name: string
  class: BillingClass | ''
  place: string // a place of service, such as "11", or "" for every place
  order: '' | 'lowest' | 'highest' // "highest" shows the highest price first; "" or "lowest", the lowest
  page: number
  page_size: number
}

// CodeCard says what the searched code is, for the results heading.
export interface CodeCard {
  type: string // CPT, HCPCS or RC
  display: string // at its standard width, such as "0450"
  short_name: string // "" when the file has none
  name: string // "" when neither the file nor CMS names it
  discontinued?: string // HCPCS: CMS's last date for the code, such as "2026-09-30"
}

// Flag marks a row worth a second look. Flagged rows are kept, with a label.
export type Flag = 'placeholder_provider' | 'one_dollar_or_less' | 'several_prices'

// Row is one provider rate: one negotiated price for one provider group.
export interface Row {
  billing_code_type: string // CPT, HCPCS or RC
  billing_code: string // as in the file, such as "450"
  display_code: string // at its standard width, such as "0450"
  label: string // the code's name
  procedure_code?: string // facility rows: the procedure in the side field, such as "27447"
  procedure_label?: string
  procedure_short_name?: string // the file's short name for it, such as "EMERGENCY DEPT VISIT"
  negotiation_arrangement: string
  discontinued?: string // HCPCS: CMS's last date for the row's own code
  procedure_discontinued?: string // the same, for the procedure in the side field
  negotiated_type: string
  negotiated_rate: number
  billing_class: BillingClass
  setting: string
  service_code: string[] // places of service, such as "11" for an office
  billing_code_modifier: string[]
  expiration_date: string // "9999-12-31" means no end date
  additional_information?: string
  tin_type: string // ein or npi
  tin: string
  business_name: string
  npis: string[]
  invalid_npis: string[]
  tin_level: boolean // the contract is with the whole practice, not named clinicians
  groups?: number // NPI searches only: how many groups list this doctor at this price
  flags: Flag[]
  source: RowSource
}

// RowSource is where the row was first listed in the rate file.
export interface RowSource {
  in_network_index: number
  negotiated_rates_index: number
  negotiated_prices_index: number
  provider_group_id: string
  provider_groups_index: number
  listed_times: number // how many identical listings the row stands for
}

// ClassCount is one class's count: its rows, and the organizations they
// come from.
export interface ClassCount {
  billing_class: BillingClass
  count: number // rows
  organization_count: number // different organizations among them, by tax ID
}

// Modifier explains one modifier code on the page.
export interface Modifier {
  code: string
  meaning?: string // CMS's official meaning, for letter modifiers
  source?: string // "CMS" when there's a meaning
  licensed?: boolean // a CPT (number) modifier: the AMA licenses its text, so there's no meaning
}

// Organizations are the names the searched NPI and EIN are listed under in
// the file; a list is empty when its box wasn't searched or matches nothing.
export interface Organizations {
  npi: string[]
  ein: string[]
}

// SummaryLine sums up the prices for one place of service, or one
// procedure.
export interface SummaryLine {
  code: string // a place, such as "11"; a procedure, such as "45380", or "" for prices that name none
  name: string // a procedure's short name in the file; else its full name (CMS's for a HCPCS code, such as H0020); "" for places and unnamed procedures
  count: number
  lowest: number
  middle: number
  highest: number
}

// Summary sums up every price for the code in the class shown, before the
// NPI, EIN and name narrow them. Flagged prices are left out.
export interface Summary {
  count: number // prices summed up
  left_out: number // flagged prices, not counted
  organization_count: number
  procedure_count: number // different procedures the prices name in hospitals' side fields
  with_procedure: number // prices that name one; in many revenue codes most name none
  lowest: number
  middle: number // by nearest rank, so always a price someone posted
  highest: number
  most_common: number // 0 unless 2+ organizations post it and it's 10%+ of the prices
  most_common_count: number
  most_common_organizations: number
  places: SummaryLine[] // places with 5+ prices, the most first
  procedures: SummaryLine[] // up to 10 procedures with 5+ prices, the most first; empty unless 2+ qualify
  mixed_pricing: boolean // the prices mix pricing modifiers, such as a reading (26) with whole tests, so the middles mix services
}

// SearchAnswer is GET /api/search's answer when it works (a 200).
export interface SearchAnswer {
  query: SearchQuery
  code: CodeCard | null // null when the code isn't in the file
  code_found: boolean // the code has rows, before the NPI, EIN and name narrow them
  provider_found: boolean // some row in the file matches every NPI, EIN and name given
  not_found: ('npi' | 'ein' | 'name')[] // boxes that match nothing in the file
  provider_organizations: Organizations
  classes: ClassCount[] // both classes, always
  class: BillingClass // the class shown
  summary: Summary | null // null when there are fewer than 5 prices
  total: number // rows in that class, at the place asked for
  organization_count: number // different tax IDs among those rows
  page: number // the page shown
  pages: number
  page_size: number // rows per page
  rows: Row[] // that page's rows
  modifiers: Modifier[] // what the modifiers on these rows mean
}

// Field is what a 400 is about: one of the form's boxes (code, npi, ein,
// name), another part of the address (class, place, order, page, page_size,
// or q, the words of a lookup of codes by name), or "" for the address as a
// whole.
// Only the boxes have a place on the form for a message; the page shows the
// rest in the line under the form.
export type Field = 'code' | 'npi' | 'ein' | 'name' | 'class' | 'place' | 'order' | 'page' | 'page_size' | 'q' | ''

// SearchOutcome is every way a search can end, so the page always has
// something to show.
export type SearchOutcome =
  | { kind: 'results'; answer: SearchAnswer }
  | { kind: 'invalid'; field: Field; message: string } // 400: bad input
  | { kind: 'loading'; status: Status } // 503: the data is still loading; describeStatus(status) says how far
  | { kind: 'failed'; status: Status } // 503: loading failed (status.error says why), so the server needs a restart
  | { kind: 'unreachable' } // no answer from the Go server
  | { kind: 'error'; message: string } // anything else, such as a crash's 500

// ErrorBody is the JSON the server sends when a request doesn't work.
interface ErrorBody {
  error?: { code: string; field?: Field; message: string }
  status?: Status // with a 503: where loading is
}

// CodeMatch is a code whose names contain the words someone typed: its
// card, how many prices the file has for it, and whether its short name
// alone has every word (when it doesn't, the full name says why it's there).
export interface CodeMatch extends CodeCard {
  count: number // prices of both kinds, as a search for the code finds them
  short_name_matches: boolean
}

// CodeGroup is one kind of code's matches: up to 10, the most prices first,
// and how many there are in all.
export interface CodeGroup {
  total: number
  codes: CodeMatch[]
}

// CodesAnswer is GET /api/codes's answer: the codes whose names contain
// the words, in two lists that are never ranked against each other.
export interface CodesAnswer {
  query: string // the words, as typed (trimmed)
  charges: CodeGroup // revenue codes: hospitals' charge codes
  services: CodeGroup // CPT and HCPCS codes
}

// Problem is every way a request can end without an answer.
export type Problem = Exclude<SearchOutcome, { kind: 'results' }>

// CodesOutcome is how a lookup of codes by name ends: the codes, or the
// same problems a search can have.
export type CodesOutcome = { kind: 'codes'; answer: CodesAnswer } | Problem

// findCodes asks the server for the codes whose names contain the words.
// Like searchApi, it never throws.
export async function findCodes(words: string): Promise<CodesOutcome> {
  let res: Response
  try {
    res = await fetch(`/api/codes?${new URLSearchParams({ q: words })}`)
  } catch {
    return { kind: 'unreachable' }
  }
  const body = await readJSON(res)
  if (res.ok && body) {
    return { kind: 'codes', answer: body as CodesAnswer }
  }
  return problem(res, body)
}

// searchAddress is the API's address for a search: what searchApi fetches,
// and what the page links to as JSON.
export function searchAddress(params: URLSearchParams): string {
  return `/api/search?${params}`
}

// searchApi asks the server for one search. It never throws: every outcome,
// even no answer at all, becomes a SearchOutcome the page can show.
export async function searchApi(params: URLSearchParams): Promise<SearchOutcome> {
  let res: Response
  try {
    res = await fetch(searchAddress(params))
  } catch {
    return { kind: 'unreachable' } // no answer at all, not even from Vite
  }
  const body = await readJSON(res)
  if (res.ok && body) {
    return { kind: 'results', answer: body as SearchAnswer }
  }
  return problem(res, body)
}

// problem says what went wrong when the answer isn't the one asked for: an
// error status, or a 200 whose JSON can't be read.
function problem(res: Response, body: unknown): Problem {
  const { error, status } = (body ?? {}) as ErrorBody
  if (res.status === 400 && error) {
    return { kind: 'invalid', field: error.field ?? '', message: error.message }
  }
  if (res.status === 503 && error && status) {
    if (error.code === 'load_failed') {
      return { kind: 'failed', status }
    }
    return { kind: 'loading', status }
  }
  if (res.status === 502 && !body) {
    return { kind: 'unreachable' } // Vite's answer when the Go server is off: an empty 502
  }
  return { kind: 'error', message: error?.message ?? (res.ok ? "The server's answer couldn't be read." : `The server answered ${res.status}.`) }
}

// readJSON reads an answer's body as JSON when the server says it's JSON.
// Otherwise, or when the JSON can't be read, it returns null.
async function readJSON(res: Response): Promise<unknown> {
  if (!res.headers.get('Content-Type')?.includes('application/json')) {
    return null
  }
  try {
    return await res.json()
  } catch {
    return null // cut off or garbled
  }
}
