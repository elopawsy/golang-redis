# Présentation de soutenance

`WasmRedis-soutenance.pptx` — 42 diapositives, format 16/9.
`WasmRedis-soutenance.pdf` — la même chose, pour un vidéoprojecteur sans PowerPoint.

## Plan

| Diapositives | Partie |
| --- | --- |
| 1 – 5 | Le projet, le périmètre, l'architecture, le modèle de données |
| 6 – 18 | Que se passe-t-il quand on fait un `SET` — les sept étapes, puis la trace réelle |
| 19 – 26 | Lire, chercher, oublier : `GET`, `DELETE`, `GET WHERE`, TTL |
| 27 – 32 | Survivre à un redémarrage : snapshot, écriture atomique, rejeu, pannes |
| 33 – 42 | API HTTP, tableau de bord, vérifications, limites, récapitulatif |

## Régénérer

Les extraits de code sont **lus dans les fichiers du dépôt par numéro de ligne**,
puis dessinés en PNG. La présentation ne peut donc pas diverger du code —
mais si le code bouge, il faut vérifier que les plages de lignes pointent
toujours au bon endroit.

```bash
python3 -m venv /tmp/deckenv
/tmp/deckenv/bin/pip install python-pptx Pillow pygments
/tmp/deckenv/bin/python presentation/build.py
```

`build.py` se lance depuis la racine du dépôt ou depuis `presentation/` :
les chemins sont résolus à partir de l'emplacement du script.

Les captures d'écran de `img/` (`ui-dashboard.png`, `ui-liste.png`) ont été
prises avec Playwright sur `go run . -demo -memory`. Elles ne sont pas
régénérées par `build.py`.
