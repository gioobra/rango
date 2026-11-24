---
name: Rango Dark Experience
colors:
  surface: '#111316'
  surface-dim: '#111316'
  surface-bright: '#37393d'
  surface-container-lowest: '#0c0e11'
  surface-container-low: '#1a1c1f'
  surface-container: '#1e2023'
  surface-container-high: '#282a2d'
  surface-container-highest: '#333538'
  on-surface: '#e2e2e6'
  on-surface-variant: '#bccbb9'
  inverse-surface: '#e2e2e6'
  inverse-on-surface: '#2f3034'
  outline: '#869585'
  outline-variant: '#3d4a3d'
  surface-tint: '#4ae176'
  primary: '#4be277'
  on-primary: '#003915'
  primary-container: '#22c55e'
  on-primary-container: '#004b1e'
  inverse-primary: '#006e2f'
  secondary: '#4edea3'
  on-secondary: '#003824'
  secondary-container: '#00a572'
  on-secondary-container: '#00311f'
  tertiary: '#ffb784'
  on-tertiary: '#4f2500'
  tertiary-container: '#fa913b'
  on-tertiary-container: '#663100'
  error: '#ffb4ab'
  on-error: '#690005'
  error-container: '#93000a'
  on-error-container: '#ffdad6'
  primary-fixed: '#6bff8f'
  primary-fixed-dim: '#4ae176'
  on-primary-fixed: '#002109'
  on-primary-fixed-variant: '#005321'
  secondary-fixed: '#6ffbbe'
  secondary-fixed-dim: '#4edea3'
  on-secondary-fixed: '#002113'
  on-secondary-fixed-variant: '#005236'
  tertiary-fixed: '#ffdcc5'
  tertiary-fixed-dim: '#ffb783'
  on-tertiary-fixed: '#301400'
  on-tertiary-fixed-variant: '#713700'
  background: '#111316'
  on-background: '#e2e2e6'
  surface-variant: '#333538'
typography:
  display-lg:
    fontFamily: Plus Jakarta Sans
    fontSize: 3rem
    fontWeight: '700'
    lineHeight: 3.5rem
    letterSpacing: -0.02em
  display-lg-mobile:
    fontFamily: Plus Jakarta Sans
    fontSize: 2.25rem
    fontWeight: '700'
    lineHeight: 2.75rem
    letterSpacing: -0.015em
  headline-lg:
    fontFamily: Plus Jakarta Sans
    fontSize: 2rem
    fontWeight: '700'
    lineHeight: 2.5rem
    letterSpacing: -0.01em
  headline-lg-mobile:
    fontFamily: Plus Jakarta Sans
    fontSize: 1.5rem
    fontWeight: '700'
    lineHeight: 2rem
  headline-md:
    fontFamily: Plus Jakarta Sans
    fontSize: 1.25rem
    fontWeight: '600'
    lineHeight: 1.75rem
  headline-sm:
    fontFamily: Plus Jakarta Sans
    fontSize: 1.125rem
    fontWeight: '600'
    lineHeight: 1.5rem
  body-lg:
    fontFamily: Plus Jakarta Sans
    fontSize: 1rem
    fontWeight: '400'
    lineHeight: 1.5rem
  body-md:
    fontFamily: Plus Jakarta Sans
    fontSize: 0.875rem
    fontWeight: '400'
    lineHeight: 1.25rem
  body-sm:
    fontFamily: Plus Jakarta Sans
    fontSize: 0.75rem
    fontWeight: '400'
    lineHeight: 1rem
  label-lg:
    fontFamily: Plus Jakarta Sans
    fontSize: 0.875rem
    fontWeight: '600'
    lineHeight: 1.25rem
  label-md:
    fontFamily: Plus Jakarta Sans
    fontSize: 0.75rem
    fontWeight: '600'
    lineHeight: 1rem
  label-sm:
    fontFamily: Plus Jakarta Sans
    fontSize: 0.6875rem
    fontWeight: '700'
    lineHeight: 0.875rem
    letterSpacing: 0.04em
rounded:
  sm: 0.25rem
  DEFAULT: 0.5rem
  md: 0.75rem
  lg: 1rem
  xl: 1.5rem
  full: 9999px
spacing:
  gutter: 1rem
  gutter-desktop: 1.5rem
  margin: 1rem
  margin-desktop: 2rem
  space-xs: 0.25rem
  space-sm: 0.5rem
  space-md: 1rem
  space-lg: 1.5rem
  space-xl: 2rem
  space-2xl: 3rem
---

## Brand & Style

This design system delivers a focused, performance-driven dark theme for nutritional tracking and dietary health. It rejects harsh pure black in favor of deep graphite tones, offering a sophisticated nocturnal canvas that reduces eye strain while celebrating vibrant food imagery and metabolic metrics. 

The aesthetic is Modern Functionalist with high-clarity data accents. High-contrast leaf greens indicate progress, balance, and health achievements, while warm citrus accents demarcate caloric targets and energetic intake. The UI feels disciplined, motivating, and luminous—elevating routine daily logs into a premium health dashboard.

## Colors

The palette relies on layered carbon neutrals and high-energy bioluminescent accents calibrated specifically for dark surfaces:

- **Canvas & Backgrounds:** Base canvas uses deep graphite (`#121417`), grounding the interface without the harsh contrast of OLED black. Base cards and standard containers sit at `#1A1F26` with interactive containers at `#222933`. Elevated menus and sheets rise to `#2A323D`.
- **Primary Green (`#22C55E`):** The signature leaf green, utilized for primary calls-to-action, completed macro indicators, and verified nutritional items. In text and dense icons against dark containers, dynamic shifts allow `#4ADE80` for elevated accessibility.
- **Secondary Green (`#10B981`):** A cooler emerald accent dedicated to supplementary dietary metrics, water intake, and secondary micro-interactions.
- **Tertiary Accent (`#FB923C`):** A warm citrus amber reserved for energy, active burn, and caloric allocation meters.
- **Typography & Details:** Primary text reads crisp at `#F1F5F9` (slate 100). Secondary metadata and supportive labels rest at `#94A3B8` (slate 400). Structural boundaries use low-contrast borders at `#2E3846`.

## Typography

The type hierarchy uses Plus Jakarta Sans across all layers. Its rounded, open curves balance technical precision with biological warmth, humanizing numbers and nutritional tables.

- **Numerics & Readouts:** Macro numbers and daily caloric totals employ bold weights with tight tracking (`-0.02em`) to deliver instant legibility during meal registration.
- **Hierarchy:** High-emphasis headlines utilize bold and semibold weights (`700`, `600`) in primary slate (`#F1F5F9`). Secondary body copy and macronutrient labels sit strictly at regular and medium weights in muted slate (`#94A3B8`).
- **Micro-Badges:** Uppercase tags (e.g., "KCAL", "PROTEÍNA", "CARBOS") use `label-sm` with expanded tracking (`0.04em`) to maintain sharp legibility at compact dimensions.

## Layout & Spacing

The interface applies a strict 4px/8px modular spacing framework:

- **Mobile Viewports (<768px):** 4-column fluid layout with `1rem` margins and `1rem` gutters. Content containers prioritize vertical stacked cards for timeline logging and quick food additions.
- **Desktop/Tablet Viewports (≥768px):** 12-column layout with `2rem` outer margins. Content groups into modular split-panes: meal diary and progress rings on the left; macro distribution, pantry access, and analytical charts on the right.
- **Interior Gaps:** Component clusters (e.g., a dish name and its calorie pill) employ `space-xs` and `space-sm`. Inner card padding adheres consistently to `space-md` (`1rem`) on compact views and `space-lg` (`1.5rem`) on larger dashboard summaries.

## Elevation & Depth

In this dark environment, spatial elevation is created through tonal layering and delicate borders rather than heavy drop shadows:

- **Level 0 (Canvas):** `#121417` — Ground floor. No border, no elevation.
- **Level 1 (Default Cards & Meal Blocks):** `#1A1F26` surface bounded by a subtle `1px` stroke in `#2E3846`. Drop shadows are suppressed or replaced with a 1px inner ambient border highlight (`rgba(255, 255, 255, 0.04)`).
- **Level 2 (Interactive Cards & Inputs):** `#222933` with `#2E3846` border. On hover or active state, the border lights up to `#22C55E` or `#4ADE80` at 40% opacity.
- **Level 3 (Modals, Overlays & Floating Navigation):** `#2A323D` surface backed by an ultra-diffused atmospheric shadow (`0 20px 40px -15px rgba(0, 0, 0, 0.7)`). Floating buttons cast a directional glow tinted with the primary seed color (`0 8px 24px -4px rgba(34, 197, 94, 0.25)`).

## Shapes

The shape scale maintains strict consistency with the product identity, using rounded level `2`:

- **Standard Cards & Containers:** `1rem` (`rounded-lg`) corner radii create comfortable, friendly containment for food entries and summary widgets.
- **Buttons & Input Fields:** `0.5rem` (`rounded-md` / base roundedness) maintains structural control and touch-target definition.
- **Progress Badges & Macro Indicators:** Pill-shaped (`9999px`) geometries are strictly reserved for dietary status tags, macro chips, and floating action button capsules.
- **Sheets & Modal Surfaces:** `1.5rem` (`rounded-xl`) on top edges to deliver a soft physical tuck at screen edges.

## Components

### Buttons
- **Primary:** Solid leaf green (`#22C55E`) background with dark contrast text (`#121417`). Rounded corner radius of `0.5rem`. Subtle green bloom shadow on hover.
- **Secondary:** Surface `#222933` with `#2E3846` border and `#F1F5F9` text. Hover shifts background to `#2A323D` with primary border tint.
- **Ghost/Tertiary:** Transparent background, `#94A3B8` icon/text, shifting to `#F1F5F9` on active press.

### Cards & Meal Tiles
- Constructed on `#1A1F26` with `1px` solid `#2E3846` border and `1rem` border radius.
- Meal entries divide using subtle `#2E3846` horizontal divider lines. Right-aligned nutritional values use bold `#F1F5F9` typography.

### Input Fields & Search Bars
- Background set to `#1A1F26` with resting border in `#2E3846` and rounded radius of `0.5rem`.
- Placeholder text in `#94A3B8`. Focus rings apply a `2px` crisp border in `#22C55E` without harsh offsets.

### Progress Bars & Macro Rings
- **Track Base:** Dark graphite trough `#222933`.
- **Calorie Bar Fill:** Citrus orange (`#FB923C`) to denote dynamic daily energy.
- **Protein, Carb, Fat Meters:** Leaf green (`#22C55E`), emerald (`#10B981`), and soft cyan-accent fills with smooth rounded caps.

### Chips & Filter Tags
- Fully rounded pills (`9999px`) with padding `0.375rem 0.75rem`. Unselected chips use `#1A1F26` with `#2E3846` border. Active chips feature a `10%` green tint fill (`rgba(34, 197, 94, 0.12)`) and bright `#4ADE80` text with `#22C55E` border.

### Checkboxes & Selection Controls
- Base box has `0.375rem` radius with `#2E3846` border on `#1A1F26`.
- Checked state switches fill to `#22C55E` with `#121417` checkmark icon.