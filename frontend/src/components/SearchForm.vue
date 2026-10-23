<script setup lang="ts">
import { ref, watch } from 'vue'
import { missingCode, type BoxMessage } from '../lib/outcome'
import { tidyEIN, type SearchBoxes } from '../lib/search'

// The boxes belong to the page: typing here changes them there.
const boxes = defineModel<SearchBoxes>({ required: true })

// messages are what to say under each box: the server's message about bad
// input (invalid), or a note that the file has nothing for that box.
const props = defineProps<{ messages: Partial<Record<keyof SearchBoxes, BoxMessage>> }>()
const emit = defineEmits<{ search: [] }>()

// The optional boxes, which narrow a search to a provider.
const providerBoxes = [
  { box: 'npi', label: 'NPI', hint: '10 digits, such as 1780644302', inputmode: 'numeric' },
  { box: 'ein', label: 'EIN (tax ID)', hint: '9 digits, such as 20-3910419', inputmode: 'numeric' },
  { box: 'name', label: 'Organization name', hint: 'Words from the name, such as sinai', inputmode: 'text' },
] as const

// noCode is what to say when Search is pressed with the code box empty (see
// missingCode). Typing a code clears it, and so do the page filling the
// boxes anew, as after Back or an example, and the next search's messages.
const noCode = ref('')
watch([() => boxes.value, () => boxes.value.code, () => props.messages], () => (noCode.value = ''))

// tries counts presses with no code, so the message is put back each time
// and a screen reader reads it again.
const tries = ref(0)

// submit asks the page to search, or, with no code to search, says so under
// the code box and puts the cursor there.
function submit() {
  noCode.value = missingCode(boxes.value)
  if (noCode.value) {
    tries.value++
    focusBox('code')
    return
  }
  emit('search')
}

// messageFor is what to say under one box, or "". A missing code's message
// follows the provider boxes as they change.
function messageFor(box: keyof SearchBoxes): string {
  return (box === 'code' && noCode.value && missingCode(boxes.value)) || props.messages[box]?.text || ''
}

// invalid reports whether a box holds bad input, by the server's check, or
// is the code box left empty.
function invalid(box: keyof SearchBoxes): 'true' | undefined {
  return (box === 'code' && noCode.value) || props.messages[box]?.invalid ? 'true' : undefined
}

// describedBy links a box to its hint and, when there is one, its message,
// so a screen reader reads them with the box.
function describedBy(box: keyof SearchBoxes): string {
  return messageFor(box) ? `hint-${box} message-${box}` : `hint-${box}`
}

// tidy writes an EIN the usual way when the cursor leaves its box, so
// 160743029 becomes 16-0743029. While typing, the box is left alone.
function tidy(box: keyof SearchBoxes) {
  if (box === 'ein') {
    boxes.value.ein = tidyEIN(boxes.value.ein)
  }
}

// focusBox puts the cursor in one of the boxes.
function focusBox(box: keyof SearchBoxes) {
  document.getElementById(`box-${box}`)?.focus()
}

defineExpose({ focusBox }) // so the page can move the cursor to a box after a search
</script>

<template>
  <form class="search" @submit.prevent="submit">
    <div class="field">
      <label for="box-code">Billing code</label>
      <div class="code-row">
        <!-- Required, said with aria-required rather than required, so the
             page's own message shows instead of the browser's. -->
        <input
          id="box-code"
          v-model="boxes.code"
          aria-required="true"
          autocomplete="off"
          spellcheck="false"
          :aria-invalid="invalid('code')"
          :aria-describedby="describedBy('code')"
        />
        <button type="submit" class="primary">Search</button>
      </div>
      <p id="hint-code" class="hint">CPT, HCPCS or revenue code, such as 99213, J1100 or 0450. Or words from its name, such as office visit.</p>
      <!-- A missing code is said as it appears (role alert): the cursor may
           already be in the box, so moving it there wouldn't read it out. -->
      <p
        v-if="messageFor('code')"
        id="message-code"
        :key="tries"
        :class="invalid('code') ? 'field-error' : 'field-note'"
        :role="noCode ? 'alert' : undefined"
      >
        {{ messageFor('code') }}
      </p>
    </div>

    <fieldset class="narrow">
      <legend>Narrow to a provider (optional)</legend>
      <div v-for="p in providerBoxes" :key="p.box" class="field">
        <label :for="`box-${p.box}`">{{ p.label }}</label>
        <input
          :id="`box-${p.box}`"
          v-model="boxes[p.box]"
          autocomplete="off"
          spellcheck="false"
          :inputmode="p.inputmode"
          @blur="tidy(p.box)"
          :aria-invalid="invalid(p.box)"
          :aria-describedby="describedBy(p.box)"
        />
        <p :id="`hint-${p.box}`" class="hint">{{ p.hint }}</p>
        <p v-if="messageFor(p.box)" :id="`message-${p.box}`" :class="messages[p.box]?.invalid ? 'field-error' : 'field-note'">
          {{ messageFor(p.box) }}
        </p>
      </div>
    </fieldset>
  </form>
</template>
