<script setup lang="ts">
import { computed } from 'vue'
import type { SearchAnswer, Summary, SummaryLine } from '../lib/api'
import { placeLabel } from '../lib/places'
import { money } from '../lib/rows'
import { bar, canPickPlaces, lineNumbers, modeShare, notes, procedureHeading, procedureLabel, scope, statsNote, summaryTitle } from '../lib/summary'

// The box sums up every price for the code in the class shown, before any
// NPI, EIN, name or place narrows them: the market the table's rows can be
// compared to. Picking a place line shows only its fees in the table.
const props = defineProps<{ answer: SearchAnswer; summary: Summary }>()
const emit = defineEmits<{ place: [place: string] }>()

// breakdowns are the box's lists of lines, by place of service and, for a
// revenue code, by procedure. A list with no lines isn't shown.
const breakdowns = computed(() => {
  const s = props.summary
  const lists: { heading: string; places: boolean; lines: { line: SummaryLine; label: string }[] }[] = [
    { heading: 'By place of service', places: true, lines: s.places.map((line) => ({ line, label: placeLabel(line.code) })) },
    { heading: procedureHeading(s), places: false, lines: s.procedures.map((line) => ({ line, label: procedureLabel(line) })) },
  ]
  return lists.filter((list) => list.lines.length > 0)
})

// canPick says the places can narrow the table (see canPickPlaces). When
// they can't, the lines are names.
const canPick = computed(() => canPickPlaces(props.answer))

// pick shows only a place's fees, or every place's again when it's the one
// shown.
function pick(place: string) {
  emit('place', props.answer.query.place === place ? '' : place)
}
</script>

<template>
  <section class="summary" aria-labelledby="summary-heading">
    <h3 id="summary-heading">{{ summaryTitle(answer) }}</h3>
    <p class="summary-scope">{{ scope(summary) }}</p>

    <dl class="stats">
      <div>
        <dt>Lowest</dt>
        <dd>{{ money(summary.lowest) }}</dd>
      </div>
      <div>
        <dt>Middle</dt>
        <dd>{{ money(summary.middle) }}</dd>
      </div>
      <div>
        <dt>Highest</dt>
        <dd>{{ money(summary.highest) }}</dd>
      </div>
      <div v-if="modeShare(summary)">
        <dt>Most common</dt>
        <dd>
          {{ money(summary.most_common) }}
          <small>{{ modeShare(summary) }}</small>
        </dd>
      </div>
    </dl>
    <p v-if="statsNote(summary)" class="stats-note">{{ statsNote(summary) }}</p>

    <!-- Each line's bar runs from its lowest price to its highest, with a
         dot at its middle, on one scale for the whole box. The bars are
         hidden from screen readers: the words under them say the same. -->
    <div v-for="list in breakdowns" :key="list.heading" class="breakdown">
      <h4>{{ list.heading }}</h4>
      <p v-if="list.places && canPick" class="hint">Pick a place to show only its fees in the table.</p>
      <ul class="lines">
        <li v-for="{ line, label } in list.lines" :key="line.code">
          <button
            v-if="list.places && canPick"
            type="button"
            class="line-label line-filter"
            :aria-pressed="answer.query.place === line.code ? 'true' : 'false'"
            @click="pick(line.code)"
          >
            <span class="sr-only">Show only fees at </span>{{ label }}
          </button>
          <span v-else class="line-label">{{ label }}</span>
          <span class="line-bar" aria-hidden="true">
            <span class="line-range" :style="{ left: bar(line, summary).left, width: bar(line, summary).width }"></span>
            <span class="line-middle" :style="{ left: bar(line, summary).middle }"></span>
          </span>
          <span class="line-numbers">{{ lineNumbers(line) }}</span>
        </li>
      </ul>
    </div>

    <p class="summary-notes">{{ notes(summary).join(' ') }}</p>
  </section>
</template>
