import { describe, expect, it } from 'vitest'
import type { SearchAnswer, Summary, SummaryLine } from './api'
import { placeLabel } from './places'
import {
  bar,
  canPickPlaces,
  lineNumbers,
  marketFirst,
  modeShare,
  noSummary,
  notes,
  procedureHeading,
  procedureLabel,
  scope,
  statsNote,
  summaryTitle,
} from './summary'

// doctor99213 is the real summary of 99213's doctor fees, with its first
// and third place lines.
const doctor99213: Summary = {
  count: 1108,
  left_out: 0,
  organization_count: 957,
  procedure_count: 0,
  with_procedure: 0,
  lowest: 41.31,
  middle: 51.45,
  highest: 159.15,
  most_common: 51.45,
  most_common_count: 618,
  most_common_organizations: 536,
  places: [
    { code: '02', name: '', count: 588, lowest: 41.31, middle: 51.45, highest: 90 },
    { code: '11', name: '', count: 309, lowest: 50.4, middle: 113.59, highest: 159.15 },
  ],
  procedures: [],
  mixed_pricing: false,
}

// line makes a summary line: a code, a name, a count, and the lowest,
// middle and highest prices.
function line(code: string, name: string, count: number, lowest: number, middle: number, highest: number): SummaryLine {
  return { code, name, count, lowest, middle, highest }
}

// surgery0490 is the real summary of 0490's facility fees: ten lines, of
// which nine are procedures, and one is the 14 fees that name none.
const surgery0490: Summary = {
  count: 485,
  left_out: 0,
  organization_count: 120,
  procedure_count: 161,
  with_procedure: 471,
  lowest: 128.34,
  middle: 649.99,
  highest: 10955.28,
  most_common: 0,
  most_common_count: 0,
  most_common_organizations: 0,
  places: [line('24', '', 470, 128.34, 644.39, 10955.28), line('22', '', 14, 176.18, 3718.92, 9091.97)],
  procedures: [
    line('45380', 'COLONOSCOPY AND BIOPSY', 42, 226.51, 433.08, 829.02),
    line('45385', 'COLONOSCOPY W/LESION REMOVAL', 34, 367.1, 568.65, 1154.16),
    line('43239', 'EGD BIOPSY SINGLE/MULTIPLE', 32, 245.12, 472.5, 1045.45),
    line('45378', 'DIAGNOSTIC COLONOSCOPY', 31, 226.51, 578.31, 1154.16),
    line('66984', '', 30, 1184, 1317.71, 1886.4),
    line('', '', 14, 176.18, 3718.92, 9091.97),
    line('29881', '', 12, 613.33, 1183.01, 1601.99),
    line('26055', '', 8, 364.82, 775.6, 979.01),
    line('29848', '', 8, 613.33, 1183.01, 1312.09),
    line('45381', '', 7, 226.51, 260.24, 312.5),
  ],
  mixed_pricing: false,
}

// er0450 is the real summary of the ER's facility fees (0450): its three
// procedures, the visit levels, all have lines, as do the 144 fees that
// name none.
const er0450: Summary = {
  count: 207,
  left_out: 0,
  organization_count: 119,
  procedure_count: 3,
  with_procedure: 63,
  lowest: 136.3,
  middle: 418.3,
  highest: 3568.46,
  most_common: 0,
  most_common_count: 0,
  most_common_organizations: 0,
  places: [line('23', '', 196, 146.76, 439.8, 3568.46), line('22', '', 12, 136.3, 164.93, 323.89)],
  procedures: [
    line('', '', 144, 136.3, 485.92, 3568.46),
    line('99285', 'EMERGENCY DEPT VISIT', 28, 146.76, 320.17, 492.82),
    line('99284', 'EMERGENCY DEPT VISIT', 27, 150.6, 308.25, 513.49),
    line('99283', 'EMERGENCY DEPT VISIT', 8, 220.59, 324.15, 540.43),
  ],
  mixed_pricing: false,
}

// lab0305 is the real summary of 0305: 7 of its 29 prices name one of 2
// procedures, but only 85025 has the 5 prices a line needs.
const lab0305: Summary = {
  count: 29,
  left_out: 0,
  organization_count: 21,
  procedure_count: 2,
  with_procedure: 7,
  lowest: 3.75,
  middle: 10.1,
  highest: 50.34,
  most_common: 0,
  most_common_count: 0,
  most_common_organizations: 0,
  places: [line('22', '', 20, 3.75, 10.1, 50.34), line('50', '', 7, 5.51, 11.03, 11.03)],
  procedures: [line('', '', 22, 3.75, 10.1, 50.34), line('85025', 'COMPLETE CBC W/AUTO DIFF WBC', 6, 5.51, 11.03, 11.03)],
  mixed_pricing: false,
}

const colonoscopy: SummaryLine = { code: '45380', name: 'COLONOSCOPY AND BIOPSY', count: 42, lowest: 226.51, middle: 433.08, highest: 829.02 }

describe('summaryTitle', () => {
  it('names every fee of the kind shown, for the code as the file shows it', () => {
    const answer = { class: 'professional', code: { display: '99213' }, query: { code: '99213' } } as SearchAnswer
    expect(summaryTitle(answer)).toBe('All professional fees for 99213')
    const typed = { class: 'institutional', code: { display: '0490' }, query: { code: '490' } } as SearchAnswer
    expect(summaryTitle(typed)).toBe('All facility fees for 0490')
  })
})

describe('marketFirst', () => {
  it('puts the market first for a code alone, and a named provider\'s fees first', () => {
    const asked = (boxes: object) => ({ query: { code: '99213', npi: '', ein: '', name: '', ...boxes } }) as SearchAnswer
    expect(marketFirst(asked({}))).toBe(true)
    expect(marketFirst(asked({ npi: '1780644302' }))).toBe(false)
    expect(marketFirst(asked({ ein: '13-4000192' }))).toBe(false)
    expect(marketFirst(asked({ name: 'sinai' }))).toBe(false)
  })
})

describe('canPickPlaces', () => {
  // asked is 99213's answer for the boxes given, with doctor fees shown.
  const asked = (boxes: object, doctorFees: number) =>
    ({
      query: { code: '99213', npi: '', ein: '', name: '', ...boxes },
      class: 'professional',
      classes: [
        { billing_class: 'professional', count: doctorFees, organization_count: 1 },
        { billing_class: 'institutional', count: 102, organization_count: 84 },
      ],
    }) as unknown as SearchAnswer

  it('lets the places narrow the table when the box comes before it', () => {
    expect(canPickPlaces(asked({}, 1108))).toBe(true)
  })

  it("doesn't after a provider's fees, or when the kind of fee shown has none", () => {
    expect(canPickPlaces(asked({ npi: '1780644302' }, 4))).toBe(false)
    expect(canPickPlaces(asked({}, 0))).toBe(false)
  })

  it("doesn't when every fee is at one place, so picking it would change nothing", () => {
    const office = { code: '11', name: '', count: 50, lowest: 0.11, middle: 0.18, highest: 0.98 } // J1100's 50 fees
    const one = (count: number) => ({ ...asked({}, 50), summary: { ...doctor99213, places: [{ ...office, count }] } }) as SearchAnswer
    expect(canPickPlaces(one(50))).toBe(false)
    // One line that leaves out fees at places with too few prices still narrows.
    expect(canPickPlaces(one(45))).toBe(true)
    // 97140: all 176 fees at 11 Office, 4 of them flagged, so the line counts 172.
    const flagged = { ...asked({}, 176), summary: { ...doctor99213, count: 172, left_out: 4, places: [{ ...office, count: 172 }] } }
    expect(canPickPlaces(flagged as SearchAnswer)).toBe(false)
  })
})

describe('scope', () => {
  it('counts the prices and organizations, and the procedures when there are several', () => {
    expect(scope(doctor99213)).toBe('1,108 prices from 957 organizations')
    expect(scope({ ...lab0305, procedure_count: 1, with_procedure: 6 })).toBe('29 prices from 21 organizations')
  })

  it('says how many prices name a procedure when some name none', () => {
    expect(scope(er0450)).toBe('207 prices from 119 organizations; 63 name one of 3 procedures')
    expect(scope(surgery0490)).toBe('485 prices from 120 organizations; 471 name one of 161 procedures')
    expect(scope(lab0305)).toBe('29 prices from 21 organizations; 7 name one of 2 procedures')
  })

  it('says the prices are for the procedures when every one names one', () => {
    // 0521's 175 prices, from 30 organizations, each name one of 64.
    const clinic0521 = { ...lab0305, count: 175, organization_count: 30, procedure_count: 64, with_procedure: 175 }
    expect(scope(clinic0521)).toBe('175 prices from 30 organizations, for 64 different procedures')
  })

  it('says one price or organization in the singular', () => {
    // Made up: the API never sums up fewer than 5 prices, but one
    // organization can post them all.
    expect(scope({ ...doctor99213, count: 5, organization_count: 1 })).toBe('5 prices from 1 organization')
  })
})

describe('modeShare', () => {
  it("says how many prices are the most common one, and whose", () => {
    expect(modeShare(doctor99213)).toBe('618 of 1,108 prices (56%), from 536 organizations')
  })

  it('rounds the share to the nearest percent', () => {
    // Made up: 112 of 1,000 is 11.2%, so 11%, not 12%.
    expect(modeShare({ ...doctor99213, count: 1000, most_common_count: 112 })).toBe('112 of 1,000 prices (11%), from 536 organizations')
  })

  it('says nothing when the server found no pattern', () => {
    expect(modeShare(surgery0490)).toBe('')
  })
})

describe('procedureHeading and procedureLabel', () => {
  it("says when some procedures aren't shown, not counting the fees that name none", () => {
    // 0490: ten lines, nine of them procedures.
    expect(procedureHeading(surgery0490)).toBe('By procedure: 9 of 161 shown')
    // 0305: one line for its 2 procedures; the other has too few prices.
    expect(procedureHeading(lab0305)).toBe('By procedure: 1 of 2 shown')
  })

  it('says nothing more when every procedure has a line', () => {
    expect(procedureHeading(er0450)).toBe('By procedure')
  })

  it("labels a line with the file's name, or says what's missing", () => {
    expect(procedureLabel(colonoscopy)).toBe('45380 · COLONOSCOPY AND BIOPSY')
    expect(procedureLabel({ ...colonoscopy, code: '66984', name: '' })).toBe('66984 · Not described in this file')
    expect(procedureLabel({ ...colonoscopy, code: '', name: '' })).toBe('No procedure named')
  })
})

describe('lineNumbers', () => {
  it('spells out what the bar draws', () => {
    expect(lineNumbers(colonoscopy)).toBe('middle $433.08 · $226.51 to $829.02 · 42 prices')
  })
})

describe('bar', () => {
  it('places a line on the scale from the lowest price to the highest', () => {
    // Telehealth: $41.31 to $90.00 of $41.31 to $159.15, middle $51.45.
    expect(bar(doctor99213.places[0], doctor99213)).toEqual({ left: '0.0%', width: '41.3%', middle: '8.6%' })
    // The office: $50.40 to $159.15, middle $113.59.
    expect(bar(doctor99213.places[1], doctor99213)).toEqual({ left: '7.7%', width: '92.3%', middle: '61.3%' })
  })

  it('puts everything at the start when every price is the same', () => {
    const flat = { ...doctor99213, lowest: 94.5, highest: 94.5 }
    expect(bar({ ...colonoscopy, lowest: 94.5, middle: 94.5, highest: 94.5 }, flat)).toEqual({ left: '0.0%', width: '0.0%', middle: '0.0%' })
  })
})

describe('statsNote', () => {
  it('says when the middle varies a lot by place', () => {
    expect(statsNote(doctor99213)).toBe(`The middle price varies by place, from $51.45 at ${placeLabel('02')} to $113.59 at ${placeLabel('11')}.`)
    expect(statsNote(surgery0490)).toBe(`The middle price varies by place, from $644.39 at ${placeLabel('24')} to $3,718.92 at ${placeLabel('22')}.`)
    // The lowest middle needn't be the first place listed: 0450's places come by count.
    expect(statsNote(er0450)).toBe(`The middle price varies by place, from $164.93 at ${placeLabel('22')} to $439.80 at ${placeLabel('23')}.`)
  })

  it('finds the lowest and highest middles anywhere in the list, and 1.5 times is enough', () => {
    const places = (...middles: [string, number][]) => ({ ...doctor99213, places: middles.map(([code, m]) => line(code, '', 5, m, m, m)) })
    expect(statsNote(places(['11', 12], ['02', 10], ['19', 20]))).toBe(
      `The middle price varies by place, from $10.00 at ${placeLabel('02')} to $20.00 at ${placeLabel('19')}.`,
    )
    expect(statsNote(places(['02', 10], ['11', 15]))).toBe(`The middle price varies by place, from $10.00 at ${placeLabel('02')} to $15.00 at ${placeLabel('11')}.`)
    // A tie names the place listed first, the one with the most prices (99214: 10 before 20).
    expect(statsNote(places(['11', 10], ['10', 20], ['20', 20]))).toBe(
      `The middle price varies by place, from $10.00 at ${placeLabel('11')} to $20.00 at ${placeLabel('10')}.`,
    )
  })

  it("says nothing when the places' middles are within 1.5 times of each other, or there's one place", () => {
    expect(statsNote(lab0305)).toBe('') // $10.10 and $11.03
    expect(statsNote({ ...doctor99213, places: doctor99213.places.slice(0, 1) })).toBe('')
    expect(statsNote({ ...doctor99213, places: [] })).toBe('')
  })

  it("warns instead when the prices mix modifiers that change what's priced", () => {
    expect(statsNote({ ...doctor99213, mixed_pricing: true })).toBe(
      "These prices mix modifiers that change what's priced, so these numbers mix different things, and fees aren't compared with them.",
    )
  })
})

describe('noSummary', () => {
  // asked is 36415's answer for the boxes given: facility fees shown, as
  // many as given, with or without a summary.
  const asked = (boxes: object, shown: number, summary: Summary | null) =>
    ({
      query: { code: '36415', npi: '', ein: '', name: '', ...boxes },
      code: { type: 'CPT', display: '36415', short_name: '', name: '' },
      class: 'institutional',
      classes: [
        { billing_class: 'professional', count: 0, organization_count: 0 },
        { billing_class: 'institutional', count: shown, organization_count: shown },
      ],
      summary,
    }) as unknown as SearchAnswer
  const why = "No market summary for 36415's facility fees: it needs at least 5 prices, not counting flagged ones."

  it('says why a code with too few prices has no box', () => {
    expect(noSummary(asked({}, 4, null))).toBe(why)
  })

  it("says so for a provider's search too, even one that found none of their fees", () => {
    expect(noSummary(asked({ npi: '1780644302' }, 0, null))).toBe(why)
  })

  it('says nothing when there is a box, or no fee of the kind shown to sum up', () => {
    expect(noSummary(asked({}, 4, doctor99213))).toBe('')
    expect(noSummary(asked({}, 0, null))).toBe('')
  })
})

describe('notes', () => {
  it('says how to read the box', () => {
    expect(notes(doctor99213)).toEqual(['Contracted rates: what the plan agreed to pay, not payments on claims.', 'A price posted for several places counts in each.'])
  })

  it('counts flagged prices left out, and skips the place note without place lines', () => {
    expect(notes({ ...surgery0490, places: [], left_out: 1 })).toEqual([
      'Contracted rates: what the plan agreed to pay, not payments on claims.',
      '1 flagged price left out.',
    ])
    expect(notes({ ...surgery0490, places: [], left_out: 2 })[1]).toBe('2 flagged prices left out.')
  })
})
