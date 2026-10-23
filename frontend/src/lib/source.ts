import type { Row } from './api'
import { formatDate } from './status'

// savedFile is where the server saved the rate file, for the jq commands:
// data/ and the last part of the file's address, as the server names it
// (run them from the project's folder). Without an address, it says what to
// put there.
export function savedFile(fileUrl: string): string {
  let name = ''
  try {
    name = decodeURIComponent(new URL(fileUrl).pathname.split('/').pop() ?? '')
  } catch {
    // no address yet, or one the browser can't read: say what to put there
  }
  return name ? `data/${name}` : 'data/<the rate file>.json'
}

// SourceLine is one place in the rate file that a row comes from: what it
// is, where it is, and a jq command that prints just that.
export interface SourceLine {
  label: string
  path: string
  command: string
}

// sourceLines are the two places a row comes from: its price (printed with
// the provider references it points at) and its provider group. The
// commands go into a shell, so they're built only from plain numbers: the
// indexes are counts, and CMS's schema makes provider_group_id an integer.
// Anything else, and a line has no command ("").
export function sourceLines(row: Row, file: string): SourceLine[] {
  const s = row.source
  const indexes = [s.in_network_index, s.negotiated_rates_index, s.negotiated_prices_index, s.provider_groups_index]
  const plain = /^\d+$/.test(s.provider_group_id) && indexes.every((n) => Number.isInteger(n) && n >= 0)
  const rate = `in_network[${s.in_network_index}].negotiated_rates[${s.negotiated_rates_index}]`
  return [
    {
      label: 'Price',
      path: `${rate}.negotiated_prices[${s.negotiated_prices_index}]`,
      command: plain ? `jq '.${rate} | {provider_references, price: .negotiated_prices[${s.negotiated_prices_index}]}' ${shellQuote(file)}` : '',
    },
    {
      label: 'Provider group',
      path: `provider_references, provider_group_id ${s.provider_group_id}, provider_groups[${s.provider_groups_index}]`,
      command: plain
        ? `jq '.provider_references[] | select(.provider_group_id == ${s.provider_group_id}) | .provider_groups[${s.provider_groups_index}]' ${shellQuote(file)}`
        : '',
    },
  ]
}

// shellQuote wraps a path in single quotes, so the shell reads it as one
// argument even with a space or a semicolon in the name. A quote inside it
// becomes '\'' (end the quotes, an escaped quote, start them again).
function shellQuote(path: string): string {
  return `'${path.replaceAll("'", `'\\''`)}'`
}

// endDate says when a price's contract ends. CMS's schema has 9999-12-31
// stand for no end date.
export function endDate(date: string): string {
  return date === '9999-12-31' ? 'No end date' : formatDate(date)
}

// listed says how many listings in the file a row stands for: copies of the
// same price for the same group (a hospital's price list can be printed
// under many items), or, after an NPI search merged look-alike rows, the
// listings of every group it stands for.
export function listed(times: number): string {
  return times === 1 ? 'Once in the file' : `${times.toLocaleString('en-US')} times in the file`
}
