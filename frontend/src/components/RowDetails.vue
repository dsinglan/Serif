<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Row } from '../lib/api'
import { count, money } from '../lib/rows'
import { endDate, listed, sourceLines, type SourceLine } from '../lib/source'

// The details under a row: where its price and its provider group are in
// the rate file, with jq commands that print just those, so a number on the
// page can be checked against the file itself. file is the saved copy's
// path.
const props = defineProps<{ row: Row; file: string }>()

// merged is a row from an NPI search that stands for several groups listing
// the doctor at this price. Its paths, commands and NPIs are the first
// group's.
const merged = computed(() => (props.row.groups ?? 0) > 1)

// copyNote says how the last copy went, in a status line a screen reader
// reads out.
const copyNote = ref('')

// copy puts a line's command on the clipboard. If the browser refuses, the
// note says so: the command is on screen to select by hand.
async function copy(line: SourceLine) {
  try {
    await navigator.clipboard.writeText(line.command)
    copyNote.value = `Copied the ${line.label.toLowerCase()} command.`
  } catch {
    copyNote.value = `Couldn't copy. Select the ${line.label.toLowerCase()} command and copy it by hand.`
  }
}
</script>

<template>
  <div class="row-details">
    <h3>
      Where this price is in the rate file<span class="sr-only">: {{ money(row.negotiated_rate) }}, {{ row.business_name }}</span>
    </h3>
    <dl>
      <div v-for="line in sourceLines(row, file)" :key="line.label">
        <dt>{{ line.label }}</dt>
        <dd>
          <code>{{ line.path }}</code>
          <span v-if="line.command" class="command">
            <code>{{ line.command }}</code>
            <button type="button" class="secondary" @click="copy(line)">
              Copy<span class="sr-only"> the {{ line.label.toLowerCase() }} command</span>
            </button>
          </span>
        </dd>
      </div>
      <div>
        <dt>Listed</dt>
        <dd>{{ listed(row.source.listed_times) }}</dd>
      </div>
      <div>
        <dt>Contract ends</dt>
        <dd>{{ endDate(row.expiration_date) }}</dd>
      </div>
      <div>
        <dt>{{ merged ? "First group's NPIs" : 'NPIs' }}</dt>
        <dd v-if="row.npis.length">
          <details>
            <summary>{{ count(row.npis.length, 'NPI', 'NPIs') }}</summary>
            <ul class="npis">
              <li v-for="npi in row.npis" :key="npi">{{ npi }}</li>
            </ul>
          </details>
        </dd>
        <dd v-else-if="row.tin_level">None: the file lists NPI 0, so the contract is with the whole practice</dd>
        <dd v-else>None listed</dd>
      </div>
      <div v-if="row.invalid_npis.length">
        <dt>Not valid NPIs</dt>
        <dd>{{ row.invalid_npis.join(', ') }}, listed in the file</dd>
      </div>
    </dl>
    <p v-if="merged" class="hint">
      This row stands for {{ row.groups }} groups that list this doctor at this price. The paths, the commands and the NPIs are the first
      group's; Listed counts every group's listings.
    </p>
    <p class="hint">Run the commands from the project's folder.</p>
    <p class="hint" role="status">{{ copyNote }}</p>
  </div>
</template>
