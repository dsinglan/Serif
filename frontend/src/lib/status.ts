import type { Status } from './api'

// The command that starts the Go server, for messages that need it.
export const startCommand = 'go -C backend run ./cmd/server'

// describeStatus says where loading is, in one line for the page. It names
// only the phase, so a screen reader hears each phase once; while the rate
// file downloads, a progress bar shows how far it has got (see download).
export function describeStatus(st: Status): string {
  switch (st.phase) {
    case 'downloading':
      return 'Downloading the rate file…'
    case 'fetching_cms_names':
      return "Loading CMS's names for HCPCS codes…"
    case 'reading':
      return 'Reading the rate file…'
    case 'ready': {
      const src = st.source
      if (!src) return 'Ready.'
      // A "provider rate" is one row: one price for one provider group.
      const rates = `${src.row_count.toLocaleString('en-US')} provider ${src.row_count === 1 ? 'rate' : 'rates'}`
      const market = src.plan_market_types.length ? `${src.plan_market_types.join(' and ')}-market ` : ''
      const plans = src.issuer_names.length ? ` for ${src.issuer_names.join(' and ')}'s ${market}plans` : ''
      return `Ready: ${rates}${plans}, from ${src.reporting_entity_name}'s in-network file (updated ${formatDate(src.file_last_updated_on)}).`
    }
    case 'failed':
      return `Couldn't load the data (${st.error ?? 'no reason given'}). Fix the cause, often the network, then restart the server: ${startCommand}`
    default:
      return 'Loading: fetching the index…'
  }
}

// download is how far the rate file's download has got, for the progress
// bar, or null when it isn't downloading or the server doesn't know the size.
export function download(st: Status): { done: number; total: number } | null {
  if (st.phase !== 'downloading' || !st.bytes_total || st.bytes_total <= 0) {
    return null
  }
  return { done: st.bytes_done ?? 0, total: st.bytes_total }
}

// formatDate shows a date-only string like "2026-09-02" as "Sep 2, 2026".
// It reads and writes the date in UTC; otherwise, in New York, midnight UTC
// would show as the day before.
export function formatDate(isoDate: string): string {
  const d = new Date(`${isoDate}T00:00:00Z`)
  if (Number.isNaN(d.getTime())) return isoDate
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' })
}
