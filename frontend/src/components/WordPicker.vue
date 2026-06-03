<template>
  <Teleport to="body">
    <div class="dialog-overlay" @click.self="$emit('close')">
      <div class="dialog">
        <div class="dialog-header">
          <h2 class="dialog-title">Ban Words from Flow</h2>
          <button class="dialog-close" @click="$emit('close')">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>

        <div v-if="flow" class="flow-info-card">
          <span class="label">Flow:</span>
          <span class="mono">{{ flow.src_ip }}:{{ flow.src_port }} → {{ flow.dst_ip }}:{{ flow.dst_port }}</span>
        </div>

        <div class="custom-input-row">
          <input
            v-model="customWord"
            class="input flex-1"
            placeholder="Type custom word or regex..."
            @keydown.enter="addCustomWord"
          />
          <button class="btn btn-sm btn-outline" @click="addCustomWord">Add</button>
        </div>

        <div class="words-section">
          <h3>Unique words (not in checker flows)</h3>
          <div class="word-chips">
            <span
              v-for="word in uniqueWords"
              :key="word"
              class="word-chip"
              :class="{ selected: selectedWords.has(word) }"
              @click="toggleWord(word)"
            >
              {{ word }}
            </span>
            <span v-if="uniqueWords.length === 0" class="empty-state">No unique words found</span>
          </div>
        </div>

        <div class="selected-section">
          <h3>Selected for ban ({{ selectedWords.size }})</h3>
          <div class="word-chips">
            <span
              v-for="word in Array.from(selectedWords)"
              :key="word"
              class="word-chip selected"
              @click="toggleWord(word)"
            >
              {{ word }} ×
            </span>
            <span v-if="selectedWords.size === 0" class="empty-state">No words selected</span>
          </div>
        </div>

        <div class="dialog-footer">
          <button class="btn btn-outline" @click="$emit('close')">Cancel</button>
          <button class="btn btn-destructive" :disabled="selectedWords.size === 0" @click="banWords">
            Ban {{ selectedWords.size }} word(s)
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, defineProps, defineEmits } from 'vue'
import type { Flow } from '@/types'

const props = defineProps<{ flow: Flow | null; uniqueWords: string[] }>()
const emit = defineEmits<{ close: []; banWords: [words: string[]] }>()

const selectedWords = ref(new Set<string>())
const customWord = ref('')

function toggleWord(word: string) {
  if (selectedWords.value.has(word)) {
    selectedWords.value.delete(word)
  } else {
    selectedWords.value.add(word)
  }
  selectedWords.value = new Set(selectedWords.value)
}

function addCustomWord() {
  if (!customWord.value.trim()) return
  selectedWords.value.add(customWord.value.trim())
  selectedWords.value = new Set(selectedWords.value)
  customWord.value = ''
}

function banWords() {
  emit('banWords', Array.from(selectedWords.value))
  selectedWords.value.clear()
}
</script>

<style scoped>
.dialog-overlay { position: fixed; inset: 0; background-color: rgba(0,0,0,0.6); backdrop-filter: blur(4px); z-index: 1000; display: flex; align-items: center; justify-content: center; }
.dialog { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 24px; max-width: 700px; width: 95%; max-height: 85vh; overflow-y: auto; box-shadow: 0 20px 60px rgba(0,0,0,0.4); }
.dialog-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.dialog-title { font-size: 20px; font-weight: 600; margin: 0; }
.dialog-close { background: none; border: none; cursor: pointer; padding: 4px; border-radius: 4px; color: var(--muted-foreground); transition: all 0.15s; }
.dialog-close:hover { filter: brightness(1.2); }
.flow-info-card { display: flex; gap: 8px; align-items: center; padding: 12px; border-radius: 8px; background-color: var(--surface); border: 1px solid var(--border); margin-bottom: 16px; }
.custom-input-row { display: flex; gap: 8px; margin-bottom: 16px; }
.custom-input-row .input { flex: 1; }
.words-section, .selected-section { margin-bottom: 16px; }
.words-section h3, .selected-section h3 { font-size: 14px; font-weight: 600; margin: 0 0 8px 0; color: var(--text-muted); }
.word-chips { display: flex; flex-wrap: wrap; gap: 6px; }
.word-chip { padding: 6px 12px; border-radius: 6px; font-size: 13px; cursor: pointer; border: 1px solid var(--border); background-color: var(--surface); color: var(--text); transition: all 0.15s; user-select: none; }
.word-chip:hover { filter: brightness(1.1); }
.word-chip.selected { background-color: var(--destructive); color: var(--destructive-foreground); border-color: var(--destructive); }
.empty-state { padding: 16px; text-align: center; color: var(--text-muted); font-size: 14px; }
.dialog-footer { display: flex; justify-content: flex-end; gap: 8px; padding-top: 16px; border-top: 1px solid var(--border); }
.mono { font-family: 'JetBrains Mono', monospace; }
.label { font-size: 12px; font-weight: 500; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }
.flex-1 { flex: 1; }
</style>
