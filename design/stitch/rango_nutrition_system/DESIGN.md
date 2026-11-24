---
name: Rango Nutrition System
colors:
  surface: '#f8f9ff'
  surface-dim: '#cbdbf5'
  surface-bright: '#f8f9ff'
  surface-container-lowest: '#ffffff'
  surface-container-low: '#eff4ff'
  surface-container: '#e5eeff'
  surface-container-high: '#dce9ff'
  surface-container-highest: '#d3e4fe'
  on-surface: '#0b1c30'
  on-surface-variant: '#3e4a3d'
  inverse-surface: '#213145'
  inverse-on-surface: '#eaf1ff'
  outline: '#6e7b6c'
  outline-variant: '#bdcaba'
  surface-tint: '#006e2d'
  primary: '#006b2c'
  on-primary: '#ffffff'
  primary-container: '#00873a'
  on-primary-container: '#f7fff2'
  inverse-primary: '#62df7d'
  secondary: '#006c49'
  on-secondary: '#ffffff'
  secondary-container: '#6cf8bb'
  on-secondary-container: '#00714d'
  tertiary: '#994100'
  on-tertiary: '#ffffff'
  tertiary-container: '#c05400'
  on-tertiary-container: '#fffbff'
  error: '#ba1a1a'
  on-error: '#ffffff'
  error-container: '#ffdad6'
  on-error-container: '#93000a'
  primary-fixed: '#7ffc97'
  primary-fixed-dim: '#62df7d'
  on-primary-fixed: '#002109'
  on-primary-fixed-variant: '#005320'
  secondary-fixed: '#6ffbbe'
  secondary-fixed-dim: '#4edea3'
  on-secondary-fixed: '#002113'
  on-secondary-fixed-variant: '#005236'
  tertiary-fixed: '#ffdbca'
  tertiary-fixed-dim: '#ffb690'
  on-tertiary-fixed: '#341100'
  on-tertiary-fixed-variant: '#783200'
  background: '#f8f9ff'
  on-background: '#0b1c30'
  surface-variant: '#d3e4fe'
typography:
  display-lg:
    fontFamily: Plus Jakarta Sans
    fontSize: 36px
    fontWeight: '800'
    lineHeight: 44px
  display-lg-mobile:
    fontFamily: Plus Jakarta Sans
    fontSize: 28px
    fontWeight: '800'
    lineHeight: 36px
  headline-lg:
    fontFamily: Plus Jakarta Sans
    fontSize: 24px
    fontWeight: '700'
    lineHeight: 32px
  headline-md:
    fontFamily: Plus Jakarta Sans
    fontSize: 20px
    fontWeight: '700'
    lineHeight: 28px
  headline-sm:
    fontFamily: Plus Jakarta Sans
    fontSize: 18px
    fontWeight: '600'
    lineHeight: 24px
  body-lg:
    fontFamily: Plus Jakarta Sans
    fontSize: 16px
    fontWeight: '400'
    lineHeight: 24px
  body-md:
    fontFamily: Plus Jakarta Sans
    fontSize: 14px
    fontWeight: '400'
    lineHeight: 20px
  body-sm:
    fontFamily: Plus Jakarta Sans
    fontSize: 12px
    fontWeight: '400'
    lineHeight: 16px
  label-lg:
    fontFamily: Plus Jakarta Sans
    fontSize: 14px
    fontWeight: '600'
    lineHeight: 20px
  label-md:
    fontFamily: Plus Jakarta Sans
    fontSize: 12px
    fontWeight: '600'
    lineHeight: 16px
  label-sm:
    fontFamily: Plus Jakarta Sans
    fontSize: 11px
    fontWeight: '700'
    lineHeight: 14px
  data-metric:
    fontFamily: Plus Jakarta Sans
    fontSize: 22px
    fontWeight: '800'
    lineHeight: 26px
rounded:
  sm: 0.5rem
  DEFAULT: 1rem
  md: 1.5rem
  lg: 2rem
  xl: 3rem
  full: 9999px
spacing:
  gutter: 1rem
  margin: 1.25rem
  space-xs: 0.25rem
  space-sm: 0.5rem
  space-md: 1rem
  space-lg: 1.5rem
  space-xl: 2rem
---

## Brand & Style

The design system establishes a welcoming, vibrant, and approachable digital companion for daily food logging and nutritional awareness. Designed specifically for effortless interaction throughout daily routines, the aesthetic prioritizes clarity, human warmth, and frictionless input.

### Personality & Emotional Response
- **Vibrant & Vital:** Evokes the crisp freshness of whole foods, balanced health, and renewed daily energy without clinical severity.
- **Empowering & Non-Judgmental:** Encourages habit-building through positive reinforcement, soft organic shapes, and calm organization rather than punitive dietary restrictions.
- **Frictionless & Accessible:** Built for one-handed mobile utility during cooking, grocery shopping, or dining out.

### Visual Style
The system adopts an ultra-clean, modern soft-geometry style. It pairs generous pill-shaped geometry, warm paper-white surfaces, and tactile low-contrast borders with a lush garden-inspired primary green. Macro-nutrients receive intuitive, high-clarity color coding designed to make meal breakdowns instant and unmistakable at a glance.

## Colors

The palette balances clean visual hygiene with appetizing chromatic signals. Surfaces remain calm and airy to let meal photography and nutrition data stand out.

### Primary & Functional Roles
- **Primary (`#16A34A`):** Leaf Green. Drives focal interactive elements, primary CTAs, completion badges, and active state indicators.
- **Carbohydrates / Secondary (`#10B981`):** Vibrant Emerald. Highlights carbohydrate intake and steady metabolic fuel metrics.
- **Proteins / Tertiary (`#F97316`):** Warm Coral Orange. Distinct visual signifier for dietary protein tracking, strength milestones, and key nutritional targets.
- **Fats (`#F59E0B`):** Warm Honey Amber. Dedicated semantic role for dietary lipids and balanced energy storage metrics.
- **Neutral Core (`#64748B`):** Cool Slate. Drives secondary metadata, subtle borders (`#E2E8F0`), and readable text hierarchy (`#0F172A` for primary headlines, `#334155` for body copy).
- **Canvas Base (`#F8FAFC` to `#F1F5F9`):** Soft luminous ground that avoids eye fatigue while making `#FFFFFF` card containers pop cleanly.

## Typography

The typographic hierarchy utilizes Plus Jakarta Sans throughout for maximum coherence and open letterforms.

### Usage Guidance
- **Display & Headlines:** Set with tighter tracking (-0.02em) and heavy weights (700/800) to anchor meal periods (Café da Manhã, Almoço, Jantar) with clarity.
- **Data Metric Level:** Specifically assigned to calorie counts and macro gram values (`g`), paired with bold numerals to prevent cognitive strain during fast scanning.
- **Body & Labels:** Generous line heights ensure readability under motion or direct sunlight when logging food outdoors.

## Layout & Spacing

The layout model is optimized primarily for one-handed mobile thumb access on handheld viewports, scaling neatly to larger viewports.

### Mobile Grid & Ergonomic Thumb Zones
- **Columns:** 4-column fluid mobile grid scaling to 8 columns on tablet.
- **Thumb Zone Priority:** Critical interactive components (Quick Log button, barcode scanner trigger, meal switcher) anchor to the bottom 40% of the viewport.
- **Canvas Bounds:** `margin: 1.25rem` keeps touch targets safely inward from edge-swipe gestures and bezel boundaries.
- **Rhythm:** Inter-card vertical spacing strictly adheres to `space-md` (1rem), grouping food entries into distinct temporal meal intervals.

## Elevation & Depth

Visual depth is communicated through ambient diffuse shadows combined with structural hair-line surface borders, preserving an uncluttered paper-stack feel.

### Elevation Hierarchy
- **Level 0 (Canvas):** `#F8FAFC`, flat ground layer.
- **Level 1 (Card & Section Containers):** Pure `#FFFFFF` fill with a subtle 1px border (`#E2E8F0`) and ambient shadow: `box-shadow: 0 2px 8px -2px rgba(15, 23, 42, 0.05)`.
- **Level 2 (Interactive Floating Controls / Action Bar):** Elevated bottom sheets and bottom navigation surfaces: `box-shadow: 0 8px 24px -4px rgba(15, 23, 42, 0.08)`.
- **Level 3 (Modal / Quick Add Radial Menu):** Top-tier overlays with soft backdrop blur: `backdrop-filter: blur(8px)` paired with `box-shadow: 0 16px 32px -8px rgba(15, 23, 42, 0.14)`.

## Shapes

The shape system embraces pill-shaped and deeply curved geometries (`roundedness: 3`).

### Geometry Strategy
- **Small Elements (Pills, Chips, Tags):** Full pill rounding (`border-radius: 9999px`) to create tactile, easy-to-tap touch areas.
- **Cards & Summary Modules:** `rounded-lg` (2rem / 32px) to evoke approachable softness and avoid severe visual angles.
- **Action Sheets & Drawers:** `rounded-xl` (3rem / 48px) along top edges for a smooth, organic slide-up feel.

## Components

### Buttons
- **Primary Action Button:** Fully rounded pill (`border-radius: 9999px`), min-height 52px for optimal thumb target. Background `#16A34A`, text `#FFFFFF`, font `label-lg`.
- **Secondary Action Button:** Surface `#F1F5F9`, text `#0F172A`, full pill shape with 1px border `#E2E8F0`.
- **Floating Action Log Button ("Adicionar"):** 60px circle or broad pill, high contrast, anchored bottom-center.

### Chips & Meal Tags
- **Meal Chips (Café, Almoço, Lanche, Jantar):** Pill shape, height 36px, `space-md` horizontal padding. Selected: `#16A34A` background with `#FFFFFF` text. Unselected: `#FFFFFF` surface with `#E2E8F0` border and `#64748B` text.

### Macro Progress Indicators
- Linear and circular meter rings with 8px stroke weight and rounded stroke caps.
- Tracks set to 12% opacity of their respective macro color; indicators use 100% solid fills (`#F97316` Proteína, `#10B981` Carboidrato, `#F59E0B` Gordura).

### Lists & Meal Entry Rows
- Row height: 64px minimum.
- Left-aligned icon or food avatar (44px circle).
- Title in `label-lg`, calorie count in `label-md` right-aligned, macro breakdown dots beneath in `body-sm`.
- Swipe actions: left-swipe reveals soft red delete pill, right-swipe duplicates item.

### Input Fields & Quick Search
- Height 52px, full pill enclosure with 16px horizontal internal padding.
- Background `#FFFFFF`, 1.5px border `#E2E8F0`, transitioning to `#16A34A` on focus with no harsh outer glow.
- Left icon slot for search glass; right slot for barcode scan trigger.

### Checkboxes & Segmented Controls
- Checkboxes: 24px rounded squares (8px radius) with `#16A34A` checked state and white checkmark icon.
- Segmented Unit Switchers (g / ml / oz): Housed inside a `#F1F5F9` pill container; active tab slides as an elevated white pill.