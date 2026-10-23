<script setup lang="ts">
import { computed, useTemplateRef } from 'vue'
import type { CodesAnswer } from '../lib/api'
import { count, cutNote } from '../lib/rows'
import { buttons, codeLabel, moreLine, sameNames, showFullName, suggestionsTitle } from '../lib/suggest'

// The codes whose names contain the words typed, in two lists that are
// never ranked against each other: hospital charges (revenue codes) and
// services (CPT and HCPCS codes). Picking one searches it.
const props = defineProps<{ suggestions: CodesAnswer }>()
const emit = defineEmits<{ pick: [code: string] }>()

// lists are the kinds of code that have matches, with what each kind is.
const lists = computed(() =>
  [
    {
      key: 'charges',
      heading: 'Hospital charges',
      note: 'Revenue codes: in this file, every price for one is a facility fee.',
      group: props.suggestions.charges,
    },
    {
      key: 'services',
      heading: 'Services',
      note: "CPT and HCPCS codes: their own prices are doctor fees, and hospitals' fees can name them too.",
      group: props.suggestions.services,
    },
  ].filter((list) => list.group.total > 0),
)

// cut explains the "…" on full names the rate file cut short, when a
// button shows one.
const cut = computed(() =>
  cutNote(lists.value.flatMap((list) => list.group.codes.slice(0, buttons).filter((m) => showFullName(m)).map((m) => m.name))),
)

const headingElement = useTemplateRef('heading-element')

// focusHeading puts the cursor on the heading, so after a search a screen
// reader starts reading the codes there.
function focusHeading() {
  headingElement.value?.focus()
}

defineExpose({ focusHeading })
</script>

<template>
  <section class="suggestions" aria-labelledby="suggestions-heading">
    <h2 id="suggestions-heading" ref="heading-element" tabindex="-1">{{ suggestionsTitle(suggestions) }}</h2>
    <template v-if="lists.length">
      <p class="hint">The ones with the most prices first. Pick one to search it.</p>
      <div v-for="list in lists" :key="list.key" class="suggestion-list">
        <h3>{{ list.heading }} ({{ list.group.total }})</h3>
        <p class="hint">{{ list.note }}</p>
        <ul>
          <li v-for="m in list.group.codes.slice(0, buttons)" :key="m.display">
            <button type="button" class="suggestion" @click="emit('pick', m.display)">
              <span class="suggestion-code">{{ codeLabel(m) }}</span>
              <span v-if="showFullName(m)" class="suggestion-name">{{ m.name }}</span>
              <span class="hint">{{ count(m.count, 'price', 'prices') }}</span>
            </button>
          </li>
        </ul>
        <p v-if="moreLine(list.group)" class="hint">{{ moreLine(list.group) }}</p>
        <p v-if="sameNames(list.group)" class="hint">{{ sameNames(list.group) }}</p>
      </div>
      <p v-if="cut" class="hint">{{ cut }}</p>
    </template>
    <p v-else>
      Names come from the file and from CMS, and some codes have none: a procedure that appears only inside hospital fees isn't named
      anywhere in the file. Try fewer words, or search by the code itself.
    </p>
  </section>
</template>
