#!/usr/bin/env python3
"""Transforme le jeu d'icônes SVG en un module React.

Les icônes doivent être inlinées pour que `currentColor` suive le thème : en
<img> le SVG est isolé et reste noir. Plutôt que d'ajouter un greffon Vite,
on génère un module unique à partir des fichiers source. Le contrat commun
(viewBox, fill, stroke, épaisseur, extrémités) vit dans le composant ; seuls
les tracés sont stockés, ce qui tient en une dizaine de kilo-octets.

Usage : python3 scripts/generer_icones.py <dossier_svg>
"""
import json
import pathlib
import re
import sys

CONTRAT = {
    'viewBox': '0 0 24 24', 'fill': 'none', 'stroke': 'currentColor',
    'stroke-width': '1.5', 'stroke-linecap': 'butt', 'stroke-linejoin': 'miter',
}


def tracés(svg: str) -> str:
    """Retourne le contenu interne du <svg>, sans le titre."""
    interne = svg[svg.index('>', svg.index('<svg')) + 1:svg.rindex('</svg>')]
    interne = re.sub(r'<title>.*?</title>', '', interne, flags=re.S)
    return ' '.join(interne.split())


def main() -> int:
    src = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else 'assets/icones')
    fichiers = sorted(src.glob('*.svg'))
    if not fichiers:
        print(f'aucun SVG dans {src}', file=sys.stderr)
        return 1

    ecarts = []
    entrees = []
    for f in fichiers:
        svg = f.read_text()
        for cle, valeur in CONTRAT.items():
            if f'{cle}="{valeur}"' not in svg:
                ecarts.append(f'{f.name} : {cle} attendu à "{valeur}"')
        titre = re.search(r'<title>(.*?)</title>', svg, re.S)
        entrees.append((f.stem, titre.group(1).strip() if titre else f.stem, tracés(svg)))

    if ecarts:
        print('contrat graphique non respecté :', file=sys.stderr)
        for e in ecarts:
            print('  ' + e, file=sys.stderr)
        return 1

    lignes = [
        "// Généré par scripts/generer_icones.py — ne pas modifier à la main.",
        "// Source : le jeu d'icônes SVG, contrat 24×24, currentColor, trait 1,5.",
        '',
        'export const ICONES = {',
    ]
    for nom, titre, corps in entrees:
        # json.dumps produit une chaîne JavaScript valide : les guillemets des
        # attributs SVG y sont échappés, ce qu'un simple %r ne garantit pas.
        lignes.append(f'  {json.dumps(nom)}: {{ titre: {json.dumps(titre)}, corps: {json.dumps(corps)} }},')
    lignes += ['}', '']
    pathlib.Path('frontend/src/components/icones.js').write_text('\n'.join(lignes))
    print(f'  {len(entrees)} icônes écrites dans frontend/src/components/icones.js')
    return 0


if __name__ == '__main__':
    sys.exit(main())
