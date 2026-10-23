import { afterEach, describe, expect, it, vi } from 'vitest'
import { findCodes, searchAddress, searchApi, type Status } from './api'

// serverAnswers makes fetch answer with this status, body and content type.
function serverAnswers(status: number, body: string, type = 'application/json') {
  const fetch = vi.fn(async () => new Response(body, { status, headers: { 'Content-Type': type } }))
  vi.stubGlobal('fetch', fetch)
  return fetch
}

const params = new URLSearchParams({ code: '99213', npi: '1780644302' })

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('searchApi', () => {
  it('asks for the search it is given, at the address searchAddress builds', async () => {
    const fetch = serverAnswers(200, '{"total": 0, "rows": []}')
    await searchApi(params)
    expect(fetch).toHaveBeenCalledWith('/api/search?code=99213&npi=1780644302')
    expect(searchAddress(params)).toBe('/api/search?code=99213&npi=1780644302')
  })

  it('returns the answer to a search that works', async () => {
    serverAnswers(200, '{"total": 4, "rows": []}')
    expect(await searchApi(params)).toEqual({ kind: 'results', answer: { total: 4, rows: [] } })
  })

  it('names the box a 400 is about', async () => {
    serverAnswers(400, '{"error": {"code": "invalid_npi", "field": "npi", "message": "An NPI has 10 digits."}}')
    expect(await searchApi(params)).toEqual({ kind: 'invalid', field: 'npi', message: 'An NPI has 10 digits.' })
  })

  it('reads a 400 about the whole address', async () => {
    serverAnswers(400, '{"error": {"code": "invalid_query", "message": "The search address is malformed."}}')
    expect(await searchApi(params)).toEqual({ kind: 'invalid', field: '', message: 'The search address is malformed.' })
  })

  it('tells still loading from failed on a 503, with the status', async () => {
    const loading: Status = { phase: 'downloading', bytes_done: 1, bytes_total: 2 }
    serverAnswers(503, JSON.stringify({ error: { code: 'not_ready', message: 'Still loading.' }, status: loading }))
    expect(await searchApi(params)).toEqual({ kind: 'loading', status: loading })

    const failed: Status = { phase: 'failed', error: 'no such host' }
    serverAnswers(503, JSON.stringify({ error: { code: 'load_failed', message: 'Loading the data failed.' }, status: failed }))
    expect(await searchApi(params)).toEqual({ kind: 'failed', status: failed })
  })

  it('treats a 503 without a status as an error', async () => {
    serverAnswers(503, '{"error": {"code": "not_ready", "message": "Still loading."}}')
    expect(await searchApi(params)).toEqual({ kind: 'error', message: 'Still loading.' })
  })

  it("treats Vite's empty 502 as the Go server being off", async () => {
    serverAnswers(502, '', 'text/plain')
    expect(await searchApi(params)).toEqual({ kind: 'unreachable' })
  })

  it('treats a 502 with a JSON error as an error, not as unreachable', async () => {
    serverAnswers(502, '{"error": {"code": "bad_gateway", "message": "A proxy failed."}}')
    expect(await searchApi(params)).toEqual({ kind: 'error', message: 'A proxy failed.' })
  })

  it('treats no answer at all as unreachable', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => Promise.reject(new TypeError('Failed to fetch'))))
    expect(await searchApi(params)).toEqual({ kind: 'unreachable' })
  })

  it("passes on a crash's message", async () => {
    serverAnswers(500, '{"error": {"code": "internal", "message": "Something went wrong on the server. See its log."}}')
    expect(await searchApi(params)).toEqual({ kind: 'error', message: 'Something went wrong on the server. See its log.' })
  })

  it('reads a body as JSON only when the server says it is JSON', async () => {
    serverAnswers(500, '{"error": {"code": "internal", "message": "unlabeled"}}', 'text/plain')
    expect(await searchApi(params)).toEqual({ kind: 'error', message: 'The server answered 500.' })
  })

  it('copes with an answer that is not JSON, or is cut off', async () => {
    serverAnswers(500, 'oops', 'text/plain')
    expect(await searchApi(params)).toEqual({ kind: 'error', message: 'The server answered 500.' })

    serverAnswers(200, '{"total": 4, "ro')
    expect(await searchApi(params)).toEqual({ kind: 'error', message: "The server's answer couldn't be read." })
  })
})

describe('findCodes', () => {
  const answer = { query: 'office visit', charges: { total: 0, codes: [] }, services: { total: 0, codes: [] } }

  it('asks for the words, spaces and all', async () => {
    const fetch = serverAnswers(200, JSON.stringify(answer))
    await findCodes('office visit & more')
    expect(fetch).toHaveBeenCalledWith('/api/codes?q=office+visit+%26+more')
  })

  it('returns the codes', async () => {
    serverAnswers(200, JSON.stringify(answer))
    expect(await findCodes('office visit')).toEqual({ kind: 'codes', answer })
  })

  it('has the same problems a search has', async () => {
    const loading: Status = { phase: 'reading' }
    serverAnswers(503, JSON.stringify({ error: { code: 'not_ready', message: 'Still loading.' }, status: loading }))
    expect(await findCodes('office visit')).toEqual({ kind: 'loading', status: loading })

    serverAnswers(400, '{"error": {"code": "invalid_q", "field": "q", "message": "Type 2 to 100 characters of a code\'s name."}}')
    expect(await findCodes('x')).toEqual({ kind: 'invalid', field: 'q', message: "Type 2 to 100 characters of a code's name." })

    vi.stubGlobal('fetch', vi.fn(async () => Promise.reject(new TypeError('Failed to fetch'))))
    expect(await findCodes('office visit')).toEqual({ kind: 'unreachable' })

    serverAnswers(200, '{"query": "office vi')
    expect(await findCodes('office visit')).toEqual({ kind: 'error', message: "The server's answer couldn't be read." })
  })
})
