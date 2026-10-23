import type { BillingClass, SearchAnswer } from './api'

// SearchBoxes are the search form's boxes: as typed, or as an answer's query
// echoes them. The code is required; the others may be "".
export interface SearchBoxes {
  code: string
  npi: string
  ein: string
  name: string
}

// Order is which price the table shows first.
export type Order = 'lowest' | 'highest'

// View is what a search asks for beyond the boxes. Leaving billingClass out
// (or "", as an answer echoes it) lets the server pick the kind of fee
// (doctor fees, unless there are only facility fees). Every place, the
// lowest price first, the first page and the server's page size are the
// defaults.
interface View {
  billingClass?: BillingClass | ''
  place?: string
  order?: Order | ''
  page?: number
  pageSize?: number
}

// tidyEIN writes an EIN the usual way, 20-3910419, once it holds 9 digits
// with nothing else but spaces or a dash, as when it's pasted. Anything else
// is left as typed: the server checks it, and its message says what's wrong.
export function tidyEIN(typed: string): string {
  const digits = typed.replace(/[\s-]/g, '')
  return /^\d{9}$/.test(digits) ? `${digits.slice(0, 2)}-${digits.slice(2)}` : typed
}

// searchParams builds a search's address: the code, and the other boxes
// that are filled in, all trimmed; the kind of fee, the place, the highest
// price first and the page size when they're asked for; and the page when
// it's past the first.
export function searchParams(boxes: SearchBoxes, view: View = {}): URLSearchParams {
  const params = new URLSearchParams({ code: boxes.code.trim() })
  for (const box of ['npi', 'ein', 'name'] as const) {
    const value = boxes[box].trim()
    if (value) {
      params.set(box, value)
    }
  }
  if (view.billingClass) {
    params.set('class', view.billingClass)
  }
  if (view.place) {
    params.set('place', view.place)
  }
  if (view.order === 'highest') {
    params.set('order', 'highest') // the lowest first goes without saying
  }
  if (view.page && view.page > 1) {
    params.set('page', String(view.page))
  }
  if (view.pageSize) {
    params.set('page_size', String(view.pageSize))
  }
  return params
}

// boxesIn reads the boxes of a search in the page's address, as a refresh,
// a shared link, or Back and Forward bring it.
export function boxesIn(params: URLSearchParams): SearchBoxes {
  return {
    code: params.get('code') ?? '',
    npi: params.get('npi') ?? '',
    ein: params.get('ein') ?? '',
    name: params.get('name') ?? '',
  }
}

// Five rules say what a click sends.

// newSearch is the Search button, an example search, or a Clear button: the
// boxes as typed, no kind of fee, so the server picks one, and the first
// page.
export function newSearch(typed: SearchBoxes): URLSearchParams {
  return searchParams(typed)
}

// switchFees is a fee switch button: the boxes, page size and order of the
// search on screen (not whatever has been typed since), the kind of fee
// clicked, and the first page. The place is dropped: the other kind of fee
// has places of its own.
export function switchFees(shown: SearchAnswer, billingClass: BillingClass): URLSearchParams {
  return searchParams(shown.query, { billingClass, order: shown.query.order, pageSize: shown.query.page_size })
}

// goToPage is the next or previous button: the boxes, page size, kind of
// fee, place and order of the search on screen (even a kind the server
// picked), and the page clicked.
export function goToPage(shown: SearchAnswer, page: number): URLSearchParams {
  const { place, order, page_size: pageSize } = shown.query
  return searchParams(shown.query, { billingClass: shown.class, place, order, page, pageSize })
}

// atPlace is a place in the summary box, or Show all places (""): the
// boxes, page size, kind of fee and order of the search on screen, the
// place, and the first page.
export function atPlace(shown: SearchAnswer, place: string): URLSearchParams {
  return searchParams(shown.query, { billingClass: shown.class, place, order: shown.query.order, pageSize: shown.query.page_size })
}

// sortBy is an order button: the search on screen in that order, from the
// first page.
export function sortBy(shown: SearchAnswer, order: Order): URLSearchParams {
  return searchParams(shown.query, { billingClass: shown.class, place: shown.query.place, order, pageSize: shown.query.page_size })
}

// thisPage is the search on screen as the server read it (the query its
// answer echoes), for the link to the same search as JSON. Like the page's
// address, it leaves out what nobody asked for, such as a kind of fee the
// server picked: the server picks the same again.
export function thisPage(shown: SearchAnswer): URLSearchParams {
  const { class: billingClass, place, order, page, page_size: pageSize } = shown.query
  return searchParams(shown.query, { billingClass, place, order, page, pageSize })
}
