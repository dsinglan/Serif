import { describe, expect, it } from 'vitest'
import type { BillingClass, SearchAnswer, SearchQuery } from './api'
import { atPlace, boxesIn, goToPage, newSearch, sortBy, switchFees, thisPage, tidyEIN } from './search'

// shown is an answer on screen, shaped the way the server sends it: the
// query it echoes, and the kind of fee and page shown, out of 120 rows of
// that kind. Its rows are left out, since the click rules never read them.
function shown(query: Partial<SearchQuery>, billingClass: BillingClass, page = 1): SearchAnswer {
  const asked: SearchQuery = { code: '99213', npi: '', ein: '', name: '', class: '', place: '', order: '', page: 0, page_size: 0, ...query }
  const total = 120
  const pageSize = asked.page_size || 50
  return {
    query: asked,
    code: { type: 'CPT', display: asked.code, short_name: '', name: '' },
    code_found: true,
    provider_found: true,
    not_found: [],
    provider_organizations: { npi: [], ein: [] },
    classes: [
      { billing_class: 'professional', count: billingClass === 'professional' ? total : 0, organization_count: 3 },
      { billing_class: 'institutional', count: billingClass === 'institutional' ? total : 0, organization_count: 3 },
    ],
    class: billingClass,
    total,
    organization_count: 3,
    page,
    pages: Math.ceil(total / pageSize),
    page_size: pageSize,
    rows: [],
    modifiers: [],
    summary: null,
  }
}

// echoOf is the query the server echoes for an address the page sent: what
// the address asks for, and "" or 0 for the rest.
function echoOf(params: URLSearchParams): Partial<SearchQuery> {
  return {
    ...boxesIn(params),
    class: (params.get('class') ?? '') as SearchQuery['class'],
    place: params.get('place') ?? '',
    order: (params.get('order') ?? '') as SearchQuery['order'],
    page: Number(params.get('page') ?? 0),
    page_size: Number(params.get('page_size') ?? 0),
  }
}

describe('tidyEIN', () => {
  it('adds the dash to 9 digits, however they were typed or pasted', () => {
    expect(tidyEIN('160743029')).toBe('16-0743029')
    expect(tidyEIN(' 16 0743029 ')).toBe('16-0743029')
    expect(tidyEIN('16-0743029')).toBe('16-0743029')
  })

  it('leaves anything else as typed, for the server to explain', () => {
    expect(tidyEIN('16-07430293432432')).toBe('16-07430293432432')
    expect(tidyEIN('12-345')).toBe('12-345')
    expect(tidyEIN('EIN 16-0743029')).toBe('EIN 16-0743029')
    expect(tidyEIN('')).toBe('')
  })
})

describe('newSearch', () => {
  it('sends the boxes filled in, trimmed, with no kind of fee or page', () => {
    const typed = { code: ' 99213 ', npi: '', ein: ' 20-3910419 ', name: '  ' }
    expect(newSearch(typed).toString()).toBe('code=99213&ein=20-3910419')
  })
})

describe('switchFees', () => {
  it('sends the boxes and page size on screen, the kind of fee clicked, and page 1', () => {
    const answer = shown({ code: '99213', npi: '1780644302', page: 3, page_size: 10 }, 'professional', 3)
    expect(switchFees(answer, 'institutional').toString()).toBe(
      'code=99213&npi=1780644302&class=institutional&page_size=10',
    )
  })
})

describe('goToPage', () => {
  it('keeps every box and the kind of fee on screen, even one the server picked', () => {
    const answer = shown({ code: '27447', npi: '1780644302', ein: '13-4000192', name: 'sinai', class: '' }, 'institutional')
    expect(goToPage(answer, 2).toString()).toBe(
      'code=27447&npi=1780644302&ein=13-4000192&name=sinai&class=institutional&page=2',
    )
  })

  it('leaves out page 1, and keeps a page size someone asked for', () => {
    const answer = shown({ code: '97110', page: 2, page_size: 10 }, 'professional', 2)
    expect(goToPage(answer, 1).toString()).toBe('code=97110&class=professional&page_size=10')
  })

  it('keeps the place on screen', () => {
    const answer = shown({ code: '99213', place: '11' }, 'professional')
    expect(goToPage(answer, 2).toString()).toBe('code=99213&class=professional&place=11&page=2')
  })
})

describe('atPlace', () => {
  it('sends the search on screen for one place, from page 1', () => {
    const answer = shown({ code: '99213', npi: '1780644302', page: 3, page_size: 10 }, 'professional', 3)
    expect(atPlace(answer, '11').toString()).toBe('code=99213&npi=1780644302&class=professional&place=11&page_size=10')
  })

  it('sends every place again for ""', () => {
    const answer = shown({ code: '99213', place: '11', page: 2 }, 'professional', 2)
    expect(atPlace(answer, '').toString()).toBe('code=99213&class=professional')
  })
})

describe('sortBy', () => {
  it('sends the search on screen in the order clicked, from page 1', () => {
    const answer = shown({ code: '99213', npi: '1780644302', place: '11', page: 3, page_size: 10 }, 'professional', 3)
    expect(sortBy(answer, 'highest').toString()).toBe('code=99213&npi=1780644302&class=professional&place=11&order=highest&page_size=10')
    expect(sortBy(answer, 'lowest').toString()).toBe('code=99213&npi=1780644302&class=professional&place=11&page_size=10')
  })

  it('keeps the order through pages, places and fee switches, but not a new search', () => {
    const answer = shown({ code: '99213', place: '11', order: 'highest' }, 'professional')
    expect(goToPage(answer, 2).toString()).toBe('code=99213&class=professional&place=11&order=highest&page=2')
    expect(atPlace(answer, '02').toString()).toBe('code=99213&class=professional&place=02&order=highest')
    expect(switchFees(answer, 'institutional').toString()).toBe('code=99213&class=institutional&order=highest')
    expect(newSearch({ code: '99213', npi: '', ein: '', name: '' }).toString()).toBe('code=99213')
  })
})

describe('switchFees and newSearch', () => {
  it('drop the place: the other kind of fee has places of its own', () => {
    const answer = shown({ code: '99213', place: '11' }, 'professional')
    expect(switchFees(answer, 'institutional').toString()).toBe('code=99213&class=institutional')
    expect(newSearch({ code: '99213', npi: '', ein: '', name: '' }).toString()).toBe('code=99213')
  })
})

describe('thisPage', () => {
  it('sends the search on screen as the server read it: every box, the kind of fee, place, order, page and page size', () => {
    const asked = { code: '99213', npi: '1780644302', ein: '13-4000192', name: 'sinai', class: 'professional', place: '11', order: 'highest', page: 2, page_size: 10 } as const
    expect(thisPage(shown(asked, 'professional', 2)).toString()).toBe(
      'code=99213&npi=1780644302&ein=13-4000192&name=sinai&class=professional&place=11&order=highest&page=2&page_size=10',
    )
  })

  it('leaves a kind of fee the server picked to the server, as the page did', () => {
    const answer = shown({ code: '0450', class: '' }, 'institutional')
    expect(thisPage(answer).toString()).toBe('code=0450')
  })

  it("is the page's own address, whichever click sent it", () => {
    const answer = shown({ code: '99213', npi: '1780644302' }, 'professional')
    const sent = [
      newSearch({ code: '99213', npi: '1780644302', ein: '', name: '' }),
      goToPage(answer, 2),
      atPlace(answer, '11'),
      sortBy(answer, 'highest'),
      switchFees(answer, 'institutional'),
    ]
    for (const params of sent) {
      expect(thisPage(shown(echoOf(params), 'professional')).toString()).toBe(params.toString())
    }
  })
})

describe('boxesIn', () => {
  it('reads the boxes of a search in the address', () => {
    const params = new URLSearchParams('code=99213&npi=1780644302&ein=13-4000192&name=sinai&page=2')
    expect(boxesIn(params)).toEqual({ code: '99213', npi: '1780644302', ein: '13-4000192', name: 'sinai' })
  })

  it('leaves a box empty when the address has none', () => {
    expect(boxesIn(new URLSearchParams('code=0450'))).toEqual({ code: '0450', npi: '', ein: '', name: '' })
    expect(boxesIn(new URLSearchParams(''))).toEqual({ code: '', npi: '', ein: '', name: '' })
  })
})
