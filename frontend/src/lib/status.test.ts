import { describe, expect, it } from 'vitest'
import type { Status } from './api'
import { describeStatus, download, formatDate } from './status'

describe('describeStatus', () => {
  it('names the phase only, so it reads the same all through a download', () => {
    expect(describeStatus({ phase: 'downloading', bytes_done: 1_000_000, bytes_total: 89_359_830 })).toBe('Downloading the rate file…')
    expect(describeStatus({ phase: 'downloading', bytes_done: 47_000_000, bytes_total: 89_359_830 })).toBe('Downloading the rate file…')
    expect(describeStatus({ phase: 'reading' })).toBe('Reading the rate file…')
  })

  it('says what is ready, and from where', () => {
    const st: Status = {
      phase: 'ready',
      source: {
        index_url: '', index_last_updated_on: '2026-08-31', reporting_entity_name: 'Centene Management Company LLC',
        reporting_entity_type: 'Third Party Administrator',
        file_url: '', file_version: '2.0.0', file_last_updated_on: '2026-09-02', file_bytes: 89_359_830,
        issuer_names: ['Fidelis Care'], plan_market_types: ['individual'],
        plan_names: [], reporting_plan_count: 88, row_count: 29_446, listing_count: 201_907,
        skipped_item_count: 0, unlinked_listing_count: 0, missing_rate_count: 0,
        hcpcs_url: '', hcpcs_code_count: 8770, hcpcs_modifier_count: 384,
      },
    }
    expect(describeStatus(st)).toBe(
      "Ready: 29,446 provider rates for Fidelis Care's individual-market plans, from Centene Management Company LLC's in-network file (updated Sep 2, 2026).",
    )
  })

  it('leaves out the plans when the index names no issuer', () => {
    const st: Status = {
      phase: 'ready',
      source: {
        index_url: '', index_last_updated_on: '2026-08-31', reporting_entity_name: 'Test Insurer', reporting_entity_type: '',
        file_url: '', file_version: '2.0.0', file_last_updated_on: '2026-09-02', file_bytes: 1,
        issuer_names: [], plan_market_types: [], plan_names: [], reporting_plan_count: 1, row_count: 1, listing_count: 1,
        skipped_item_count: 0, unlinked_listing_count: 0, missing_rate_count: 0,
        hcpcs_url: '', hcpcs_code_count: 0, hcpcs_modifier_count: 0,
      },
    }
    expect(describeStatus(st)).toBe("Ready: 1 provider rate, from Test Insurer's in-network file (updated Sep 2, 2026).")
  })

  it('says why loading failed, and how to restart', () => {
    expect(describeStatus({ phase: 'failed', error: 'downloading the index: no such host' })).toBe(
      "Couldn't load the data (downloading the index: no such host). Fix the cause, often the network, then restart the server: go -C backend run ./cmd/server",
    )
  })
})

describe('download', () => {
  it('says how far the rate file has got, for the progress bar', () => {
    expect(download({ phase: 'downloading', bytes_done: 47_000_000, bytes_total: 89_359_830 })).toEqual({
      done: 47_000_000,
      total: 89_359_830,
    })
  })

  it('has nothing to show when the size is unknown, or nothing is downloading', () => {
    expect(download({ phase: 'downloading', bytes_done: 1000, bytes_total: -1 })).toBeNull()
    expect(download({ phase: 'reading', bytes_done: 89_359_830, bytes_total: 89_359_830 })).toBeNull()
  })
})

describe('formatDate', () => {
  it('keeps the date the file says, whatever the time zone', () => {
    expect(formatDate('2026-09-02')).toBe('Sep 2, 2026')
  })

  it('leaves text it cannot read as it is', () => {
    expect(formatDate('soon')).toBe('soon')
  })
})
