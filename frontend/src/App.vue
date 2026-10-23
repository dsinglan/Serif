<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, useTemplateRef, watchEffect } from 'vue'
import CodeSuggestions from './components/CodeSuggestions.vue'
import Results from './components/Results.vue'
import SearchForm from './components/SearchForm.vue'
import {
  findCodes,
  getStatus,
  searchApi,
  type BillingClass,
  type CodesAnswer,
  type SearchAnswer,
  type SearchOutcome,
  type Status,
} from './lib/api'
import {
  boxMessages,
  canTryAgain,
  clearableBoxes,
  clearLabel,
  describeOutcome,
  describeView,
  isBox,
  isProblem,
  pageTitle,
  providerNotes,
  type ProviderBox,
} from './lib/outcome'
import { atPlace, boxesIn, goToPage, newSearch, sortBy, switchFees, type Order, type SearchBoxes } from './lib/search'
import { savedFile } from './lib/source'
import { describeStatus, download, startCommand } from './lib/status'
import { afterLookup, matchCount, suggestionsLine, wordsOnly, wordsToLookUp } from './lib/suggest'

const status = ref<Status | null>(null)
const unreachable = ref(false) // the Go server isn't answering at all
let timer: number | undefined

const boxes = ref<SearchBoxes>({ code: '', npi: '', ein: '', name: '' })
const outcome = ref<SearchOutcome | null>(null) // how the last search ended; null before the first
const searching = ref(false) // a search is on its way
const suggestions = ref<CodesAnswer | null>(null) // codes whose names contain the words typed for a code
let asked: URLSearchParams | null = null // the last search sent, for Try again
let waiting: URLSearchParams | null = null // a search sent while the data was loading, to send again once it's ready
let latest = 0 // counts searches, so an answer to an older one is dropped

const form = useTemplateRef('form')
const outcomeArea = useTemplateRef('outcomeArea')
const results = useTemplateRef('results')
const suggestionsPanel = useTemplateRef('suggestions-panel') // not "suggestions": that's the codes above

// progress is how far the rate file has downloaded, for the progress bar.
const progress = computed(() => (status.value && !unreachable.value ? download(status.value) : null))

// answer is the last search's answer, when it got one.
const answer = computed(() => {
  const o = outcome.value
  return o?.kind === 'results' ? o.answer : null
})

// messages are what to say under each box after a search. With codes to
// pick from, there are none: the search was for a code that isn't one, so
// the code box's error and the provider boxes' notes don't apply.
const messages = computed(() => (suggestions.value ? {} : boxMessages(outcome.value)))

// The browser tab is named after what's on screen (see pageTitle). A search
// that ends in a problem leaves the address as it was, so the tab too: its
// name stays the history entry's.
watchEffect(() => {
  const shown = outcome.value
  if (!suggestions.value && shown && shown.kind !== 'results') {
    return
  }
  document.title = pageTitle(shown, suggestions.value)
})

// boxError is the server's message about bad input in a box. It's shown
// under that box, and read out again from the line under the form, which
// otherwise says nothing then.
const boxError = computed(() => {
  const o = outcome.value
  return o?.kind === 'invalid' && isBox(o.field) ? o.message : ''
})

// Example searches for a first visit, each showing something different
// about the file. The names after the codes are the ones the results'
// heading shows: the file's short names, and 0450's standard name.
const examples: { label: string; note: string; boxes: Partial<SearchBoxes> }[] = [
  { label: '99213 · OFFICE/OUTPATIENT VISIT EST', note: 'An office visit: what doctors and clinics are paid', boxes: { code: '99213' } },
  { label: '27447', note: 'A procedure found only inside hospital fees', boxes: { code: '27447' } },
  { label: '450 · Emergency Room General', note: 'Hospital ER fees; the leading zero is optional', boxes: { code: '450' } },
  { label: 'J1100 · INJ DEXMETH SODM PHOSHA 1MG', note: 'An injected drug, priced per 1 mg', boxes: { code: 'J1100' } },
  { label: '90837 · PSYTX W PT 60 MINUTES', note: '60-minute psychotherapy, with many pages of prices', boxes: { code: '90837' } },
  { label: '99213 with NPI 1780644302', note: 'One NPI, listed by two organizations', boxes: { code: '99213', npi: '1780644302' } },
]

// poll asks the server where loading is: every second while it loads, and
// every 5 seconds after a failure or while the server isn't answering, so a
// restarted server is noticed. Once loading ends, ready or failed, a
// waiting search is sent again: it gets its answer, or the reason it can't.
async function poll() {
  window.clearTimeout(timer)
  try {
    status.value = await getStatus()
    unreachable.value = false
  } catch {
    unreachable.value = true
  }
  const phase = status.value?.phase
  if (!unreachable.value && (phase === 'ready' || phase === 'failed') && waiting) {
    search(waiting, false)
  }
  if (unreachable.value || phase === 'failed') {
    timer = window.setTimeout(poll, 5000)
  } else if (phase !== 'ready') {
    timer = window.setTimeout(poll, 1000)
  }
}

// search sends one search and shows how it ended. A search the person just
// asked for moves the cursor: to a box with bad input, to the results
// heading (or, for another page, place or order of the same results, to
// the table), or back to the code box. A waiting one that runs by itself
// later (userAsked false) leaves it where they are now.
async function search(params: URLSearchParams, userAsked = true, then: 'heading' | 'table' = 'heading') {
  const id = ++latest
  asked = params
  waiting = null // a new search replaces one that was waiting
  searching.value = true
  // Words in the code box, such as "office visit", aren't a code: look up
  // the codes whose names contain them. Words that can't be a code skip the
  // search (see wordsOnly); others, such as "knee", are looked up when the
  // search finds no such code.
  const typed = params.get('code') ?? ''
  const direct = wordsOnly(typed)
  const result = direct ? null : await searchApi(params)
  const words = direct || (result ? wordsToLookUp(typed, result) : '')
  const lookup = words ? await findCodes(words) : null
  if (id !== latest) {
    return // a newer search has started, so this answer is out of date
  }
  const { outcome: shown, suggestions: offered } = afterLookup(result, lookup)
  searching.value = false
  outcome.value = shown
  suggestions.value = offered
  if (shown?.kind === 'results' || offered) {
    // The address holds the search, so a refresh or a shared link opens it.
    // A new one is a step Back can return from; the same one again (a
    // refresh, Try again, or Back itself) isn't.
    const same = new URLSearchParams(location.search).toString() === params.toString()
    history[same ? 'replaceState' : 'pushState'](null, '', `?${params}`)
  }
  if (shown?.kind === 'loading') {
    waiting = params // send it again once the data is ready
  }
  // The status line follows the server: how loading is going, why it
  // failed, that the server isn't answering or, when it said anything but
  // Ready, that an answer just came, so it may be back.
  const kind = shown?.kind
  if (kind === 'loading' || kind === 'failed' || kind === 'unreachable' || unreachable.value || status.value?.phase !== 'ready') {
    poll()
  }
  await nextTick()
  if (!userAsked) {
    return // the line under the form says how it ended
  }
  if (offered && matchCount(offered) > 0) {
    suggestionsPanel.value?.focusHeading() // a screen reader starts at the codes to pick from
  } else if (offered) {
    form.value?.focusBox('code') // no code's name has these words, so try others
  } else if (shown?.kind === 'invalid' && isBox(shown.field)) {
    form.value?.focusBox(shown.field) // so the bad input can be fixed
  } else if (shown?.kind === 'results' && then === 'table') {
    results.value?.focusTable() // another page, place or order: its rows, not the summary above them
    return
  } else if (shown?.kind === 'results' && shown.answer.rows.length > 0) {
    // A screen reader starts at the heading; the line under the form says how many.
    results.value?.focusHeading()
  } else if (document.activeElement === document.body) {
    // The button the cursor was on (an example, or Try again) is gone:
    // put the cursor back in the code box.
    form.value?.focusBox('code')
  }
  outcomeArea.value?.scrollIntoView({ block: 'nearest' }) // on a phone, the line can be below the screen
}

function tryExample(example: (typeof examples)[number]) {
  boxes.value = { code: '', npi: '', ein: '', name: '', ...example.boxes }
  search(newSearch(boxes.value))
}

// pickCode searches a code picked from the suggestions, with the provider
// boxes as they are.
function pickCode(code: string) {
  boxes.value.code = code
  search(newSearch(boxes.value))
}

function tryAgain() {
  if (asked) {
    search(asked)
  }
}

// clearBox empties one provider box and searches again, as if the person
// had emptied it and pressed Search.
function clearBox(box: ProviderBox) {
  boxes.value[box] = ''
  search(newSearch(boxes.value))
}

// onScreen is the answer whose results are on screen, for their buttons to
// ask for another view of it. While a search is on its way it's null: the
// results are out of date (Results fades them), and a click on one mustn't
// replace that search with another view of them.
function onScreen(): SearchAnswer | null {
  return searching.value ? null : answer.value
}

// showFees and showPage send the search on screen again, for the other
// kind of fee or another page. search.ts has the rules for what they send.
function showFees(billingClass: BillingClass) {
  const shown = onScreen()
  if (shown) {
    search(switchFees(shown, billingClass))
  }
}

function showPage(page: number) {
  const shown = onScreen()
  if (shown) {
    search(goToPage(shown, page), true, 'table')
  }
}

// showOrder shows the fees with the lowest or the highest price first.
function showOrder(order: Order) {
  const shown = onScreen()
  if (shown) {
    search(sortBy(shown, order), true, 'table')
  }
}

// showPlace shows only one place's fees in the table, or every place's
// again for "".
function showPlace(place: string) {
  const shown = onScreen()
  if (shown) {
    search(atPlace(shown, place), true, 'table')
  }
}

// showAddress shows the search in the page's address after Back or
// Forward, or the first visit's examples when there's none.
function showAddress() {
  const params = new URLSearchParams(location.search)
  boxes.value = boxesIn(params)
  if (params.get('code')) {
    search(params, false)
    return
  }
  latest++ // an answer still on its way is out of date
  waiting = null
  searching.value = false
  outcome.value = null
  suggestions.value = null
}

onMounted(() => {
  poll()
  window.addEventListener('popstate', showAddress)
  // A search in the page's address, from a refresh or a shared link, runs right away.
  const params = new URLSearchParams(location.search)
  if (params.get('code')) {
    boxes.value = boxesIn(params)
    search(params)
  }
})
onUnmounted(() => {
  window.clearTimeout(timer)
  window.removeEventListener('popstate', showAddress)
})
</script>

<template>
  <main>
    <!-- The title goes back to the first visit, with its examples. -->
    <h1><a href="./">Price Transparency Explorer</a></h1>
    <p class="lede">
      Look up what this health plan agreed to pay doctors and hospitals for a service, by its billing code, and see how one provider
      compares with the rest.
    </p>
    <!-- One status line that stays on the page, so a screen reader hears
         each change of phase; the progress bar shows how far a download is. -->
    <p class="notice" :class="{ error: unreachable || status?.phase === 'failed' }" role="status">
      <template v-if="unreachable">
        Can't reach the search server. Is it running? Start it with <code>{{ startCommand }}</code>.
      </template>
      <template v-else-if="status">{{ describeStatus(status) }}</template>
      <template v-else>Connecting…</template>
    </p>
    <progress v-if="progress" class="download" :value="progress.done" :max="progress.total" aria-label="Rate file downloaded" />

    <SearchForm ref="form" v-model="boxes" :messages="messages" @search="search(newSearch(boxes))" />

    <!-- Every answer is new text here, so a screen reader reads it, even a
         repeat: the line says "Searching…" in between. When a search finds
         no prices, the notes say where the file lists the NPI and EIN, and
         a button per provider box clears it and searches again. -->
    <div ref="outcomeArea" class="outcome" aria-live="polite">
      <p v-if="searching">Searching…</p>
      <p v-else-if="suggestions" class="sr-only">{{ suggestionsLine(suggestions) }}</p>
      <template v-else-if="outcome">
        <p v-if="describeOutcome(outcome)" :class="{ 'outcome-error': isProblem(outcome) }">{{ describeOutcome(outcome) }}</p>
        <p v-if="boxError" class="sr-only">{{ boxError }}</p>
        <template v-if="answer">
          <p v-if="describeView(answer)" class="sr-only">{{ describeView(answer) }}</p>
          <div v-for="note in providerNotes(answer)" :key="note.lead" class="provider-note">
            <p>{{ note.lead }}</p>
            <ul>
              <li v-for="name in note.names" :key="name">{{ name }}</li>
            </ul>
          </div>
          <div v-if="clearableBoxes(answer).length" class="outcome-actions">
            <button v-for="box in clearableBoxes(answer)" :key="box" type="button" class="secondary" @click="clearBox(box)">
              {{ clearLabel(box) }}
            </button>
          </div>
        </template>
        <div v-if="canTryAgain(outcome)" class="outcome-actions">
          <button type="button" class="secondary" @click="tryAgain">Try again</button>
        </div>
      </template>
    </div>

    <CodeSuggestions v-if="suggestions" ref="suggestions-panel" :suggestions="suggestions" @pick="pickCode" />

    <Results
      v-if="answer?.code_found"
      ref="results"
      :answer="answer"
      :busy="searching"
      :cms-read="Boolean(status?.source?.hcpcs_code_count)"
      :file="savedFile(status?.source?.file_url ?? '')"
      @fees="showFees"
      @page="showPage"
      @place="showPlace"
      @order="showOrder"
    />

    <section v-if="!outcome && !suggestions && !searching" class="first-visit" aria-labelledby="examples-heading">
      <h2 id="examples-heading">Search one billing code, then narrow by NPI, EIN or organization name</h2>
      <p class="hint">Or try one of these:</p>
      <ul class="examples">
        <li v-for="example in examples" :key="example.label">
          <button type="button" class="example" @click="tryExample(example)">
            <span class="example-label">{{ example.label }}</span>
            <span class="example-note">{{ example.note }}</span>
          </button>
        </li>
      </ul>
    </section>
  </main>
</template>
