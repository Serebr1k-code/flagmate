import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { themes, type ThemeName } from '@/themes'

export const useThemeStore = defineStore('theme', () => {
  const currentTheme = ref<ThemeName>(localStorage.getItem('theme') as ThemeName || 'midnight')

  function applyTheme(name: ThemeName) {
    const theme = themes[name]
    if (!theme) return

    const root = document.documentElement
    const colors = theme.colors

    root.style.setProperty('--background', colors.background)
    root.style.setProperty('--surface', colors.surface)
    root.style.setProperty('--surface-hover', colors.surfaceHover)
    root.style.setProperty('--border', colors.border)
    root.style.setProperty('--text', colors.text)
    root.style.setProperty('--text-muted', colors.textMuted)
    root.style.setProperty('--primary', colors.primary)
    root.style.setProperty('--primary-hover', colors.primaryHover)
    root.style.setProperty('--primary-foreground', colors.primaryForeground)
    root.style.setProperty('--secondary', colors.secondary)
    root.style.setProperty('--secondary-foreground', colors.secondaryForeground)
    root.style.setProperty('--accent', colors.accent)
    root.style.setProperty('--accent-foreground', colors.accentForeground)
    root.style.setProperty('--destructive', colors.destructive)
    root.style.setProperty('--destructive-foreground', colors.destructiveForeground)
    root.style.setProperty('--muted', colors.muted)
    root.style.setProperty('--muted-foreground', colors.mutedForeground)
    root.style.setProperty('--success', colors.success)
    root.style.setProperty('--success-foreground', colors.successForeground)
    root.style.setProperty('--warning', colors.warning)
    root.style.setProperty('--warning-foreground', colors.warningForeground)
    root.style.setProperty('--input', colors.input)
    root.style.setProperty('--ring', colors.ring)
    root.style.setProperty('--card', colors.card)
    root.style.setProperty('--card-foreground', colors.cardForeground)
    root.style.setProperty('--popover', colors.popover)
    root.style.setProperty('--popover-foreground', colors.popoverForeground)

    currentTheme.value = name
    localStorage.setItem('theme', name)
  }

  watch(currentTheme, (name) => applyTheme(name))

  applyTheme(currentTheme.value)

  return {
    currentTheme,
    themes,
    applyTheme
  }
})
