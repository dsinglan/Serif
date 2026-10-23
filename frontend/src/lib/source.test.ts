import { describe, expect, it } from 'vitest'
import type { Row } from './api'
import { endDate, listed, savedFile, sourceLines } from './source'

// sourced makes a row with only a source: where its price and its group are.
function sourced(rate: number, ratesIndex: number, priceIndex: number, groupID: string, groupIndex: number): Row {
  return {
    source: {
      in_network_index: 1417, // 99213's item
      negotiated_rates_index: ratesIndex,
      negotiated_prices_index: priceIndex,
      provider_group_id: groupID,
      provider_groups_index: groupIndex,
      listed_times: 1,
    },
    negotiated_rate: rate,
  } as Row
}

// Real 99213 rows. ENT & Allergy's $137.39 is the only price in its entry,
// and its group is first in provider reference 1160. Dermatologic Services'
// $82.32 is the second price in its entry. East Point's $41.31 is the second
// group in provider reference 874.
const ent = sourced(137.39, 11, 0, '1160', 0)
const dermatology = sourced(82.32, 43, 1, '1224', 0)
const eastPoint = sourced(41.31, 47, 0, '874', 1)

const file = 'data/2026-08-31_centene-management-company-llc_fidelis-ex_in-network.json'

describe('savedFile', () => {
  it('names the saved copy the way the server does: data/ and the end of the address', () => {
    const url = 'https://www.centene.com/content/dam/centene/Centene%20Corporate/json/DOCUMENT/2026-08-31_centene-management-company-llc_fidelis-ex_in-network.json'
    expect(savedFile(url)).toBe(file)
  })

  it('decodes the name, as the server does', () => {
    expect(savedFile('https://example.com/files/my%20rates.json')).toBe('data/my rates.json')
  })

  it('says what to put there before the address is known, or when it makes no sense', () => {
    expect(savedFile('')).toBe('data/<the rate file>.json')
    expect(savedFile('not an address')).toBe('data/<the rate file>.json')
  })
})

describe('sourceLines', () => {
  it('give the price and the group, each with a path and a command that prints just it', () => {
    expect(sourceLines(ent, file)).toEqual([
      {
        label: 'Price',
        path: 'in_network[1417].negotiated_rates[11].negotiated_prices[0]',
        command: `jq '.in_network[1417].negotiated_rates[11] | {provider_references, price: .negotiated_prices[0]}' '${file}'`,
      },
      {
        label: 'Provider group',
        path: 'provider_references, provider_group_id 1160, provider_groups[0]',
        command: `jq '.provider_references[] | select(.provider_group_id == 1160) | .provider_groups[0]' '${file}'`,
      },
    ])
  })

  it("pick out the row's own price when its entry has several", () => {
    const [price] = sourceLines(dermatology, file)
    expect(price.path).toBe('in_network[1417].negotiated_rates[43].negotiated_prices[1]')
    expect(price.command).toBe(`jq '.in_network[1417].negotiated_rates[43] | {provider_references, price: .negotiated_prices[1]}' '${file}'`)
  })

  it("pick the row's own group when its provider reference lists several", () => {
    const [, group] = sourceLines(eastPoint, file)
    expect(group.path).toBe('provider_references, provider_group_id 874, provider_groups[1]')
    expect(group.command).toBe(`jq '.provider_references[] | select(.provider_group_id == 874) | .provider_groups[1]' '${file}'`)
  })

  it('build no command from anything but plain numbers', () => {
    // Made up: a value that would break out of the quotes if pasted.
    const odd = sourced(1, 0, 0, "1160' ; touch pwned ; '", 0)
    expect(sourceLines(odd, file).map((line) => line.command)).toEqual(['', ''])
    const negative = sourced(1, -1, 0, '1160', 0)
    expect(sourceLines(negative, file).map((line) => line.command)).toEqual(['', ''])
    const fraction = sourced(1, 0, 0.5, '1160', 0)
    expect(sourceLines(fraction, file).map((line) => line.command)).toEqual(['', ''])
  })

  it('quote the file for the shell, whatever its name', () => {
    const [, spaced] = sourceLines(ent, 'data/my rates.json')
    expect(spaced.command.endsWith(` 'data/my rates.json'`)).toBe(true)
    const [, quoted] = sourceLines(ent, "data/o'brien.json")
    expect(quoted.command.endsWith(` 'data/o'\\''brien.json'`)).toBe(true)
  })
})

describe('endDate', () => {
  it('reads 9999-12-31 as no end date, as CMS defines it', () => {
    expect(endDate('9999-12-31')).toBe('No end date')
  })

  it('shows any other date as a date', () => {
    expect(endDate('2026-03-31')).toBe('Mar 31, 2026')
  })
})

describe('listed', () => {
  it('says how many listings in the file the row stands for', () => {
    expect(listed(1)).toBe('Once in the file')
    expect(listed(162)).toBe('162 times in the file')
    expect(listed(1500)).toBe('1,500 times in the file')
  })
})
