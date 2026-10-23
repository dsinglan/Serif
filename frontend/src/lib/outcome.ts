import type { BillingClass, CodesAnswer, Field, SearchAnswer, SearchOutcome } from './api'
import { placeLabel } from './places'
import { feesOf, placeNote } from './rows'
import type { SearchBoxes } from './search'

// ProviderBox is one of the boxes that narrow a search to a provider.
export type ProviderBox = 'npi' | 'ein' | 'name'

// The provider boxes, by the names messages use for them.
const boxNames: Record<ProviderBox, string> = { npi: 'NPI', ein: 'EIN', name: 'organization name' }

// isBox reports whether a field is one of the form's boxes. A message about
// a box is shown under that box; any other is shown in the line under the
// form.
export function isBox(field: Field): field is keyof SearchBoxes {
  return field === 'code' || field === 'npi' || field === 'ein' || field === 'name'
}

// describeOutcome says how a search ended, in the line under the form. A
// message about one of the boxes goes under that box instead, so for that
// it says "".
export function describeOutcome(outcome: SearchOutcome): string {
  switch (outcome.kind) {
    case 'results':
      return describeAnswer(outcome.answer)
    case 'invalid':
      return isBox(outcome.field) ? '' : outcome.message
    case 'loading':
      return "Still loading the data. Your search will run as soon as it's ready."
    case 'failed':
      return "Searches can't run until the data loads. The status line above says what went wrong."
    case 'unreachable':
      return "The search didn't reach the server." // the status line above says how to start it
    case 'error':
      return outcome.message
  }
}

// isProblem reports whether an outcome is something to fix, shown in the
// error color. Results aren't, and neither is waiting for the data.
export function isProblem(outcome: SearchOutcome): boolean {
  return outcome.kind !== 'results' && outcome.kind !== 'loading'
}

// canTryAgain reports whether sending the same search again might help: after
// no answer, an error, or a failed load, once the server is restarted.
export function canTryAgain(outcome: SearchOutcome): boolean {
  return outcome.kind === 'unreachable' || outcome.kind === 'error' || outcome.kind === 'failed'
}

// BoxMessage is what to say under one box: the server's error about it
// (invalid), or a note that the file has nothing for it.
export interface BoxMessage {
  text: string
  invalid: boolean
}

// boxMessages says what goes under each box after a search: the server's
// message about bad input, or, for a box that matches nothing in the file,
// a note saying so.
export function boxMessages(outcome: SearchOutcome | null): Partial<Record<keyof SearchBoxes, BoxMessage>> {
  if (outcome?.kind === 'invalid' && isBox(outcome.field)) {
    return { [outcome.field]: { text: outcome.message, invalid: true } }
  }
  if (outcome?.kind !== 'results') {
    return {}
  }
  const notes: Partial<Record<keyof SearchBoxes, BoxMessage>> = {}
  for (const box of outcome.answer.not_found) {
    // Several words in the name box match one by one, so what's missing is a
    // name with all of them.
    const words = box === 'name' && outcome.answer.query.name.trim().includes(' ')
    notes[box] = { text: words ? "No organization's name in this file has all of these words." : notFoundNotes[box], invalid: false }
  }
  return notes
}

// missingCode is what to say under the code box when Search is pressed with
// it empty, or "" when it isn't. A provider's NPI, EIN or name alone isn't a
// search here: their rates are shown for one code at a time, so with one of
// those typed, the message says so.
export function missingCode(boxes: SearchBoxes): string {
  if (boxes.code.trim() !== '') {
    return ''
  }
  if ((['npi', 'ein', 'name'] as const).some((box) => boxes[box].trim() !== '')) {
    return "Enter a billing code too, such as 99213: a provider's rates are shown one code at a time."
  }
  return 'Enter a billing code, such as 99213, or words from its name, such as office visit.'
}

// What to say under a provider box that matches nothing in the file.
const notFoundNotes: Record<ProviderBox, string> = {
  npi: 'No provider group in this file lists this NPI.',
  ein: 'No organization in this file has this EIN.',
  name: "No organization's name in this file has a word starting with this.",
}

// ProviderNote is a lead-in and a list of organization names. The names go
// in a list, not a sentence, because many have commas or "and" in them
// ("ENT & Allergy Associates, LLP").
export interface ProviderNote {
  lead: string
  names: string[]
}

// providerNotes says, when a search found no rows, which organizations the
// file lists the searched NPI and EIN under, so the person can see why. (The
// server leaves a list empty when its box wasn't searched.)
export function providerNotes(answer: SearchAnswer): ProviderNote[] {
  if (rowCount(answer) > 0 || !answer.code_found) {
    return []
  }
  const { npi, ein } = answer.provider_organizations
  const notes: ProviderNote[] = []
  if (npi.length > 0) {
    notes.push({ lead: `NPI ${answer.query.npi} is listed under:`, names: firstNames(npi) })
  }
  if (ein.length > 0) {
    notes.push({ lead: `EIN ${answer.query.ein} is listed as:`, names: firstNames(ein) })
  }
  return notes
}

// describeView says which kind of fee, place, order and page are on screen.
// It's read out with the line under the form, but not shown: after a fee
// button, a place, an order button or the pager, that line has the same
// totals as before, and the page shows the change in the pressed button and
// the table's caption. When a place leaves no rows, it says so.
export function describeView(answer: SearchAnswer): string {
  if (answer.rows.length === 0) {
    return placeNote(answer) // "None of these fees are at …", or ""
  }
  const fees = feesOf(answer.class).toLowerCase()
  const place = answer.query.place ? ` at ${placeLabel(answer.query.place)}` : ''
  const order = answer.query.order === 'highest' ? ', highest price first' : ''
  return answer.pages > 1
    ? `Showing ${fees}${place}${order}, page ${answer.page} of ${answer.pages}.`
    : `Showing ${fees}${place}${order}.`
}

// clearableBoxes are the provider boxes a search used when it found no rows,
// so the page can offer to clear each one and search again.
export function clearableBoxes(answer: SearchAnswer): ProviderBox[] {
  if (rowCount(answer) > 0 || !answer.code_found) {
    return []
  }
  return (['npi', 'ein', 'name'] as const).filter((box) => answer.query[box] !== '')
}

// clearLabel names the button that clears one provider box.
export function clearLabel(box: ProviderBox): string {
  return `Clear the ${boxNames[box]}`
}

// describeAnswer sums up an answer: how many fees of each kind it found or,
// when it found none, why.
function describeAnswer(answer: SearchAnswer): string {
  const code = answer.code?.display ?? answer.query.code
  if (!answer.code_found) {
    return `${code} isn't in this file: no price lists it, as its own code or as a facility fee's procedure. Try another code, or words from its name, such as office visit.`
  }
  if (rowCount(answer) > 0) {
    return `${code}${forWhom(answer)}: ${fees(count(answer, 'professional'), 'professional')} and ${fees(count(answer, 'institutional'), 'facility')}.`
  }
  if (answer.not_found.length > 0) {
    return `No ${code} price: the file has nothing for the ${listOf(answer.not_found.map((box) => boxNames[box]))} you gave.`
  }
  // Each box matches something in the file. Either they never match the
  // same price, or they do, just not for this code. That's all it says: an
  // NPI search skips whole-practice prices, which list no NPIs, so the NPI's
  // practice may still have a price for this code.
  const given = (['npi', 'ein', 'name'] as const).filter((box) => answer.query[box] !== '')
  const boxes = listOf(given.map((box) => boxNames[box]))
  if (!answer.provider_found) {
    return `No price in this file has this ${boxes} together.`
  }
  return `No ${code} price matches this ${boxes}${given.length > 1 ? ' together' : ''}.`
}

// pageTitle names the browser tab after what's on screen, so tabs and the
// history list tell searches apart: "99213 for NPI 1780644302 · Price
// Transparency Explorer", the words looked up ("“office visit” · …"), or
// the app's name alone before a search. (After a problem, App.vue leaves
// the title as it was.)
export function pageTitle(outcome: SearchOutcome | null, suggestions: CodesAnswer | null): string {
  const app = 'Price Transparency Explorer'
  if (suggestions) {
    return `“${suggestions.query}” · ${app}`
  }
  if (outcome?.kind === 'results') {
    return `${outcome.answer.code?.display ?? outcome.answer.query.code}${forWhom(outcome.answer)} · ${app}`
  }
  return app
}

// forWhom names the provider a search narrowed to, so the line counting its
// fees says whose they are: " for NPI 1780644302", or " for EIN 13-4000192
// and organization name “ent”". "" for a code alone.
function forWhom(answer: SearchAnswer): string {
  const { npi, ein, name } = answer.query
  const who = [npi ? `NPI ${npi}` : '', ein ? `EIN ${ein}` : '', name ? `organization name “${name}”` : ''].filter((w) => w !== '')
  return who.length > 0 ? ` for ${listOf(who)}` : ''
}

function count(answer: SearchAnswer, billingClass: BillingClass): number {
  return answer.classes.find((c) => c.billing_class === billingClass)?.count ?? 0
}

function rowCount(answer: SearchAnswer): number {
  return answer.classes.reduce((n, c) => n + c.count, 0)
}

// fees says "1 professional fee" or "1,108 professional fees".
function fees(n: number, kind: string): string {
  return `${n.toLocaleString('en-US')} ${kind} ${n === 1 ? 'fee' : 'fees'}`
}

// listOf joins words for a sentence: "NPI", "NPI and EIN", "NPI, EIN and organization name".
export function listOf(words: string[]): string {
  return words.length < 2 ? words.join('') : `${words.slice(0, -1).join(', ')} and ${words[words.length - 1]}`
}

// firstNames keeps up to three names, then says how many more.
function firstNames(list: string[]): string[] {
  return list.length > 3 ? [...list.slice(0, 3), `and ${list.length - 3} more`] : list
}
