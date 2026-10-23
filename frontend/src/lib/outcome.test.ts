import { describe, expect, it } from 'vitest'
import type { Field, SearchAnswer, SearchOutcome } from './api'
import {
  boxMessages,
  canTryAgain,
  clearableBoxes,
  clearLabel,
  describeOutcome,
  describeView,
  isBox,
  isProblem,
  missingCode,
  pageTitle,
  providerNotes,
} from './outcome'
import type { SearchBoxes } from './search'

// answer is a 200 as the server sends it: the code as typed and as shown,
// and each class's fees and organizations.
function answer(typed: string, shown: string | null, professional: [number, number], facility: [number, number]): SearchAnswer {
  return {
    query: { code: typed, npi: '', ein: '', name: '', class: '', place: '', order: '', page: 0, page_size: 0 },
    code: shown === null ? null : { type: 'CPT', display: shown, short_name: '', name: '' },
    code_found: shown !== null,
    provider_found: true,
    not_found: [],
    provider_organizations: { npi: [], ein: [] },
    classes: [
      { billing_class: 'professional', count: professional[0], organization_count: professional[1] },
      { billing_class: 'institutional', count: facility[0], organization_count: facility[1] },
    ],
    class: professional[0] > 0 || facility[0] === 0 ? 'professional' : 'institutional',
    total: professional[0] > 0 || facility[0] === 0 ? professional[0] : facility[0],
    organization_count: professional[0] > 0 || facility[0] === 0 ? professional[1] : facility[1],
    page: 1,
    pages: 1,
    page_size: 50,
    rows: [],
    modifiers: [],
    summary: null,
  }
}

const results = (a: SearchAnswer): SearchOutcome => ({ kind: 'results', answer: a })

// narrowed is a search with provider boxes that found no rows. As the server
// does, it says no price matches the boxes together (provider_found false)
// when a box matches nothing in the file.
function narrowed(code: string, change: Partial<SearchAnswer> = {}, boxes: Partial<SearchAnswer['query']> = {}): SearchAnswer {
  const a = answer(code, code, [0, 0], [0, 0])
  return { ...a, query: { ...a.query, ...boxes }, provider_found: !change.not_found?.length, ...change }
}

// Names from the file: NPI 1780644302 is listed under two organizations, EIN
// 11-2419534 under one, and the two never share a price.
const npiNames = ['ENT & Allergy Associates, LLP', 'Icahn School of Medicine at Mount Sinai']
const einNames = ['North Shore Hematology/Oncology Associates, PC']

describe('describeOutcome', () => {
  it('counts fees, not organizations, under the code as shown', () => {
    expect(describeOutcome(results(answer('99213', '99213', [1108, 957], [102, 84])))).toBe(
      '99213: 1,108 professional fees and 102 facility fees.',
    )
    expect(describeOutcome(results(answer('450', '0450', [0, 0], [207, 119])))).toBe(
      '0450: 0 professional fees and 207 facility fees.',
    )
    expect(describeOutcome(results(answer('27447', '27447', [0, 0], [1, 1])))).toBe(
      '27447: 0 professional fees and 1 facility fee.',
    )
  })

  it('names the provider a search narrowed to', () => {
    const fees = answer('99213', '99213', [4, 2], [0, 0])
    const npi = { ...fees, query: { ...fees.query, npi: '1780644302' } }
    expect(describeOutcome(results(npi))).toBe('99213 for NPI 1780644302: 4 professional fees and 0 facility fees.')
    const all = { ...fees, query: { ...fees.query, npi: '1780644302', ein: '13-4000192', name: 'ent' } }
    expect(describeOutcome(results(all))).toBe(
      '99213 for NPI 1780644302, EIN 13-4000192 and organization name “ent”: 4 professional fees and 0 facility fees.',
    )
  })

  it('says a code is missing from the file, wherever a price could list it', () => {
    expect(describeOutcome(results(answer('70450', null, [0, 0], [0, 0])))).toBe(
      "70450 isn't in this file: no price lists it, as its own code or as a facility fee's procedure. Try another code, or words from its name, such as office visit.",
    )
  })

  it('names the boxes the file has nothing for', () => {
    const npi = narrowed('99213', { not_found: ['npi'] }, { npi: '1234567893' })
    expect(describeOutcome(results(npi))).toBe('No 99213 price: the file has nothing for the NPI you gave.')
    const all = narrowed('99213', { not_found: ['npi', 'ein', 'name'] }, { npi: '1234567893', ein: '00-0000000', name: 'zzz' })
    expect(describeOutcome(results(all))).toBe(
      'No 99213 price: the file has nothing for the NPI, EIN and organization name you gave.',
    )
  })

  it('tells boxes that never share a price from ones with no price for this code', () => {
    const apart = narrowed('99214', { provider_found: false }, { npi: '1780644302', ein: '11-2419534' })
    expect(describeOutcome(results(apart))).toBe('No price in this file has this NPI and EIN together.')
    const elsewhere = narrowed('0450', {}, { npi: '1780644302' })
    expect(describeOutcome(results(elsewhere))).toBe('No 0450 price matches this NPI.')
  })

  it("claims only what the search matched, since an NPI search skips whole-practice prices", () => {
    // In the file, EIN 16-0743964 has a whole-practice price for 99213
    // ($51.45, no NPIs listed), and NPI 1033520853 is listed under that EIN.
    const practice = narrowed('99213', {}, { npi: '1033520853', ein: '16-0743964' })
    expect(describeOutcome(results(practice))).toBe('No 99213 price matches this NPI and EIN together.')
  })

  it('leaves a message about a box to that box, and says one about the address', () => {
    expect(describeOutcome({ kind: 'invalid', field: 'code', message: 'Enter a billing code.' })).toBe('')
    expect(describeOutcome({ kind: 'invalid', field: '', message: 'The search address is malformed.' })).toBe(
      'The search address is malformed.',
    )
  })

  it('says what is happening while the data loads, or when it failed', () => {
    expect(describeOutcome({ kind: 'loading', status: { phase: 'downloading' } })).toBe(
      "Still loading the data. Your search will run as soon as it's ready.",
    )
    expect(describeOutcome({ kind: 'failed', status: { phase: 'failed', error: 'no such host' } })).toBe(
      "Searches can't run until the data loads. The status line above says what went wrong.",
    )
  })

  it("says when a search didn't get through, and passes on the server's message", () => {
    expect(describeOutcome({ kind: 'unreachable' })).toBe("The search didn't reach the server.")
    expect(describeOutcome({ kind: 'error', message: 'Something went wrong on the server. See its log.' })).toBe(
      'Something went wrong on the server. See its log.',
    )
  })
})

describe('isBox', () => {
  it.each<[Field, boolean]>([
    ['code', true],
    ['npi', true],
    ['ein', true],
    ['name', true],
    ['class', false],
    ['page', false],
    ['page_size', false],
    ['', false],
  ])('%s is a box: %s', (field, box) => {
    expect(isBox(field)).toBe(box)
  })
})

describe('missingCode', () => {
  const boxes = (typed: Partial<SearchBoxes>): SearchBoxes => ({ code: '', npi: '', ein: '', name: '', ...typed })

  it('asks for a code, saying why when a provider is typed', () => {
    expect(missingCode(boxes({}))).toBe('Enter a billing code, such as 99213, or words from its name, such as office visit.')
    const provider = "Enter a billing code too, such as 99213: a provider's rates are shown one code at a time."
    expect(missingCode(boxes({ ein: '13-4000192' }))).toBe(provider)
    expect(missingCode(boxes({ npi: '1780644302', code: '   ' }))).toBe(provider)
    expect(missingCode(boxes({ name: 'sinai' }))).toBe(provider)
  })

  it('says nothing when there is a code', () => {
    expect(missingCode(boxes({ code: '99213', ein: '13-4000192' }))).toBe('')
  })
})

describe('pageTitle', () => {
  it('names the search on screen, with its provider', () => {
    const fees = answer('99213', '99213', [4, 2], [0, 0])
    expect(pageTitle(results(fees), null)).toBe('99213 · Price Transparency Explorer')
    expect(pageTitle(results({ ...fees, query: { ...fees.query, npi: '1780644302' } }), null)).toBe(
      '99213 for NPI 1780644302 · Price Transparency Explorer',
    )
    expect(pageTitle(results(answer('450', '0450', [0, 0], [207, 119])), null)).toBe('0450 · Price Transparency Explorer')
  })

  it('names the words looked up, even beside the search that led to them', () => {
    const codes = (query: string) => ({ query, charges: { total: 0, codes: [] }, services: { total: 0, codes: [] } })
    expect(pageTitle(null, codes('office visit'))).toBe('“office visit” · Price Transparency Explorer')
    // "knee" is searched first: the server answers KNEE isn't a code in the file, then the page shows codes named knee.
    expect(pageTitle(results(answer('KNEE', null, [0, 0], [0, 0])), codes('knee'))).toBe('“knee” · Price Transparency Explorer')
  })

  it("is the app's name before a search", () => {
    expect(pageTitle(null, null)).toBe('Price Transparency Explorer')
  })
})

describe('isProblem and canTryAgain', () => {
  const each: SearchOutcome[] = [
    results(answer('99213', '99213', [1108, 957], [102, 84])),
    { kind: 'loading', status: { phase: 'reading' } },
    { kind: 'invalid', field: 'npi', message: 'An NPI has 10 digits.' },
    { kind: 'failed', status: { phase: 'failed', error: 'no such host' } },
    { kind: 'unreachable' },
    { kind: 'error', message: 'Something went wrong on the server. See its log.' },
  ]

  it('shows only things to fix as problems: not results, and not waiting for the data', () => {
    expect(each.map(isProblem)).toEqual([false, false, true, true, true, true])
  })

  it('offers Try again only when the same search might work the next time', () => {
    // A failed load can work once the server is restarted.
    expect(each.map(canTryAgain)).toEqual([false, false, false, true, true, true])
  })
})

describe('boxMessages', () => {
  it("puts the server's message about bad input under that box", () => {
    expect(boxMessages({ kind: 'invalid', field: 'npi', message: 'An NPI has 10 digits.' })).toEqual({
      npi: { text: 'An NPI has 10 digits.', invalid: true },
    })
  })

  it("puts nothing under the boxes for a message that isn't about one", () => {
    expect(boxMessages({ kind: 'invalid', field: 'page', message: 'page is a whole number, 1 or more.' })).toEqual({})
    expect(boxMessages(null)).toEqual({})
    expect(boxMessages({ kind: 'unreachable' })).toEqual({})
  })

  it('notes each box the file has nothing for, without calling it bad input', () => {
    const none = narrowed('99213', { not_found: ['npi', 'name'] }, { npi: '1234567893', name: 'zzz' })
    expect(boxMessages(results(none))).toEqual({
      npi: { text: 'No provider group in this file lists this NPI.', invalid: false },
      name: { text: "No organization's name in this file has a word starting with this.", invalid: false },
    })
  })

  it('says no name has all the words, when the name box has several', () => {
    // "unity" and "buffalo" each start a word of some name in the file, but no one name has both.
    const both = narrowed('0450', { not_found: ['name'] }, { name: 'unity buffalo' })
    expect(boxMessages(results(both))).toEqual({
      name: { text: "No organization's name in this file has all of these words.", invalid: false },
    })
  })
})

describe('providerNotes', () => {
  const organizations = { npi: npiNames, ein: einNames }

  it('says where the file lists the NPI and EIN, when no price matched', () => {
    const apart = narrowed('99214', { provider_found: false, provider_organizations: organizations }, { npi: '1780644302', ein: '11-2419534' })
    expect(providerNotes(apart)).toEqual([
      { lead: 'NPI 1780644302 is listed under:', names: npiNames },
      { lead: 'EIN 11-2419534 is listed as:', names: einNames },
    ])
  })

  it('notes the EIN even when the file has nothing for the NPI', () => {
    const a = narrowed('99213', { not_found: ['npi'], provider_organizations: { npi: [], ein: einNames } }, { npi: '1234567893', ein: '11-2419534' })
    expect(providerNotes(a)).toEqual([{ lead: 'EIN 11-2419534 is listed as:', names: einNames }])
  })

  it('lists names as the file has them, commas and periods included', () => {
    const a = narrowed('27447', { provider_organizations: { npi: [], ein: ['Council on Addiction Recovery Services, Inc.'] } }, { ein: '16-1160187' })
    expect(providerNotes(a)).toEqual([
      { lead: 'EIN 16-1160187 is listed as:', names: ['Council on Addiction Recovery Services, Inc.'] },
    ])
  })

  it('lists three organizations, then says how many more', () => {
    // NPI 1881987113 is listed under four organizations in the file.
    const four = [
      'Main Street Radiology at Bayside, LLC',
      'North Shore LIJ Medical PC',
      'Radiology Association of Main Street, LLC',
      'Southside Faculty Medical Affiliates University Faculty',
    ]
    const a = narrowed('0450', { provider_organizations: { npi: four, ein: [] } }, { npi: '1881987113' })
    expect(providerNotes(a)).toEqual([
      { lead: 'NPI 1881987113 is listed under:', names: [...four.slice(0, 3), 'and 1 more'] },
    ])
    const three = narrowed('0450', { provider_organizations: { npi: four.slice(0, 3), ein: [] } }, { npi: '1881987113' })
    expect(providerNotes(three)[0].names).toEqual(four.slice(0, 3))
  })

  it('says nothing when prices were found, the code is missing, or the file lists no one', () => {
    const found = { ...answer('99213', '99213', [3, 2], [0, 0]), provider_organizations: organizations }
    found.query = { ...found.query, npi: '1780644302' }
    expect(providerNotes(found)).toEqual([])
    const missing = { ...answer('70450', null, [0, 0], [0, 0]), provider_organizations: organizations }
    missing.query = { ...missing.query, npi: '1780644302' }
    expect(providerNotes(missing)).toEqual([])
    expect(providerNotes(narrowed('99213', { not_found: ['npi'] }, { npi: '1234567893' }))).toEqual([])
    expect(providerNotes(narrowed('99213', { not_found: ['ein'] }, { ein: '00-0000000' }))).toEqual([])
  })
})

describe('describeView', () => {
  it('says which kind of fee and page are on screen', () => {
    const page2 = { ...answer('99213', '99213', [1108, 957], [102, 84]), class: 'institutional' as const, page: 2, pages: 3 }
    page2.rows = [{} as SearchAnswer['rows'][number]]
    expect(describeView(page2)).toBe('Showing facility fees, page 2 of 3.')
    expect(describeView({ ...page2, class: 'professional', page: 1, pages: 1 })).toBe('Showing professional fees.')
  })

  it('says nothing when no rows are shown', () => {
    expect(describeView(narrowed('99213', {}, { npi: '1780644302' }))).toBe('')
  })

  it('names the place shown, or says none of the fees are there', () => {
    const office = { ...answer('99213', '99213', [1108, 957], [102, 84]), pages: 7, total: 309 }
    office.query = { ...office.query, place: '11' }
    office.rows = [{} as SearchAnswer['rows'][number]]
    expect(describeView(office)).toBe('Showing professional fees at 11 Office, page 1 of 7.')
    expect(describeView({ ...office, pages: 1 })).toBe('Showing professional fees at 11 Office.')
    expect(describeView({ ...office, query: { ...office.query, order: 'highest' } })).toBe(
      'Showing professional fees at 11 Office, highest price first, page 1 of 7.',
    )
    expect(describeView({ ...office, pages: 1, query: { ...office.query, order: 'highest' } })).toBe(
      'Showing professional fees at 11 Office, highest price first.',
    )
    expect(describeView({ ...office, rows: [], total: 0, pages: 1 })).toBe('None of these fees are at 11 Office.')
  })
})

describe('clearableBoxes and clearLabel', () => {
  it('offers to clear each provider box the search used, when it found no rows', () => {
    const a = narrowed('99213', { provider_found: false }, { npi: '1780644302', name: 'sinai' })
    expect(clearableBoxes(a)).toEqual(['npi', 'name'])
    expect(clearableBoxes(a).map(clearLabel)).toEqual(['Clear the NPI', 'Clear the organization name'])
    expect(clearLabel('ein')).toBe('Clear the EIN')
  })

  it('offers nothing when prices were found or the code is missing', () => {
    const found = answer('99213', '99213', [3, 2], [0, 0])
    found.query = { ...found.query, npi: '1780644302' }
    expect(clearableBoxes(found)).toEqual([])
    const missing = answer('70450', null, [0, 0], [0, 0])
    missing.query = { ...missing.query, npi: '1780644302' }
    expect(clearableBoxes(missing)).toEqual([])
  })
})
