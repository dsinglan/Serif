import { describe, expect, it } from 'vitest'
import type { CodeGroup, CodeMatch, CodesAnswer, SearchAnswer, SearchOutcome, Status } from './api'
import {
  afterLookup,
  codeLabel,
  matchCount,
  moreLine,
  sameNames,
  showFullName,
  suggestionsLine,
  suggestionsTitle,
  wordsOnly,
  wordsToLookUp,
} from './suggest'

// match is a code as GET /api/codes sends it: "CPT 99213", its prices'
// count, its names, and whether the short name alone has every word.
function match(code: string, count: number, short_name = '', name = '', short_name_matches = true): CodeMatch {
  const [type = '', display = ''] = code.split(' ')
  return { type, display, short_name, name, count, short_name_matches }
}

const group = (total: number, codes: CodeMatch[]): CodeGroup => ({ total, codes })
const none = group(0, [])

// The file's names for some office visits: it cuts full names off at 100
// characters, so 99213, 99214 and 99215 end up with the same ones, and so
// do 99204 and 99203.
const established = 'OFFICE OR OTH OUTPATIENT VISIT FOR THE EVALUATION AND MANAGEMENT OF AN ESTABLISHED PATIENT, WHICH RE…'
const newPatient = 'OFFICE OR OTH OUTPATIENT VISIT FOR THE EVALUATION AND MANAGEMENT OF A NEW PATIENT, WHICH REQUIRES TH…'
const v99214 = match('CPT 99214', 1699, 'OFFICE/OUTPATIENT VISIT EST', established)
const v99213 = match('CPT 99213', 1210, 'OFFICE/OUTPATIENT VISIT EST', established)
const v99215 = match('CPT 99215', 335, 'OFFICE/OUTPATIENT VISIT EST', established)
const v99212 = match('CPT 99212', 333, 'OFFICE OR OTH OUTPATIENT VISIT', established)
const v99204 = match('CPT 99204', 231, 'OFFICE/OUTPATIENT VISIT NEW', newPatient)
const v99211 = match(
  'CPT 99211',
  208,
  'OFFICE/OUTPATIENT VISIT EST',
  'OFFICE OR OTHER OUTPATIENT VISIT FOR THE EVALUATION AND MANAGEMENT OF AN ESTABLISHED PATIENT, THAT M…',
)
const v99203 = match('CPT 99203', 145, 'OFFICE/OUTPATIENT VISIT NEW', newPatient)

// found is a search's answer for a code that is, or isn't, in the file.
// wordsToLookUp reads only code_found.
const found = (codeFound: boolean): SearchOutcome => ({ kind: 'results', answer: { code_found: codeFound } as SearchAnswer })
const invalid = (field: 'code' | 'npi'): SearchOutcome => ({ kind: 'invalid', field, message: 'Enter a billing code…' })

describe('wordsToLookUp', () => {
  it('looks up words the server took for no code', () => {
    expect(wordsToLookUp('  office visit ', invalid('code'))).toBe('office visit')
    expect(wordsToLookUp('knee', found(false))).toBe('knee')
    expect(wordsToLookUp('MRI', found(false))).toBe('MRI') // in capitals
  })

  it("doesn't look when the search found its code, or the problem is elsewhere", () => {
    expect(wordsToLookUp('knee', found(true))).toBe('')
    expect(wordsToLookUp('office visit', invalid('npi'))).toBe('')
    expect(wordsToLookUp('office visit', { kind: 'unreachable' })).toBe('')
    expect(wordsToLookUp('office visit', { kind: 'error', message: 'A crash.' })).toBe('')
  })

  it('leaves one word with a digit to its own message: it is meant as a code', () => {
    expect(wordsToLookUp('70450', found(false))).toBe('')
    expect(wordsToLookUp('J9999', found(false))).toBe('')
    expect(wordsToLookUp('99-213!', invalid('code'))).toBe('')
    expect(wordsToLookUp('CPT 70450', found(false))).toBe('') // typed with its type
    expect(wordsToLookUp('hcpcs g0438', found(false))).toBe('')
  })

  it('looks up words with digits, since a code has no spaces', () => {
    expect(wordsToLookUp('covid 19', invalid('code'))).toBe('covid 19')
    expect(wordsToLookUp('PSYTX W PT 60 MINUTES', invalid('code'))).toBe('PSYTX W PT 60 MINUTES') // an example's own label
  })

  it('takes 2 to 100 characters, as the server does, with a letter', () => {
    expect(wordsToLookUp('x', invalid('code'))).toBe('')
    expect(wordsToLookUp('er', invalid('code'))).toBe('er')
    expect(wordsToLookUp('--', invalid('code'))).toBe('')
    expect(wordsToLookUp('99 213', invalid('code'))).toBe('')
    expect(wordsToLookUp('a'.repeat(100), invalid('code'))).toBe('a'.repeat(100))
    expect(wordsToLookUp('a'.repeat(101), invalid('code'))).toBe('')
  })
})

describe('wordsOnly', () => {
  it("takes text that can't be a code as words, without a search", () => {
    expect(wordsOnly('office visit')).toBe('office visit')
    expect(wordsOnly('  flu shot ')).toBe('flu shot')
    expect(wordsOnly('covid 19')).toBe('covid 19') // a code has no spaces
    expect(wordsOnly('x-ray')).toBe('x-ray') // a mark
    expect(wordsOnly('emergency')).toBe('emergency') // more than 7 letters
    expect(wordsOnly('ER')).toBe('ER') // fewer than 3
  })

  it('leaves anything that could be a code to the search', () => {
    for (const typed of ['99213', 'J1100', '0450', 'knee', 'MRI', 'CPT 99213', 'hcpcs a0427', 'RC  450']) {
      expect(wordsOnly(typed)).toBe('')
    }
  })

  it('leaves text that is neither to the server, which says what is wrong with it', () => {
    expect(wordsOnly('99-213!')).toBe('') // no letter
    expect(wordsOnly('covid-19')).toBe('') // one word with a digit: meant as a code
    expect(wordsOnly('CPT 9')).toBe('') // the same, with its type
    expect(wordsOnly('a'.repeat(101))).toBe('') // too long to look up
  })

  it('trims what the server trims, so a pasted code is still searched', () => {
    expect(wordsOnly('CPT 99213\u0085')).toBe('') // U+0085, which Go trims and JavaScript doesn't
    expect(wordsOnly('\u0085HCPCS J1100')).toBe('')
  })
})

describe('afterLookup', () => {
  const search = invalid('code')
  const answer: CodesAnswer = { query: 'office visit', charges: none, services: group(1, [v99213]) }
  const loading: Status = { phase: 'reading' }

  it('has no outcome of its own for words looked up without a search', () => {
    expect(afterLookup(null, { kind: 'codes', answer })).toEqual({ outcome: null, suggestions: answer })
    expect(afterLookup(null, { kind: 'unreachable' })).toEqual({ outcome: { kind: 'unreachable' }, suggestions: null })
  })

  it("shows the search's own outcome when there was no lookup", () => {
    expect(afterLookup(search, null)).toEqual({ outcome: search, suggestions: null })
  })

  it('shows the codes to pick from', () => {
    expect(afterLookup(search, { kind: 'codes', answer })).toEqual({ outcome: search, suggestions: answer })
  })

  it("shows the lookup's problem, such as the data still loading, instead of the code's error", () => {
    expect(afterLookup(search, { kind: 'loading', status: loading })).toEqual({
      outcome: { kind: 'loading', status: loading },
      suggestions: null,
    })
    expect(afterLookup(search, { kind: 'unreachable' })).toEqual({ outcome: { kind: 'unreachable' }, suggestions: null })
  })
})

describe('suggestionsLine', () => {
  const answer = (query: string, charges: CodeGroup, services: CodeGroup): CodesAnswer => ({ query, charges, services })

  it('counts each kind of code', () => {
    const emergency = answer('emergency', group(4, []), group(5, []))
    expect(matchCount(emergency)).toBe(9)
    expect(suggestionsTitle(emergency)).toBe('Codes whose names contain “emergency”')
    expect(suggestionsLine(emergency)).toBe(
      'Codes whose names contain “emergency”: 4 hospital charge codes and 5 service codes. Pick one below.',
    )
    expect(suggestionsLine(answer('office visit', none, group(11, [])))).toBe(
      'Codes whose names contain “office visit”: 11 service codes. Pick one below.',
    )
    const general = answer('general', group(1, []), none) // hospital charges only
    expect(suggestionsTitle(general)).toBe('Codes whose names contain “general”')
    expect(suggestionsLine(general)).toBe('Codes whose names contain “general”: 1 hospital charge code. Pick one below.')
  })

  it('says when no name has the words', () => {
    const kneeReplacement = answer('knee replacement', none, none)
    expect(matchCount(kneeReplacement)).toBe(0)
    expect(suggestionsTitle(kneeReplacement)).toBe("No code's name contains “knee replacement”")
    expect(suggestionsLine(kneeReplacement)).toBe("No code's name contains “knee replacement”.")
  })
})

describe('codeLabel', () => {
  it('names a code the way the results heading does', () => {
    expect(codeLabel(v99213)).toBe('CPT 99213 · OFFICE/OUTPATIENT VISIT EST')
    expect(codeLabel(match('RC 0450', 207, 'Emergency Room General', 'Emergency Room General'))).toBe(
      'RC 0450 · Emergency Room General',
    )
  })

  it("uses the full name when there's no short one", () => {
    // A HCPCS code seen only inside hospital fees has only CMS's name.
    expect(codeLabel(match('HCPCS J0640', 1, '', 'Injection, leucovorin calcium, per 50 mg'))).toBe(
      'HCPCS J0640 · Injection, leucovorin calcium, per 50 mg',
    )
  })
})

describe('showFullName', () => {
  it('shows the full name when the short name lacks some of the words', () => {
    const a0427 = match('HCPCS A0427', 17, 'AMB SVC ALS ER TRNSPRT LVL1', 'Ambulance service, advanced life support…', false)
    expect(showFullName(a0427)).toBe(true)
  })

  it("doesn't when the short name has every word", () => {
    expect(showFullName(v99213)).toBe(false)
  })

  it("doesn't when the label already shows the full name", () => {
    expect(showFullName(match('HCPCS J0640', 1, '', 'Injection, leucovorin calcium, per 50 mg', false))).toBe(false)
  })
})

describe('moreLine', () => {
  const codes = (...names: string[]) => names.map((code, i) => match(code, 100 - i))

  it('says nothing when every code is a button', () => {
    expect(moreLine(group(5, codes('CPT 1', 'CPT 2', 'CPT 3', 'CPT 4', 'CPT 5')))).toBe('')
    expect(moreLine(none)).toBe('')
  })

  it('counts the codes that aren\'t buttons, and says how to reach them', () => {
    const knee = codes('CPT 20611', 'CPT 20610', 'CPT 73562', 'CPT 73564', 'HCPCS A6530', 'CPT 73560', 'CPT 29530', 'HCPCS L1833', 'CPT 29581')
    expect(moreLine(group(9, knee))).toBe('And 4 more: add a word to narrow them down.')
    const office = codes('CPT 99214', 'CPT 99213', 'CPT 99215', 'CPT 99212', 'CPT 99204')
    office.push(...codes('CPT 99211', 'HCPCS G2211', 'CPT 99203', 'CPT 99205', 'CPT 99202'))
    expect(moreLine(group(1046, office))).toBe('And 1,041 more: add a word to narrow them down.')
  })
})

describe('sameNames', () => {
  it('names the codes the file gives the same short and full names', () => {
    expect(sameNames(group(5, [v99214, v99213, v99215, v99212, v99211]))).toBe(
      'The file gives these the same names: CPT 99214, CPT 99213 and CPT 99215.',
    )
  })

  it('separates each set of codes that share names', () => {
    expect(sameNames(group(5, [v99214, v99213, v99215, v99204, v99203]))).toBe(
      'The file gives these the same names: CPT 99214, CPT 99213 and CPT 99215; CPT 99204 and CPT 99203.',
    )
  })

  it('names only codes shown as buttons', () => {
    // 99203 shares 99204's names, but it's the seventh code, past the five buttons.
    expect(sameNames(group(7, [v99214, v99213, v99215, v99212, v99204, v99211, v99203]))).toBe(
      'The file gives these the same names: CPT 99214, CPT 99213 and CPT 99215.',
    )
  })

  it('says nothing when the names tell every code apart', () => {
    expect(sameNames(group(3, [v99213, v99212, v99211]))).toBe('')
  })
})
