import type { CodeGroup, CodeMatch, CodesAnswer, CodesOutcome, SearchOutcome } from './api'
import { listOf } from './outcome'
import { count } from './rows'

// buttons is how many codes of each kind are offered as buttons: the ones
// with the most prices, which are almost always the ones wanted.
export const buttons = 5

// wordsToLookUp says which words to look up in codes' names after a search
// hit a dead end a name could get past: a word passed for a code but isn't
// one in the file ("knee", "MRI"), or the server didn't take the text as a
// code. "" means don't look (see asWords).
export function wordsToLookUp(typed: string, outcome: SearchOutcome): string {
  const deadEnd =
    (outcome.kind === 'invalid' && outcome.field === 'code') || (outcome.kind === 'results' && !outcome.answer.code_found)
  return deadEnd ? asWords(typed) : ''
}

// wordsOnly is the code box's text when it can only be words: it isn't 3 to
// 7 letters or digits, the server's rule for a code, nor a code after its
// type, such as "CPT 99213" (see asWords). The page then looks the words up
// in codes' names directly ("office visit", "x-ray", "emergency"), since a
// search would only be rejected, with a 400 the browser logs as an error.
// "" for anything that could be a code: the server says what's wrong with
// "99-213!", and whether "knee" is in the file.
export function wordsOnly(typed: string): string {
  const text = typed.replace(/^[\s\u0085]+|[\s\u0085]+$/g, '') // trimmed as the server trims it, U+0085 included
  return /^[a-z0-9]{3,7}$/i.test(text) ? '' : asWords(text)
}

// asWords is the code box's text as words to look up, or "" when it isn't:
// one word with a digit (70450, J9999, or CPT 70450 with its type) is meant
// as a code, so its own message stands, but a code has no spaces, so "covid
// 19" is words. The server takes 2 to 100 characters; the letter is this
// page's own rule.
function asWords(typed: string): string {
  const words = typed.trim()
  const fits = words.length >= 2 && words.length <= 100
  const code = words.replace(/^(CPT|HCPCS|RC)\s+/i, '')
  const aCode = !/\s/.test(code) && /\d/.test(code)
  return fits && /[a-z]/i.test(words) && !aCode ? words : ''
}

// afterLookup says what the page shows once a search and its lookup are
// back. With codes to pick from, it shows them. If the lookup couldn't be
// done, as while the data loads, it shows that problem: it's about what the
// person meant. With no lookup, it shows the search's own outcome. A search
// skipped for words (see wordsOnly) has no outcome of its own.
export function afterLookup(
  search: SearchOutcome | null,
  lookup: CodesOutcome | null,
): { outcome: SearchOutcome | null; suggestions: CodesAnswer | null } {
  if (lookup?.kind === 'codes') {
    return { outcome: search, suggestions: lookup.answer }
  }
  return { outcome: lookup ?? search, suggestions: null }
}

// matchCount is how many codes' names contain the words, of both kinds.
export function matchCount(s: CodesAnswer): number {
  return s.charges.total + s.services.total
}

// suggestionsTitle heads the codes to pick from.
export function suggestionsTitle(s: CodesAnswer): string {
  return matchCount(s) > 0 ? `Codes whose names contain “${s.query}”` : `No code's name contains “${s.query}”`
}

// suggestionsLine sums up the suggestions for the line under the form, which
// a screen reader reads out.
export function suggestionsLine(s: CodesAnswer): string {
  if (matchCount(s) === 0) {
    return `${suggestionsTitle(s)}.`
  }
  const kinds = []
  if (s.charges.total > 0) {
    kinds.push(count(s.charges.total, 'hospital charge code', 'hospital charge codes'))
  }
  if (s.services.total > 0) {
    kinds.push(count(s.services.total, 'service code', 'service codes'))
  }
  return `Codes whose names contain “${s.query}”: ${listOf(kinds)}. Pick one below.`
}

// codeLabel names a suggested code the way the results' heading does:
// "CPT 99213 · OFFICE/OUTPATIENT VISIT EST". A code found by its names has
// at least one: a HCPCS code seen only inside hospital fees has only CMS's
// full name ("HCPCS J0640 · Injection, leucovorin calcium, per 50 mg").
export function codeLabel(m: CodeMatch): string {
  return `${m.type} ${m.display} · ${m.short_name || m.name}`
}

// showFullName reports whether a code's button shows its full name too:
// when its short name lacks some of the words, so the full name says why
// the code is there ("knee" finds 20611, DRAIN/INJ JOINT/BURSA W/US,
// through its full name). The server says which name has the words.
export function showFullName(m: CodeMatch): boolean {
  return m.short_name !== '' && !m.short_name_matches
}

// moreLine counts the codes of a kind that aren't buttons, and says how to
// reach them; "" when every code is a button.
export function moreLine(group: CodeGroup): string {
  const more = group.total - Math.min(group.codes.length, buttons)
  return more > 0 ? `And ${more.toLocaleString('en-US')} more: add a word to narrow them down.` : ''
}

// sameNames names the codes among the buttons that the file gives the same
// names to, such as 99213, 99214 and 99215, so a person knows their names
// won't tell them apart. "" when no two buttons share them.
export function sameNames(group: CodeGroup): string {
  const byName = new Map<string, string[]>()
  for (const m of group.codes.slice(0, buttons)) {
    const key = `${m.short_name}\n${m.name}`
    byName.set(key, [...(byName.get(key) ?? []), `${m.type} ${m.display}`])
  }
  const shared = [...byName.values()].filter((codes) => codes.length > 1).map(listOf)
  return shared.length > 0 ? `The file gives these the same names: ${shared.join('; ')}.` : ''
}
