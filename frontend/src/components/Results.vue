<script setup lang="ts">
import { computed, ref, useTemplateRef, watch } from 'vue'
import { searchAddress, type BillingClass, type Row, type SearchAnswer } from '../lib/api'
import { thisPage, type Order } from '../lib/search'
import { placeLabel } from '../lib/places'
import {
  caption,
  cutNote,
  discontinued,
  facilityColumns,
  feeButton,
  flagLabels,
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
} from '../lib/rows'
import { marketFirst, noSummary } from '../lib/summary'
import Pager from './Pager.vue'
import RowDetails from './RowDetails.vue'
import SummaryBox from './SummaryBox.vue'

// busy says a newer search is on its way, so these results are out of date:
// they fade, and the page ignores their buttons. cmsRead says whether the
// server could read CMS's HCPCS file, for the legend; file is where the
// rate file is saved, for the rows' jq commands.
const props = defineProps<{ answer: SearchAnswer; busy: boolean; cmsRead: boolean; file: string }>()

// fees, page, place and order ask the page for the same search with the
// other kind of fee, another page, one place ("" for every place), or the
// other order.
const emit = defineEmits<{ fees: [billingClass: BillingClass]; page: [page: number]; place: [place: string]; order: [order: Order] }>()

// order is which price the table shows first.
const order = computed<Order>(() => (props.answer.query.order === 'highest' ? 'highest' : 'lowest'))

const head = computed(() => (props.answer.code ? heading(props.answer.code) : null))
const anyFees = computed(() => props.answer.classes.some((c) => c.count > 0)) // with none, the switch has nothing to switch
const facility = computed(() => props.answer.class === 'institutional') // facility fees get their own columns
const columns = computed(() => facilityColumns(props.answer)) // which of them
const modifierLegend = computed(() => legend(props.answer.modifiers, props.cmsRead))
const jsonAddress = computed(() => searchAddress(thisPage(props.answer))) // the API's address for this search
const missingSummary = computed(() => noSummary(props.answer)) // why there's no market box, or ""

// pick asks for the other kind of fee. The kind shown is already pressed,
// so pressing it again does nothing.
function pick(billingClass: BillingClass) {
  if (billingClass !== props.answer.class) {
    emit('fees', billingClass)
  }
}

// sort asks for the other order. The order shown is already pressed, so
// pressing it again does nothing.
function sort(o: Order) {
  if (o !== order.value) {
    emit('order', o)
  }
}

// open holds the rows whose details are showing, by rowKey. Another search
// or page closes them.
const open = ref(new Set<string>())
watch(
  () => props.answer,
  () => open.value.clear(),
)

function toggle(key: string) {
  if (open.value.has(key)) {
    open.value.delete(key)
  } else {
    open.value.add(key)
  }
}

// taxID says which tax ID a row's organization bills under.
function taxID(row: Row): string {
  return row.tin_type === 'npi' ? `NPI ${row.tin}` : `EIN ${row.tin}`
}

// The h2's ref can't be named "heading": that's the heading() imported
// above, and a production build would use that function as the ref.
const headingElement = useTemplateRef('heading-element')

// focusHeading puts the cursor on the heading, so after a search a screen
// reader starts reading the results there.
function focusHeading() {
  headingElement.value?.focus()
}

const tableElement = useTemplateRef('table-element')
const placeBar = useTemplateRef('place-bar')
const sortSwitch = useTemplateRef('sort-switch')
const allPlacesButton = useTemplateRef('all-places-button')

// focusTable puts the cursor on the table. When its first row is off screen,
// as after Next at the bottom of a page or a place picked in the summary, it
// brings the table's top into view, starting at the controls just above it
// (the place bar or the order buttons), so they stay in sight. When the first
// row is already on screen, as after an order click, the page stays where it
// is. When a place leaves no rows, the cursor goes to Show all places; with
// no rows and no place, to the heading.
function focusTable() {
  const cursorAt = tableElement.value ?? allPlacesButton.value ?? headingElement.value
  const viewFrom = placeBar.value ?? sortSwitch.value ?? tableElement.value ?? headingElement.value
  cursorAt?.focus({ preventScroll: true })
  const firstRow = tableElement.value?.querySelector('tbody tr') ?? cursorAt
  if (firstRow && !onScreen(firstRow)) {
    viewFrom?.scrollIntoView({ block: 'start' })
  }
}

// onScreen says the whole element is inside the window.
function onScreen(el: Element): boolean {
  const box = el.getBoundingClientRect()
  return box.top >= 0 && box.bottom <= window.innerHeight
}

defineExpose({ focusHeading, focusTable })
</script>

<template>
  <section class="results" :class="{ busy }" :aria-busy="busy ? 'true' : undefined" aria-labelledby="results-heading">
    <h2 id="results-heading" ref="heading-element" tabindex="-1">{{ head?.title ?? answer.query.code }}</h2>
    <p v-if="head?.subtitle" class="code-name">{{ head.subtitle }}</p>
    <p v-if="cutNote([head?.subtitle ?? ''])" class="hint">{{ cutNote([head?.subtitle ?? '']) }}</p>
    <p v-if="head?.note" class="code-note">{{ head.note }}</p>

    <!-- Two on/off buttons, not tabs: the one pressed is the class shown. A
         class with no fees has nothing to show, so its button is off. -->
    <div v-if="anyFees" class="fee-switch" role="group" aria-label="Kind of fee">
      <button
        v-for="c in answer.classes"
        :key="c.billing_class"
        type="button"
        class="secondary"
        :aria-pressed="c.billing_class === answer.class ? 'true' : 'false'"
        :disabled="c.count === 0"
        @click="pick(c.billing_class)"
      >
        {{ feeButton(c) }}
      </button>
    </div>

    <p v-if="noFeesShown(answer)" class="no-fees">{{ noFeesShown(answer) }}</p>

    <!-- The market first for a search for a code alone; for one that names a
         provider, after their fees (see marketFirst). With too few prices
         for a box, a line says so where it would be. -->
    <SummaryBox v-if="answer.summary && marketFirst(answer)" :answer="answer" :summary="answer.summary" @place="emit('place', $event)" />
    <p v-if="missingSummary && marketFirst(answer)" class="no-summary">{{ missingSummary }}</p>

    <div v-if="answer.query.place" ref="place-bar" class="place-filter">
      <p>{{ placeNote(answer) }}</p>
      <button ref="all-places-button" type="button" class="secondary" @click="emit('place', '')">Show all places</button>
    </div>

    <!-- Two on/off buttons: the one pressed is the order shown. With fewer
         than 2 fees there's nothing to order. Beside them, the page buttons,
         which the end of the table has too. -->
    <div v-if="answer.total > 1" class="table-controls">
      <div ref="sort-switch" class="sort-switch" role="group" aria-labelledby="sort-label">
        <span id="sort-label">Order:</span>
        <button type="button" class="secondary" :aria-pressed="order === 'lowest' ? 'true' : 'false'" @click="sort('lowest')">
          Lowest price first
        </button>
        <button type="button" class="secondary" :aria-pressed="order === 'highest' ? 'true' : 'false'" @click="sort('highest')">
          Highest price first
        </button>
      </div>
      <Pager v-if="answer.pages > 1" :page="answer.page" :pages="answer.pages" label="Pages, above the table" @page="emit('page', $event)" />
    </div>

    <!-- For a provider's fees, how the lines under their prices add up. The
         table points to it, so a screen reader hears it when the cursor
         lands on the table after a page or order click. -->
    <p v-if="versusTally(answer)" id="versus-tally" class="versus-tally">{{ versusTally(answer) }}</p>

    <!-- The roles keep the table a table for screen readers on phones,
         where CSS turns each row into a card. -->
    <table
      v-if="answer.rows.length"
      ref="table-element"
      class="rates"
      role="table"
      tabindex="-1"
      :aria-describedby="versusTally(answer) ? 'versus-tally' : undefined"
    >
      <caption>{{ caption(answer) }}</caption>
      <thead role="rowgroup">
        <tr role="row">
          <th role="columnheader" scope="col" class="price">Price</th>
          <th role="columnheader" scope="col">Organization</th>
          <template v-if="facility">
            <th v-if="columns.billedAs" role="columnheader" scope="col">Billed as</th>
            <th v-if="columns.procedure" role="columnheader" scope="col">Procedure</th>
          </template>
          <th v-else role="columnheader" scope="col">Covers</th>
          <th role="columnheader" scope="col">Place</th>
          <th v-if="!facility" role="columnheader" scope="col">Modifier</th>
          <th role="columnheader" scope="col">Source</th>
        </tr>
      </thead>
      <tbody role="rowgroup">
        <template v-for="(row, i) in answer.rows" :key="rowKey(row)">
          <tr role="row">
            <td role="cell" class="price">
              <span class="cell-label" aria-hidden="true">Price</span>
              <span class="cell-value">
                {{ money(row.negotiated_rate) }}
                <span v-if="versusMiddle(row, answer)" class="versus"><span class="sr-only">, </span>{{ versusMiddle(row, answer) }}</span>
              </span>
            </td>
            <td role="cell">
              <span class="cell-label" aria-hidden="true">Organization</span>
              <span class="cell-value">
                <span class="org">{{ row.business_name }}</span>
                <span class="detail">{{ taxID(row) }}</span>
                <span v-for="flag in row.flags" :key="flag" class="flag">{{ flagLabels[flag] }}</span>
              </span>
            </td>
            <template v-if="facility">
              <td v-if="columns.billedAs" role="cell">
                <span class="cell-label" aria-hidden="true">Billed as</span>
                <span class="cell-value">{{ row.billing_code_type }} {{ row.display_code }} · {{ nameOrNote(row.label, row.billing_code_type === 'RC') }}</span>
              </td>
              <!-- The procedure's short name, with its full name on a hover. -->
              <td v-if="columns.procedure" role="cell" class="procedure">
                <span class="cell-label" aria-hidden="true">Procedure</span>
                <span v-if="row.procedure_code" class="cell-value" :title="row.procedure_short_name ? row.procedure_label : undefined">
                  {{ row.procedure_code }} · {{ procedureName(row) }}
                  <span v-if="row.procedure_discontinued" class="flag">{{ discontinued(row.procedure_discontinued) }}</span>
                </span>
                <span v-else class="cell-value detail">None listed</span>
              </td>
            </template>
            <td v-else role="cell" class="covers">
              <span class="cell-label" aria-hidden="true">Covers</span>
              <span class="cell-value">{{ whoIsCovered(row) }}</span>
            </td>
            <td role="cell">
              <span class="cell-label" aria-hidden="true">Place</span>
              <span class="cell-value places">
                <span v-for="code in row.service_code" :key="code" class="place">{{ placeLabel(code) }}</span>
              </span>
            </td>
            <td v-if="!facility" role="cell">
              <span class="cell-label" aria-hidden="true">Modifier</span>
              <!-- A modifier CMS explains shows its meaning on a hover; the
                   legend under the table explains them all. -->
              <span class="cell-value">
                <template v-for="(code, i) in row.billing_code_modifier" :key="code">
                  <template v-if="i">, </template>
                  <abbr v-if="modifierMeaning(code, answer.modifiers)" :title="modifierMeaning(code, answer.modifiers)">{{ code }}</abbr>
                  <template v-else>{{ code }}</template>
                </template>
                <template v-if="row.billing_code_modifier.length === 0">None</template>
              </span>
            </td>
            <td role="cell">
              <span class="cell-label" aria-hidden="true">Source</span>
              <span class="cell-value">
                <button
                  type="button"
                  class="secondary details-button"
                  :aria-expanded="open.has(rowKey(row)) ? 'true' : 'false'"
                  :aria-controls="`details-${i}`"
                  @click="toggle(rowKey(row))"
                >
                  Details<span class="sr-only">: {{ money(row.negotiated_rate) }}, {{ row.business_name }}</span>
                </button>
              </span>
            </td>
          </tr>
          <!-- Where the row came from in the file, under it, the full width of the table. -->
          <tr v-if="open.has(rowKey(row))" :id="`details-${i}`" role="row" class="details-row">
            <td role="cell" :colspan="facility ? 5 : 6">
              <RowDetails :row="row" :file="file" />
            </td>
          </tr>
        </template>
      </tbody>
    </table>

    <Pager v-if="answer.pages > 1" :page="answer.page" :pages="answer.pages" label="Pages, below the table" @page="emit('page', $event)" />

    <template v-if="modifierLegend.length">
      <h3 class="legend-heading">Modifiers on this page</h3>
      <dl class="legend">
        <div v-for="line in modifierLegend" :key="line.codes">
          <dt>{{ line.codes }}</dt>
          <dd>{{ line.meaning }}</dd>
        </div>
      </dl>
    </template>

    <SummaryBox v-if="answer.summary && !marketFirst(answer)" :answer="answer" :summary="answer.summary" @place="emit('place', $event)" />
    <p v-if="missingSummary && !marketFirst(answer)" class="no-summary">{{ missingSummary }}</p>

    <!-- The link asks the API for the search on screen, so anyone can read
         the page's data as JSON. -->
    <p class="api-link">
      This page as JSON:
      <a :href="jsonAddress" target="_blank" rel="noopener"
        ><code>{{ jsonAddress }}</code><span aria-hidden="true"> ↗</span><span class="sr-only"> (opens in a new tab)</span></a
      >
    </p>
  </section>
</template>
