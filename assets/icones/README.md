# Ibalya Icon System

31 icônes SVG cohérentes pour l’interface Ibalya.

## Contrat graphique

- `viewBox="0 0 24 24"`
- `fill="none"`
- `stroke="currentColor"`
- `stroke-width="1.5"`
- `stroke-linecap="butt"`
- `stroke-linejoin="miter"`
- monochrome, sans dégradé, sans ombre, sans couleur en dur
- dessin pensé pour rester lisible à 16 px et 20 px

## Usage

```html
<img src="/icons/nav-synthese.svg" width="16" height="16" alt="" />
```

Pour bénéficier de `currentColor` directement depuis CSS, le plus robuste est d’inliner le SVG (ou de l’importer comme composant SVG dans le bundler) plutôt que de l’utiliser comme image externe.

## Animation de `divers-analyse-en-cours`

Le glyphe est symétrique autour du centre et conçu pour une rotation continue :

```css
@keyframes ibalya-spin { to { transform: rotate(360deg); } }
.ibalya-spin { animation: ibalya-spin 900ms linear infinite; transform-origin: center; }
```

## Catégories

- Navigation : 10
- Actions : 8
- Détecteurs produit : 5
- États : 4
- Divers : 4
