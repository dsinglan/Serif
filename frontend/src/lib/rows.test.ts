import { describe, expect, it } from 'vitest'
import type { Row, SearchAnswer } from './api'
import { placeLabel } from './places'
import {
  caption,
  cutNote,
  facilityColumns,
  feeButton,
  heading,
  legend,
  modifierMeaning,
  money,
  nameOrNote,
  noFeesShown,
  placeNote,
  procedureName,
  rowKey,
  versusMiddle,
  versusTally,
  whoIsCovered,
} from './rows'

// row is modeled on East Point Medical PC's $41.31 telehealth price for
// 99213, with any field changed.
function row(change: Partial<Row> = {}): Row {
  return {
    billing_code_type: 'CPT',
    billing_code: '99213',
    display_code: '99213',
    label: 'OFFICE OR OTH OUTPATIENT VISIT',
    negotiation_arrangement: 'ffs',
    negotiated_type: 'negotiated',
    negotiated_rate: 41.31,
    billing_class: 'professional',
    setting: 'outpatient',
    service_code: ['02'],
    billing_code_modifier: ['95'],
    expiration_date: '9999-12-31',
    tin_type: 'ein',
    tin: '85-2329231',
    business_name: 'East Point Medical PC',
    npis: Array.from({ length: 19 }, (_, i) => String(1265503726 + i)),
    invalid_npis: [],
    tin_level: false,
    flags: [],
    source: {
      in_network_index: 1417,
      negotiated_rates_index: 44,
      negotiated_prices_index: 0,
      provider_group_id: '1115',
      provider_groups_index: 0,
      listed_times: 1,
    },
    ...change,
  }
}

describe('money', () => {
  it('shows dollars and cents', () => {
    expect(money(41.31)).toBe('$41.31')
    expect(money(10955.28)).toBe('$10,955.28')
  })
})

describe('whoIsCovered', () => {
  it("counts a group's NPIs, or says the whole practice", () => {
    expect(whoIsCovered(row())).toBe('19 NPIs')
    expect(whoIsCovered(row({ npis: ['1780644302'] }))).toBe('1 NPI')
    expect(whoIsCovered(row({ npis: [], tin_level: true }))).toBe('Whole practice')
  })

  it('says how many groups a merged row stands for, not one group’s size', () => {
    expect(whoIsCovered(row({ groups: 2 }))).toBe('In 2 groups')
    expect(whoIsCovered(row({ groups: 1 }))).toBe('19 NPIs')
  })
})

describe('procedureName', () => {
  it("uses the file's short name, else the full name, else says there's none", () => {
    const er = { procedure_code: '99285', procedure_label: 'EMERGENCY DEPARTMENT VISIT FOR THE EVALUATION…', procedure_short_name: 'EMERGENCY DEPT VISIT' }
    expect(procedureName(row(er))).toBe('EMERGENCY DEPT VISIT')
    expect(procedureName(row({ ...er, procedure_short_name: '' }))).toBe('EMERGENCY DEPARTMENT VISIT FOR THE EVALUATION…')
    expect(procedureName(row({ procedure_code: '27447' }))).toBe('Not described in this file')
  })
})

describe('facilityColumns', () => {
  const searched = (type: string) => ({ code: { type } }) as SearchAnswer
  it('shows the procedure for a revenue code, and what a row is billed as for a procedure', () => {
    expect(facilityColumns(searched('RC'))).toEqual({ billedAs: false, procedure: true })
    expect(facilityColumns(searched('CPT'))).toEqual({ billedAs: true, procedure: false })
    expect(facilityColumns(searched('HCPCS'))).toEqual({ billedAs: true, procedure: false })
  })
})

describe('nameOrNote', () => {
  it('gives the name, or says why there is none', () => {
    expect(nameOrNote('Emergency Room General', true)).toBe('Emergency Room General')
    expect(nameOrNote('', true)).toBe('No standard name in this file')
    expect(nameOrNote('', false)).toBe('Not described in this file')
  })
})

describe('heading', () => {
  it('shows the short name in the heading and the full name under it', () => {
    const code = { type: 'CPT', display: '99213', short_name: 'OFFICE/OUTPATIENT VISIT EST', name: 'OFFICE OR OTH OUTPATIENT VISIT…' }
    expect(heading(code)).toEqual({
      title: 'CPT 99213 · OFFICE/OUTPATIENT VISIT EST',
      subtitle: 'OFFICE OR OTH OUTPATIENT VISIT…',
      note: '',
    })
  })

  it('shows a name once when both say the same', () => {
    const code = { type: 'RC', display: '0450', short_name: 'Emergency Room General', name: 'Emergency Room General' }
    expect(heading(code)).toEqual({ title: 'RC 0450 · Emergency Room General', subtitle: '', note: '' })
  })

  it('says so when nothing names the code, in words that fit the kind of code', () => {
    expect(heading({ type: 'CPT', display: '27447', short_name: '', name: '' }).subtitle).toBe('Not described in this file')
    expect(heading({ type: 'RC', display: '0821', short_name: '', name: '' }).subtitle).toBe('No standard name in this file')
  })

  it("notes CMS's last date for a discontinued code", () => {
    const code = { type: 'HCPCS', display: 'J0640', short_name: '', name: 'Injection, leucovorin calcium, per 50 mg', discontinued: '2026-09-30' }
    expect(heading(code)).toEqual({
      title: 'HCPCS J0640',
      subtitle: 'Injection, leucovorin calcium, per 50 mg',
      note: 'Discontinued by CMS: last usable Sep 30, 2026',
    })
  })
})

describe('feeButton', () => {
  it('names the kind of fee and how many there are', () => {
    expect(feeButton({ billing_class: 'professional', count: 1108, organization_count: 957 })).toBe('Professional fees (1,108)')
    expect(feeButton({ billing_class: 'institutional', count: 0, organization_count: 0 })).toBe('Facility fees (0)')
  })
})

describe('caption', () => {
  // answer99213 is 99213's answer showing one class: 1,108 professional fees
  // from 957 organizations, and 102 facility fees from 84.
  function answer99213(shown: 'professional' | 'institutional', page: number, pages: number): SearchAnswer {
    return {
      query: { code: '99213', npi: '', ein: '', name: '', class: shown, place: '', order: '', page, page_size: 0 },
      code: null,
      code_found: true,
      provider_found: true,
      not_found: [],
      provider_organizations: { npi: [], ein: [] },
      classes: [
        { billing_class: 'professional', count: 1108, organization_count: 957 },
        { billing_class: 'institutional', count: 102, organization_count: 84 },
      ],
      class: shown,
      total: shown === 'professional' ? 1108 : 102,
      organization_count: shown === 'professional' ? 957 : 84,
      page,
      pages,
      page_size: 50,
      rows: [],
      modifiers: [],
      summary: null,
    }
  }

  it('counts only the class shown: its fees, its organizations, and the page', () => {
    expect(caption(answer99213('professional', 1, 23))).toBe(
      'Professional fees, lowest price first: 1,108 fees from 957 organizations. Page 1 of 23.',
    )
    expect(caption(answer99213('institutional', 2, 3))).toBe(
      'Facility fees, lowest price first: 102 fees from 84 organizations. Page 2 of 3.',
    )
  })

  it('names the place, and counts the fees and organizations there', () => {
    const office = { ...answer99213('professional', 1, 7), total: 309, organization_count: 297 }
    office.query = { ...office.query, place: '11' }
    expect(caption(office)).toBe('Professional fees at 11 Office, lowest price first: 309 fees from 297 organizations. Page 1 of 7.')
  })

  it('says when the highest price is first', () => {
    const highest = answer99213('professional', 1, 23)
    highest.query = { ...highest.query, order: 'highest' }
    expect(caption(highest)).toBe('Professional fees, highest price first: 1,108 fees from 957 organizations. Page 1 of 23.')
  })

  it('says what the line under a price is, when the page compares fees', () => {
    const office = { code: '11', name: '', count: 309, lowest: 50.4, middle: 113.59, highest: 159.15 }
    const cpt = { type: 'CPT', display: '99213', short_name: '', name: '' }
    const answer = (code: object, summary: object) => ({ ...answer99213('professional', 1, 23), code, summary }) as unknown as SearchAnswer
    expect(caption(answer(cpt, { places: [office], procedures: [], mixed_pricing: false }))).toBe(
      'Professional fees, lowest price first: 1,108 fees from 957 organizations. Page 1 of 23. The line under a price compares it with the middle price at its place.',
    )
    // Not for a revenue code, prices that mix pricing modifiers, or a summary with no places.
    const plain = 'Professional fees, lowest price first: 1,108 fees from 957 organizations. Page 1 of 23.'
    expect(caption(answer({ ...cpt, type: 'RC' }, { places: [office], procedures: [], mixed_pricing: false }))).toBe(plain)
    expect(caption(answer(cpt, { places: [office], procedures: [], mixed_pricing: true }))).toBe(plain)
    expect(caption(answer(cpt, { places: [], procedures: [], mixed_pricing: false }))).toBe(plain)
  })
})

describe('versusMiddle', () => {
  // market99213 is 99213's doctor-fee answer, with three of its real place
  // lines: telehealth outside the home (02), the office (11), and an
  // off-campus hospital clinic (19). place is the place picked, if any, and
  // change changes the answer.
  function market99213(place = '', change: object = {}): SearchAnswer {
    const line = (code: string, middle: number) => ({ code, name: '', count: 5, lowest: middle, middle, highest: middle })
    return {
      query: { code: '99213', npi: '', ein: '', name: '', class: 'professional', place, order: '', page: 0, page_size: 0 },
      code: { type: 'CPT', display: '99213', short_name: '', name: '' },
      summary: { places: [line('02', 51.45), line('11', 113.59), line('19', 71.38)], procedures: [], mixed_pricing: false },
      ...change,
    } as unknown as SearchAnswer
  }

  it('compares a fee with the middle at its place', () => {
    // NPI 1780644302's prices: ENT & Allergy in an office, Mount Sinai at an off-campus clinic.
    expect(versusMiddle(row({ negotiated_rate: 136.68, service_code: ['11'] }), market99213())).toBe('20% above the $113.59 middle')
    expect(versusMiddle(row({ negotiated_rate: 70.16, service_code: ['19'] }), market99213())).toBe('2% below the $71.38 middle')
  })

  it('rounds to the nearest percent', () => {
    // $137.39 is 20.95% above the office middle.
    expect(versusMiddle(row({ negotiated_rate: 137.39, service_code: ['11'] }), market99213())).toBe('21% above the $113.59 middle')
  })

  it('calls a fee within half a percent of the middle at it', () => {
    expect(versusMiddle(row({ negotiated_rate: 51.45, service_code: ['02'] }), market99213())).toBe('At the $51.45 middle')
    expect(versusMiddle(row({ negotiated_rate: 51.6, service_code: ['02'] }), market99213())).toBe('At the $51.45 middle')
  })

  it('compares a fee posted for several places only once a place is picked, then names no price', () => {
    const both = row({ negotiated_rate: 86.71, service_code: ['10', '11'] })
    expect(versusMiddle(both, market99213())).toBe('Posted for several places')
    expect(versusMiddle(both, market99213('11'))).toBe('24% below the middle')
    expect(versusMiddle(row({ negotiated_rate: 113.59, service_code: ['11'] }), market99213('11'))).toBe('At the middle')
  })

  it("can't compare at a place with no line in the summary", () => {
    expect(versusMiddle(row({ service_code: ['24'] }), market99213())).toBe('Too few to compare')
  })

  it("doesn't compare a flagged fee, which the summary leaves out", () => {
    expect(versusMiddle(row({ negotiated_rate: 0.5, service_code: ['11'], flags: ['one_dollar_or_less'] }), market99213())).toBe('')
    expect(versusMiddle(row({ service_code: ['11'], flags: ['placeholder_provider'] }), market99213())).toBe('')
    // Several prices for one NPI still count in the summary, so they're compared.
    const several = row({ negotiated_rate: 137.39, service_code: ['11'], flags: ['several_prices'] })
    expect(versusMiddle(several, market99213())).toBe('21% above the $113.59 middle')
  })

  it('compares nothing for a revenue code, prices that mix pricing modifiers, or no summary', () => {
    const office = row({ service_code: ['11'] })
    const revenueCode = market99213('', { code: { type: 'RC', display: '0300', short_name: '', name: '' } })
    expect(versusMiddle(office, revenueCode)).toBe('')
    const mixed = market99213()
    mixed.summary = { ...mixed.summary!, mixed_pricing: true }
    expect(versusMiddle(office, mixed)).toBe('')
    expect(versusMiddle(office, market99213('', { summary: null }))).toBe('')
  })
})

describe('versusTally', () => {
  // npi1780644302 is NPI 1780644302's answer for 99213: its 4 real fees,
  // and three of 99213's real place lines. change changes the answer.
  function npi1780644302(change: object = {}): SearchAnswer {
    const line = (code: string, middle: number) => ({ code, name: '', count: 5, lowest: middle, middle, highest: middle })
    return {
      query: { code: '99213', npi: '1780644302', ein: '', name: '', class: 'professional', place: '', order: '', page: 0, page_size: 0 },
      code: { type: 'CPT', display: '99213', short_name: '', name: '' },
      summary: { places: [line('02', 51.45), line('11', 113.59), line('19', 71.38)], procedures: [], mixed_pricing: false },
      pages: 1,
      rows: [
        row({ negotiated_rate: 70.16, service_code: ['19'] }),
        row({ negotiated_rate: 136.68, service_code: ['11'] }),
        row({ negotiated_rate: 137.39, service_code: ['11'], flags: ['several_prices'] }),
        row({ negotiated_rate: 139.39, service_code: ['11'], flags: ['several_prices'] }),
      ],
      ...change,
    } as unknown as SearchAnswer
  }

  it("adds up the lines under a provider's fees", () => {
    expect(versusTally(npi1780644302())).toBe('Compared with the middle price at their place: 3 above, 1 below.')
  })

  it('counts fees at the middle, and the ones not compared', () => {
    const rows = [row({ negotiated_rate: 51.45, service_code: ['02'] }), row({ service_code: ['10', '11'] }), row({ service_code: ['24'] })]
    expect(versusTally(npi1780644302({ rows }))).toBe('Compared with the middle price at their place: 1 at the middle; 2 not compared.')
  })

  it('adds up to every fee on the page, a flagged one counted as not compared', () => {
    const rows = [
      row({ negotiated_rate: 130, service_code: ['11'] }),
      row({ negotiated_rate: 140, service_code: ['11'] }),
      row({ negotiated_rate: 113.59, service_code: ['11'] }),
      row({ negotiated_rate: 40, service_code: ['02'] }),
      row({ negotiated_rate: 60, service_code: ['24'] }),
      row({ negotiated_rate: 0.5, service_code: ['11'], flags: ['one_dollar_or_less'] }),
    ]
    expect(versusTally(npi1780644302({ rows }))).toBe(
      'Compared with the middle price at their place: 2 above, 1 at the middle, 1 below; 2 not compared.',
    )
  })

  it('says when it adds up only this page', () => {
    expect(versusTally(npi1780644302({ pages: 2 }))).toBe('On this page, compared with the middle price at their place: 3 above, 1 below.')
  })

  it('says nothing for a code alone, or when no fee on the page is compared', () => {
    const market = npi1780644302()
    market.query = { ...market.query, npi: '' }
    expect(versusTally(market)).toBe('')
    expect(versusTally(npi1780644302({ summary: null }))).toBe('')
    expect(versusTally(npi1780644302({ rows: [row({ service_code: ['10', '11'] })] }))).toBe('')
    // A flagged fee isn't compared.
    expect(versusTally(npi1780644302({ rows: [row({ service_code: ['11'], flags: ['placeholder_provider'] })] }))).toBe('')
  })
})

describe('placeNote', () => {
  const at = (place: string, total: number): SearchAnswer => {
    const a = {
      query: { code: '99213', npi: '', ein: '', name: '', class: 'professional', place, order: '', page: 0, page_size: 0 },
      classes: [
        { billing_class: 'professional', count: 1108, organization_count: 957 },
        { billing_class: 'institutional', count: 102, organization_count: 84 },
      ],
      class: 'professional',
      total,
    }
    return a as SearchAnswer
  }

  it('says which place the table shows, out of every place', () => {
    expect(placeNote(at('11', 309))).toBe('Only fees at 11 Office: 309 fees of 1,108.')
  })

  it('names the middle price there, when the page compares fees', () => {
    const office = { code: '11', name: '', count: 309, lowest: 50.4, middle: 113.59, highest: 159.15 }
    const answer = {
      ...at('11', 309),
      code: { type: 'CPT', display: '99213', short_name: '', name: '' },
      summary: { places: [office], procedures: [], mixed_pricing: false },
    } as unknown as SearchAnswer
    expect(placeNote(answer)).toBe('Only fees at 11 Office: 309 fees of 1,108. The middle price there is $113.59.')
    // Prices that mix pricing modifiers make no middle worth naming.
    const mixed = { ...answer, summary: { places: [office], procedures: [], mixed_pricing: true } } as unknown as SearchAnswer
    expect(placeNote(mixed)).toBe('Only fees at 11 Office: 309 fees of 1,108.')
  })

  it('says when none of the fees are at the place', () => {
    expect(placeNote(at('23', 0))).toBe('None of these fees are at 23 Emergency Room – Hospital.')
  })

  it('says nothing for every place', () => {
    expect(placeNote(at('', 1108))).toBe('')
  })
})

describe('noFeesShown', () => {
  // j1100 is J1100's answer when an address asks for its facility fees: it
  // has 50 doctor fees and none for facilities.
  function j1100(rows: Row[]): SearchAnswer {
    return {
      query: { code: 'J1100', npi: '', ein: '', name: '', class: 'institutional', place: '', order: '', page: 0, page_size: 0 },
      code: { type: 'HCPCS', display: 'J1100', short_name: 'INJ DEXMETH SODM PHOSHA 1MG', name: '' },
      code_found: true,
      provider_found: true,
      not_found: [],
      provider_organizations: { npi: [], ein: [] },
      classes: [
        { billing_class: 'professional', count: 50, organization_count: 40 },
        { billing_class: 'institutional', count: 0, organization_count: 0 },
      ],
      class: 'institutional',
      total: 0,
      organization_count: 0,
      page: 1,
      pages: 1,
      page_size: 50,
      rows,
      modifiers: [],
      summary: null,
    }
  }

  it('says the kind of fee shown has none, when the other kind has some', () => {
    expect(noFeesShown(j1100([]))).toBe('No facility fees for J1100.')
  })

  it('says nothing when the kind shown has fees, or when neither kind has any', () => {
    const some = j1100([row()])
    some.classes = some.classes.map((c) => ({ ...c, count: c.billing_class === 'institutional' ? 1 : c.count }))
    expect(noFeesShown(some)).toBe('')
    // A place can leave no rows on the page while the kind shown has fees.
    const elsewhere = { ...some, rows: [], total: 0, query: { ...some.query, place: '23' } }
    expect(noFeesShown(elsewhere)).toBe('')
    const none = j1100([])
    none.classes = none.classes.map((c) => ({ ...c, count: 0 }))
    expect(noFeesShown(none)).toBe('')
  })
})

describe('rowKey', () => {
  it('tells rows apart by every part of where they come from', () => {
    const first = row()
    const s = first.source
    const others = [
      row({ source: { ...s, in_network_index: 1418 } }),
      row({ source: { ...s, negotiated_rates_index: 45 } }),
      row({ source: { ...s, negotiated_prices_index: 1 } }),
      row({ source: { ...s, provider_group_id: '1227' } }),
      row({ source: { ...s, provider_groups_index: 1 } }),
    ]
    for (const other of others) {
      expect(rowKey(other)).not.toBe(rowKey(first))
    }
  })
})

describe('legend', () => {
  const modifiers = [
    { code: '25', licensed: true },
    { code: '95', licensed: true },
    { code: 'GT', meaning: 'Via interactive audio and video telecommunication systems', source: 'CMS' },
    { code: 'RH' },
  ]

  it("gives a modifier's CMS meaning for a hover, or nothing for a CPT modifier and one CMS doesn't list", () => {
    expect(modifierMeaning('GT', modifiers)).toBe('Via interactive audio and video telecommunication systems (CMS)')
    expect(modifierMeaning('25', modifiers)).toBe('')
    expect(modifierMeaning('RH', modifiers)).toBe('')
  })

  it("gives CMS's meanings, and one line each for CPT modifiers and the rest", () => {
    expect(legend(modifiers, true)).toEqual([
      { codes: 'GT', meaning: 'Via interactive audio and video telecommunication systems (CMS)' },
      { codes: '25, 95', meaning: 'CPT modifiers: the AMA licenses their text, so only the code is shown' },
      { codes: 'RH', meaning: "Not in CMS's HCPCS file" },
    ])
  })

  it("doesn't claim a modifier is missing from CMS's file when that file wasn't read", () => {
    const unread = [{ code: '25', licensed: true }, { code: 'GT' }, { code: 'LT' }]
    expect(legend(unread, false)).toEqual([
      { codes: '25', meaning: 'CPT modifiers: the AMA licenses their text, so only the code is shown' },
      { codes: 'GT, LT', meaning: "No meaning shown: CMS's HCPCS file couldn't be loaded" },
    ])
  })
})

describe('placeLabel', () => {
  it("gives the code and CMS's name, and an unknown code alone", () => {
    expect(placeLabel('02')).toBe('02 Telehealth Provided Other than in Patient’s Home')
    expect(placeLabel('24')).toBe('24 Ambulatory Surgical Center')
    expect(placeLabel('07')).toBe('07')
  })
})

describe('cutNote', () => {
  it('explains the mark when a name shown ends with it', () => {
    const cut99213 = 'OFFICE OR OTH OUTPATIENT VISIT FOR THE EVALUATION AND MANAGEMENT OF AN ESTABLISHED PATIENT, WHICH RE…'
    expect(cutNote(['Emergency Room General', cut99213])).toBe(
      '… marks a name the rate file may have cut short: it stops names at 100 characters.',
    )
  })

  it('says nothing when no name has it', () => {
    expect(cutNote(['Emergency Room General', ''])).toBe('')
    expect(cutNote([])).toBe('')
  })
})
