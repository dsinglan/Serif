import type { BillingClass, ClassCount, CodeCard, Flag, Modifier, Row, SearchAnswer } from './api'
import { placeLabel } from './places'
import { formatDate } from './status'

// money shows a price in dollars, such as "$1,108.00".
export function money(amount: number): string {
  return amount.toLocaleString('en-US', { style: 'currency', currency: 'USD' })
}

// count says how many of something, with the right plural: "1 NPI", "19 NPIs".
export function count(n: number, one: string, many: string): string {
  return `${n.toLocaleString('en-US')} ${n === 1 ? one : many}`
}

// whoIsCovered says who a row's contract covers: the group's NPIs, or the
// whole practice when the file lists only the placeholder NPI 0. In a
// search for one NPI, a row can stand for several groups that list that
// NPI at this price; their sizes differ, so it says how many groups.
export function whoIsCovered(row: Row): string {
  if (row.groups && row.groups > 1) {
    return `In ${row.groups} groups`
  }
  return row.tin_level ? 'Whole practice' : count(row.npis.length, 'NPI', 'NPIs')
}

// The flags' labels, short enough for a table cell.
export const flagLabels: Record<Flag, string> = {
  placeholder_provider: 'Placeholder provider',
  one_dollar_or_less: '$1 or less: unusually low',
  several_prices: 'Several prices for this NPI here',
}

// discontinued says when CMS stopped a code, from CMS's termination date:
// the last date Medicare providers may use it.
export function discontinued(date: string): string {
  return `Discontinued by CMS: last usable ${formatDate(date)}`
}

// nameOrNote is a code's name, or what to say when the file gives it none:
// two revenue codes have no standard name, and some procedures are never
// described.
export function nameOrNote(name: string, revenueCode: boolean): string {
  if (name) {
    return name
  }
  return revenueCode ? 'No standard name in this file' : 'Not described in this file'
}

// procedureName names a hospital row's procedure for its cell: the file's
// short name when it has one ("EMERGENCY DEPT VISIT"), else the full name,
// or what's missing.
export function procedureName(row: Row): string {
  return nameOrNote(row.procedure_short_name || row.procedure_label || '', false)
}

// facilityColumns says which code columns a facility table shows. Searching
// a revenue code, every row is billed as it, so only the procedure tells
// rows apart; searching a procedure, every row names it or is billed as it,
// so only what the row is billed as does.
export function facilityColumns(answer: SearchAnswer): { billedAs: boolean; procedure: boolean } {
  const revenueCode = answer.code?.type === 'RC'
  return { billedAs: !revenueCode, procedure: revenueCode }
}

// heading is the results heading for a code: its type, code and short name,
// the full name under it (once, when it says the same), and a note when CMS
// has discontinued it.
export function heading(code: CodeCard): { title: string; subtitle: string; note: string } {
  const title = code.short_name ? `${code.type} ${code.display} · ${code.short_name}` : `${code.type} ${code.display}`
  let subtitle = code.name !== code.short_name ? code.name : ''
  if (!code.name && !code.short_name) {
    subtitle = nameOrNote('', code.type === 'RC')
  }
  return { title, subtitle, note: code.discontinued ? discontinued(code.discontinued) : '' }
}

// cutNote explains the "…" the server puts on a name the rate file may have
// cut short, or is "" when no name shown has one. The file stops names at
// 100 characters (99213's ends "WHICH RE…").
export function cutNote(names: string[]): string {
  return names.some((name) => name.endsWith('…')) ? '… marks a name the rate file may have cut short: it stops names at 100 characters.' : ''
}

// feesOf names a class of fees the way the page does.
export function feesOf(billingClass: BillingClass): string {
  return billingClass === 'professional' ? 'Professional fees' : 'Facility fees'
}

// feeButton labels a fee switch button with how many fees it holds, such as
// "Professional fees (1,108)".
export function feeButton(c: ClassCount): string {
  return `${feesOf(c.billing_class)} (${c.count.toLocaleString('en-US')})`
}

// noFeesShown says the kind of fee on screen has none, when the other kind
// has some. Only an address can ask for that kind: its button is off. It
// counts the kind's fees, not the page's rows, which a place can empty.
export function noFeesShown(answer: SearchAnswer): string {
  const shown = answer.classes.find((c) => c.billing_class === answer.class)?.count ?? 0
  if (shown > 0 || !answer.classes.some((c) => c.count > 0)) {
    return ''
  }
  return `No ${feesOf(answer.class).toLowerCase()} for ${answer.code?.display ?? answer.query.code}.`
}

// caption sums up the table: which fees (at which place, when the table
// shows one), in which order, how many, from how many organizations, and
// which page; and, when the page compares fees, what with.
export function caption(answer: SearchAnswer): string {
  const place = answer.query.place ? ` at ${placeLabel(answer.query.place)}` : ''
  const order = answer.query.order === 'highest' ? 'highest price first' : 'lowest price first'
  const fees = count(answer.total, 'fee', 'fees')
  const organizations = count(answer.organization_count, 'organization', 'organizations')
  const compared = comparing(answer) ? ' The line under a price compares it with the middle price at its place.' : ''
  return `${feesOf(answer.class)}${place}, ${order}: ${fees} from ${organizations}. Page ${answer.page} of ${answer.pages}.${compared}`
}

// comparing says the page compares fees with the middle price at their
// place, like with like. It doesn't for a hospital's revenue code, whose
// fees mix procedures (0300 holds a prothrombin time and a testosterone
// test), or when the prices mix pricing modifiers, such as a professional
// reading (26) with whole tests.
export function comparing(answer: SearchAnswer): boolean {
  const s = answer.summary
  if (!s) {
    return false // under 5 prices: no summary, so no middle
  }
  return s.places.length > 0 && answer.code?.type !== 'RC' && !s.mixed_pricing
}

// versusMiddle compares a fee with the middle price at its place, from the
// summary: "20% above the $113.59 middle", "2% below the $71.38 middle", or
// "At the $51.45 middle" (within half a percent). With a place picked, the
// bar above the table names the middle, so the line says "the middle". A
// fee posted for several places is compared only once a place is picked; a
// fee at a place with no line in the summary (under 5 prices) can't be; and
// a flagged one isn't, since the summary leaves it out. "" when there's
// nothing to say.
export function versusMiddle(row: Row, answer: SearchAnswer): string {
  const v = versus(row, answer)
  if (v === null) {
    return ''
  }
  if (v.kind === 'several places') {
    return 'Posted for several places'
  }
  if (v.kind === 'too few') {
    return 'Too few to compare'
  }
  const middle = answer.query.place ? 'the middle' : `the ${money(v.middle)} middle`
  return v.kind === 'at' ? `At ${middle}` : `${v.percent}% ${v.kind} ${middle}`
}

// versusTally sums up the lines under a provider's fees on this page: how
// many are above, at and below the middle price at their place, and how
// many aren't compared (flagged ones included), so the counts add up to the
// page's fees. "" for a code alone, whose page is the market itself, and
// when no fee on the page is compared.
export function versusTally(answer: SearchAnswer): string {
  const { npi, ein, name } = answer.query
  if (!npi && !ein && !name) {
    return ''
  }
  const n = { above: 0, at: 0, below: 0, not: 0 }
  for (const row of answer.rows) {
    const v = versus(row, answer)
    n[!v || v.kind === 'several places' || v.kind === 'too few' ? 'not' : v.kind]++
  }
  const compared = [
    n.above ? `${n.above} above` : '',
    n.at ? `${n.at} at the middle` : '',
    n.below ? `${n.below} below` : '',
  ].filter((part) => part !== '')
  if (compared.length === 0) {
    return ''
  }
  const not = n.not ? `; ${n.not} not compared` : ''
  return `${answer.pages > 1 ? 'On this page, compared' : 'Compared'} with the middle price at their place: ${compared.join(', ')}${not}.`
}

// Versus is how a fee compares with the middle price at its place: above,
// below or at it, by a whole percent, or why it can't be compared.
type Versus = { kind: 'above' | 'below' | 'at'; percent: number; middle: number } | { kind: 'several places' } | { kind: 'too few' }

// versus compares a fee with the middle price at its place (see
// versusMiddle). null when the page compares nothing, or the fee is flagged.
function versus(row: Row, answer: SearchAnswer): Versus | null {
  const flagged = row.flags.includes('placeholder_provider') || row.flags.includes('one_dollar_or_less')
  if (!comparing(answer) || flagged) {
    return null
  }
  const picked = answer.query.place
  if (!picked && row.service_code.length > 1) {
    return { kind: 'several places' }
  }
  const line = answer.summary?.places.find((l) => l.code === (picked || row.service_code[0]))
  if (!line) {
    return { kind: 'too few' }
  }
  const percent = Math.round(((row.negotiated_rate - line.middle) / line.middle) * 100)
  return { kind: percent > 0 ? 'above' : percent < 0 ? 'below' : 'at', percent: Math.abs(percent), middle: line.middle }
}

// placeNote says which place the table shows, out of how many fees of the
// kind shown, and, when the page compares fees, the middle price there, or
// that none are there. "" when it shows every place.
export function placeNote(answer: SearchAnswer): string {
  const place = answer.query.place
  if (!place) {
    return ''
  }
  if (answer.total === 0) {
    return `None of these fees are at ${placeLabel(place)}.`
  }
  const all = answer.classes.find((c) => c.billing_class === answer.class)?.count ?? 0
  const line = comparing(answer) ? answer.summary?.places.find((l) => l.code === place) : undefined
  const middle = line ? ` The middle price there is ${money(line.middle)}.` : ''
  return `Only fees at ${placeLabel(place)}: ${count(answer.total, 'fee', 'fees')} of ${all.toLocaleString('en-US')}.${middle}`
}

// rowKey names a row uniquely, for Vue's list: where its price is in the
// file, and which provider group it's for. The price's place alone repeats,
// since one price can be for many groups.
export function rowKey(row: Row): string {
  const s = row.source
  return `${s.in_network_index}.${s.negotiated_rates_index}.${s.negotiated_prices_index}/${s.provider_group_id}.${s.provider_groups_index}`
}

// modifierMeaning is CMS's meaning for a modifier on the page, for a hover
// over its code in the table, such as "Obstetrical treatment/services,
// prenatal or postpartum (CMS)" for TH. "" when CMS has none: a CPT
// (number) modifier's text is the AMA's.
export function modifierMeaning(code: string, modifiers: Modifier[]): string {
  const meaning = modifiers.find((m) => m.code === code)?.meaning
  return meaning ? `${meaning} (CMS)` : ''
}

// legend explains the modifiers on the page: CMS's meaning for each letter
// modifier, and one line each for the CPT (number) modifiers, whose text
// the AMA licenses, and for the rest. The rest aren't in CMS's file, unless
// the server couldn't read CMS's file at all (cmsRead false).
export function legend(modifiers: Modifier[], cmsRead: boolean): { codes: string; meaning: string }[] {
  const lines = modifiers
    .filter((m) => m.meaning)
    .map((m) => ({ codes: m.code, meaning: `${m.meaning} (CMS)` }))
  const licensed = modifiers.filter((m) => m.licensed).map((m) => m.code)
  if (licensed.length) {
    lines.push({ codes: licensed.join(', '), meaning: 'CPT modifiers: the AMA licenses their text, so only the code is shown' })
  }
  const unknown = modifiers.filter((m) => !m.meaning && !m.licensed).map((m) => m.code)
  if (unknown.length) {
    const meaning = cmsRead ? "Not in CMS's HCPCS file" : "No meaning shown: CMS's HCPCS file couldn't be loaded"
    lines.push({ codes: unknown.join(', '), meaning })
  }
  return lines
}
