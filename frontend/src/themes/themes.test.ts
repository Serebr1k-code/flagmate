import { describe, it, expect } from 'vitest'
import { themes } from '@/themes'

describe('Themes', () => {
  it('should have 11 themes', () => {
    expect(Object.keys(themes).length).toBe(11)
  })

  it('should have all required color properties for each theme', () => {
    const requiredColors = [
      'background', 'surface', 'surfaceHover', 'border', 'text', 'textMuted',
      'primary', 'primaryHover', 'primaryForeground', 'secondary', 'secondaryForeground',
      'accent', 'accentForeground', 'destructive', 'destructiveForeground',
      'muted', 'mutedForeground', 'success', 'successForeground',
      'warning', 'warningForeground', 'input', 'ring', 'card', 'cardForeground',
      'popover', 'popoverForeground'
    ]

    for (const [name, theme] of Object.entries(themes)) {
      for (const color of requiredColors) {
        expect(theme.colors).toHaveProperty(color)
        expect(typeof theme.colors[color]).toBe('string')
        expect(theme.colors[color].length).toBeGreaterThan(0)
      }
    }
  })

  it('should have unique names for all themes', () => {
    const names = Object.values(themes).map(t => t.name)
    const uniqueNames = new Set(names)
    expect(names.length).toBe(uniqueNames.size)
  })
})
