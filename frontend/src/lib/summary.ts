import type { SearchAnswer, Summary, SummaryLine } from './api'
import { placeLabel } from './places'
import { count, feesOf, money, nameOrNote } from './rows'

// summaryTitle names what the box sums up: every fee of the kind shown, for
// the code, not only the ones the filters leave in the table.
export function summaryTitle(answer: SearchAnswer): string {
  return `All ${feesOf(answer.class).toLowerCase()} for ${answer.code?.display ?? answer.query.code}`
}

// marketFirst says the box comes before the table. A search for a code
// alone asks about the market. One that names a provider (an NPI, EIN or
// name) asks about that provider, so the page shows their fees first, and
// the market after them, to compare with.
export function marketFirst(answer: SearchAnswer): boolean {
  const { npi, ein, name } = answer.query
  return !npi && !ein && !name
}

// canPickPlaces says the box's places can narrow the table: the box comes
// before the table it narrows, and the kind of fee shown has fees, not all
// at one place, flagged fees aside (J1100's 50 are all at 11 Office, so
// picking it would change nothing). After a provider's fees, the box is only
// there to compare with: a place picked there would change the table above
// it, and most places would have none of their fees.
export function canPickPlaces(answer: SearchAnswer): boolean {
  const shown = answer.classes.find((c) => c.billing_class === answer.class)?.count ?? 0
  const places = answer.summary?.places ?? []
  const leftOut = answer.summary?.left_out ?? 0 // flagged fees count in no line
  const allAtOne = places.length === 1 && (places[0]?.count ?? 0) + leftOut === shown
  return marketFirst(answer) && shown > 0 && !allAtOne
}

// scope says how many prices the box sums up, from how many organizations,
// and, when they name more than one procedure, how many name one: in many
// revenue codes most name none (0450: 63 of 207).
export function scope(s: Summary): string {
  const prices = `${count(s.count, 'price', 'prices')} from ${count(s.organization_count, 'organization', 'organizations')}`
  if (s.procedure_count < 2) {
    return prices
  }
  const procedures = s.procedure_count.toLocaleString('en-US')
  return s.with_procedure === s.count
    ? `${prices}, for ${procedures} different procedures`
    : `${prices}; ${s.with_procedure.toLocaleString('en-US')} name one of ${procedures} procedures`
}

// modeShare says how many prices are the most common one, and from how
// many organizations, or "" when there's no most common price to show.
export function modeShare(s: Summary): string {
  if (s.most_common_count === 0) {
    return ''
  }
  const percent = Math.round((s.most_common_count / s.count) * 100)
  return `${s.most_common_count.toLocaleString('en-US')} of ${count(s.count, 'price', 'prices')} (${percent}%), from ${count(s.most_common_organizations, 'organization', 'organizations')}`
}

// procedureHeading heads the procedure lines, saying when some procedures
// aren't shown. The "No procedure named" line isn't a procedure, so it
// doesn't count.
export function procedureHeading(s: Summary): string {
  const shown = s.procedures.filter((line) => line.code !== '').length
  return s.procedure_count > shown ? `By procedure: ${shown} of ${s.procedure_count.toLocaleString('en-US')} shown` : 'By procedure'
}

// procedureLabel names a procedure line: its code and name, or what's
// missing.
export function procedureLabel(line: SummaryLine): string {
  return line.code === '' ? 'No procedure named' : `${line.code} · ${nameOrNote(line.name, false)}`
}

// lineNumbers spells out a line in words, for everyone, screen readers
// included: the bar above it only draws the same numbers.
export function lineNumbers(line: SummaryLine): string {
  return `middle ${money(line.middle)} · ${money(line.lowest)} to ${money(line.highest)} · ${count(line.count, 'price', 'prices')}`
}

// bar places a line on the box's scale, which runs from the lowest price
// to the highest: where its range starts and how wide it is, and where its
// middle is, all as percentages for CSS.
export function bar(line: SummaryLine, s: Summary): { left: string; width: string; middle: string } {
  const span = s.highest - s.lowest
  const at = (price: number) => (span > 0 ? ((price - s.lowest) / span) * 100 : 0)
  const percent = (n: number) => `${n.toFixed(1)}%`
  return { left: percent(at(line.lowest)), width: percent(at(line.highest) - at(line.lowest)), middle: percent(at(line.middle)) }
}

// statsNote warns, under the box's numbers, when they don't describe one
// kind of price: when the prices mix modifiers that change what's priced
// (71046's mix a reading alone with the equipment's part), or when the
// middle differs a lot by place, so the one middle shouldn't be read as
// every place's (99213's is the telehealth middle, $51.45, while an office
// visit's is $113.59). A lot is 1.5 times or more. "" when neither.
export function statsNote(s: Summary): string {
  if (s.mixed_pricing) {
    return "These prices mix modifiers that change what's priced, so these numbers mix different things, and fees aren't compared with them."
  }
  // The lowest and highest middles. Places come with the most prices first,
  // so a tie names the place with more.
  const [first, ...rest] = s.places
  if (!first) {
    return ''
  }
  let low = first
  let high = first
  for (const line of rest) {
    if (line.middle < low.middle) {
      low = line
    }
    if (line.middle > high.middle) {
      high = line
    }
  }
  if (high.middle < low.middle * 1.5) {
    return ''
  }
  return `The middle price varies by place, from ${money(low.middle)} at ${placeLabel(low.code)} to ${money(high.middle)} at ${placeLabel(high.code)}.`
}

// noSummary says why there's no box, when the code's fees of the kind shown
// are too few to sum up: the box needs 5 prices, flagged prices left out
// (36415 has 4 facility fees; 94760's 59 doctor fees are all $1 or less).
// "" when there's a box, or the kind shown has no fees to sum up.
export function noSummary(answer: SearchAnswer): string {
  const shown = answer.classes.find((c) => c.billing_class === answer.class)?.count ?? 0
  const filtered = !marketFirst(answer) // the counts are the provider's, and the box is the market's
  if (answer.summary || (shown === 0 && !filtered)) {
    return ''
  }
  const code = answer.code?.display ?? answer.query.code
  return `No market summary for ${code}'s ${feesOf(answer.class).toLowerCase()}: it needs at least 5 prices, not counting flagged ones.`
}

// notes say how to read the box.
export function notes(s: Summary): string[] {
  const out = ['Contracted rates: what the plan agreed to pay, not payments on claims.']
  if (s.places.length > 0) {
    out.push('A price posted for several places counts in each.')
  }
  if (s.left_out > 0) {
    out.push(`${count(s.left_out, 'flagged price', 'flagged prices')} left out.`)
  }
  return out
}
