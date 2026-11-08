# Lenapay — Brand reference

Lenapay uses the Lebane visual identity (same palette, typography and radii as the Lebane ERP
and the Lebane Payments admin). Code uses tokens only (M9/ME15); hex values live in one place:
`apps/mobile/global.css` `@theme`.

## Name and voice
- Product name: **Lenapay** (one word, capital L). Tagline idea: "Tus cobros y pagos, al instante."
- Voice: es-AR, "vos", short and clear, no jargon ("Te transfirieron $ 150.000 al CVU de Juan").
- Money: `$ 1.500,00` (ARS), `US$ 1.500,00` (USD); minor units → `Intl.NumberFormat('es-AR')`.

## Typography
- Plus Jakarta Sans 400 body, 600 titles, 700 only for big balances.
- JetBrains Mono for CVU, CBU, alias and CUIT.
- Scale (pt): 12 / 14 / 16 / 18 / 20 / 24 / 32 (balance).

## Color tokens

| Token | Light | Dark | Use |
|---|---|---|---|
| `primary` | #125BFF | #296BFE | main actions, links |
| `primary-hover` / `primary-active` | #296BFE / #0546D8 | #4A82FF / #125BFF | pressed states |
| `primary-soft` | #F5F7FF | #10204A | selected backgrounds |
| `primary-bg` | #E2E8FE | #16285C | chips, highlights |
| `bg-page` | #F4F6F8 | #0E1015 | screen background |
| `bg-surface` | #FFFFFF | #181A21 | cards, sheets |
| `text-primary` | #181A21 | #F4F6F8 | main text |
| `text-secondary` | #505664 | #CCD2DB | secondary text |
| `text-muted` | #656E83 | #939BAE | captions |
| `text-disabled` | #888E9E | #656E83 | disabled |
| `border-default` / `border-subtle` / `border-strong` | #CCD2DB / #E6EBEF / #939BAE | #394050 / #262A33 / #656E83 | borders |
| `success` / `success-bg` | #0BA85C / #E2FDF0 | #0ED776 / #0B3322 | settled, credited |
| `warning` / `warning-bg` | #F5AD03 / #FFF4D9 | #F5C34D / #3A2C05 | pending, held |
| `danger` / `danger-bg` | #FE3D51 / #FEE8E7 | #F9948F / #3D1216 | failed, rejected |
| `accent` | #FC8846 | #F99165 | secondary highlights |
| `currency-ars` / `currency-usd` | #89CFF0 / #90EE90 | same | currency chips |
| `hero-from` / `hero-to` | #1F5FF5 / #0A3FC2 | #1A56E0 / #0A2B85 | balance hero gradient (white text ≥ 5:1) |
| `hero-sheen` / `hero-tile` | white 12 % / ink 28 % | white 8 % / ink 35 % | glossy circles and inner tiles on the hero |

## Shape
- Radius: input 4, chip 8 (status chips are full pills), card 16, sheet/dialog 24, button 20 (pill-ish).
- Cards: surface background, `border-subtle` border and elevation `md` (see below). Status always
  with a status chip on a soft tinted background, never colour alone.
- Spacing scale 4/8/12/16/20/24/32.

## Elevation
Depth comes from layered, ink-tinted shadows in light mode. In dark mode an ink shadow alone does
not read on the #0E1015 page, so every dark level combines four cues: a surface one step lighter
per level (`surface-1/2/3`), a 1 px top highlight (white 8–12 %), a hairline outer ring (white
3–6 %) and a deep black drop; primary and hero elements get a primary glow at ~35 % with a large
blur, which does read on dark. The tokens are the `--shadow-*` entries of
`apps/mobile/global.css` `@theme` and the `--color-surface-*` colours; code reads them ONLY
through `src/shared/ui/elevation.ts` (`elevation(level)`, `SURFACE_TONE`, `FOCUS_RING`,
`pressedScale`), which pairs every level with its dark replacement. A spec fails if any other
file writes shadow classes or shadow styles. Uniwind turns the classes into the `boxShadow`
style, which React Native 0.88 draws on iOS and Android (new architecture; Android 9+) and
react-native-web on the web — no `shadowColor`/`elevation` pairs.

| Level | Light token | Dark shadow | Dark surface (`SURFACE_TONE`) | Use |
|---|---|---|---|---|
| `sm` | `shadow-sm` | `shadow-edge` (8 % highlight, 3 % ring, short drop) | — (component's own) | secondary buttons, segmented thumb, filter pills, small round actions |
| `md` | `shadow-md` | `shadow-edge-md` (10 % highlight, 4 % ring, 24 px drop) | `surface-1` #1B1E26 | cards and pressable cards (pressed: sinks to `sm`) |
| `lg` | `shadow-lg` | `shadow-edge-strong` (12 % highlight, 6 % ring, 40 px drop) | `surface-2` #222631 | toasts, floating badges |
| `xl` | `shadow-xl` | `shadow-edge-strong` | `surface-3` #2A2F3B | sheets and dialogs (over a blurred backdrop, expo-blur) |
| `bar` | `shadow-bar` (upward) | `shadow-edge-strong` | `surface-2` | the floating tab bar: a blurred capsule inset 16 dp from the sides, 8 dp above the home indicator (solid surface on Android) |
| `glow` | `shadow-glow` (primary-tinted) | `shadow-glow-dark` (#296BFE at 35 %, 36 px blur) | — | primary buttons, quick actions, the balance hero, the selected tab's liquid pill (its leading edge springs fast, the trailing edge follows softly, so it stretches and contracts; Reduce Motion jumps) |
| focus | `shadow-focus` (4 px primary ring at 16 %) | same | — | focused inputs, with the `primary` border |

- Light surfaces stay white (`surface-1/2/3` = #FFFFFF); the light shadows are ~30 % stronger
  than v2 so cards clearly lift off the #F4F6F8 page.
- Hero glass tiles (Retenido / Total) are not elevated: `hero-glass` fill, `hero-glass-border`
  hairline and a `hero-track` progress bar, all translucent white on the gradient.
- Pressed controls scale to 0.97 and their shadow drops one level (`glow` → `glow-pressed`).
- Inactive (disabled) buttons are flat: no elevation at 50 % opacity.
- Lists: rows of a virtualized list are drawn as ONE grouped card (`groupedRowClass`), bordered,
  without per-row shadows (stacked shadows leave seams).
- Do not stack elevated surfaces inside elevated surfaces; an inner group uses border or tint.

## App icon and splash
Generated from `apps/mobile/assets/brand/` (Lebane files) by a one-off script (System.Drawing):
- `assets/icon.png` 1024² opaque: Lebane isotype in its blue (#296BFF), 600 px, centred on white.
- `assets/adaptive-icon.png` 1024² transparent, isotype 440 px (inside the 66 % safe zone) over
  `backgroundColor` #FFFFFF; `assets/adaptive-icon-monochrome.png` for Android 13 themed icons.
- `assets/splash-icon.png` 512² transparent isotype, `imageWidth` 112, background #FFFFFF / #0E1015.
- `assets/favicon.png` 48² and `assets/logo/*` (@1x/@2x/@3x) for in-app marks.

## Logo
- Lebane is the parent brand; the product name stays **Lenapay**. The mark is the Lebane isotype
  next to the wordmark "Lenapay" in Plus Jakarta Sans 700, `primary` colour
  (`src/shared/ui/Wordmark.tsx`), on Welcome, Login, Unlock and the Home header.
- On Welcome the mark carries a small "by" + Lebane wordmark below it: `lebane-wordmark-on-light`
  (dark letters) in light mode and `lebane-wordmark-on-dark` (white letters) in dark mode,
  switched with `dark:` classes.
