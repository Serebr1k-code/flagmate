<template>
  <div class="theme-switcher" ref="switcherRef">
    <button @click="toggleDropdown" class="theme-btn">
      <span class="theme-dot" :style="{ backgroundColor: currentThemeColors.primary }"></span>
      {{ currentTheme.name }}
      <svg class="chevron" :class="{ open: isOpen }" width="16" height="16" viewBox="0 0 16 16" fill="none">
        <path d="M4 6l4 4 4-4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    </button>

    <div v-if="isOpen" class="theme-dropdown">
      <div
        v-for="(theme, name) in themes"
        :key="name"
        @click="selectTheme(name as ThemeName)"
        class="theme-option"
        :class="{ active: currentThemeName === name }"
      >
        <span class="theme-dot" :style="{ backgroundColor: theme.colors.primary }"></span>
        {{ theme.name }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useThemeStore } from '@/stores/theme'
import { themes, type ThemeName } from '@/themes'

const themeStore = useThemeStore()
const isOpen = ref(false)
const switcherRef = ref<HTMLElement | null>(null)

const currentThemeName = computed(() => themeStore.currentTheme)
const currentTheme = computed(() => themes[currentThemeName.value])
const currentThemeColors = computed(() => currentTheme.value.colors)

function toggleDropdown() {
  isOpen.value = !isOpen.value
}

function selectTheme(name: ThemeName) {
  themeStore.applyTheme(name)
  isOpen.value = false
}

function handleClickOutside(event: MouseEvent) {
  if (switcherRef.value && !switcherRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.theme-switcher { position: relative; display: inline-block; }
.theme-btn { display: flex; align-items: center; gap: 8px; padding: 8px 12px; border: 1px solid var(--border); border-radius: 8px; cursor: pointer; font-size: 14px; font-weight: 500; transition: all 0.2s; background-color: var(--surface); color: var(--text); }
.theme-btn:hover { filter: brightness(1.1); }
.theme-dot { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; }
.chevron { transition: transform 0.2s; margin-left: 4px; }
.chevron.open { transform: rotate(180deg); }
.theme-dropdown { position: absolute; top: calc(100% + 4px); left: 0; z-index: 1000; background-color: var(--popover); border: 1px solid var(--border); border-radius: 8px; padding: 4px; box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3); min-width: 180px; }
.theme-option { display: flex; align-items: center; gap: 8px; padding: 8px 12px; border-radius: 6px; cursor: pointer; font-size: 14px; transition: all 0.15s; color: var(--text); }
.theme-option:hover { filter: brightness(1.1); background-color: var(--surface-hover); }
.theme-option.active { font-weight: 600; background-color: var(--surface-hover); }
</style>
