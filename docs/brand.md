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

## Shape
- Radius: input 4, chip 8, card 12, sheet/dialog 16, button 20 (pill-ish).
- Cards: surface background, `border-subtle` border, no shadow. Status always with a status chip.
- Spacing scale 4/8/12/16/20/24/32.

## Logo
- Use the Lebane isotype (`docs/brand-assets/lebane-isotipo.png`) as app icon base until a
  Lenapay mark exists; wordmark "Lenapay" in Plus Jakarta Sans 700, `primary` color.
