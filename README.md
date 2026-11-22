# SC Telemetry Local

Tableau de bord **100 % local et en lecture seule** pour Star Citizen, sur ton PC ou ton téléphone : vaisseau, voyants, alimentation, vol, carburant, boucliers, coque, vitals, contrat et objectif en cours.

*English below.*

## Ce que fait l'outil

- Lit la télémétrie que le jeu fournit lui-même sur `127.0.0.1` (service gRPC `sc.game.telemetry.v1`) et le fichier `Game.log`.
- Lit les définitions de la télémétrie directement dans `StarCitizen.exe` : il suit les patchs sans mise à jour.
- Sert une page web sur `http://localhost:8742`, et en option sur ton réseau local (téléphone, tablette) avec un QR code.

## Ce qu'il ne fait pas

- Aucune donnée envoyée sur Internet, aucun compte, aucun serveur tiers.
- Ne lit pas la mémoire du jeu, n'injecte rien, ne touche pas à Easy Anti-Cheat.
- N'envoie aucune commande au jeu.
- Seule modification du PC, sur clic de ta part : la variable `GRPC_CLIENT_SERVER_ENABLED=true` pour ton compte Windows (et, si tu le demandes, une règle de pare-feu pour l'accès depuis le téléphone, réseaux privés uniquement).

## Utilisation

1. Télécharge le `.zip` dans [Releases](../../releases), décompresse-le et lance `SCTelemetryLocal.exe`. L'exe n'est pas signé : SmartScreen demande « Informations complémentaires » puis « Exécuter quand même ».
2. Le tableau de bord s'ouvre dans le navigateur. Le panneau « Mise en route » guide pas à pas.
3. Laisse la fenêtre de console ouverte pendant que tu joues.

Options : `-port 8742`, `-lan` (accès réseau local), `-nobrowser`, `-sc "D:\…\StarCitizen\LIVE"` (si l'installation n'est pas trouvée), `-once` (test en console).

## LED VKB (optionnel)

Désactivé par défaut. Bouton « 💡 LED » dans la page (sur le PC uniquement). L'outil envoie aux LED des
contrôleurs VKB le même rapport HID que VKBDevCfg (rien n'est enregistré dans l'appareil ; débrancher/rebrancher
rend les LED d'origine) :

- Gladiator EVO : barre de la poignée = mode (vert SCM, bleu NAV, rouge touché, rouge rapide missile), LED bleue = découplé ; le point du ministick n'est pas touché.
- S-TECS : grande barre = boucliers (vert / orange / rouge), DNT = train, LOCK = voyant quantum, ON = centrale.

L'outil suit aussi la branche lancée (LIVE, PTU…) et se relance de lui-même si elle change.
## Virpil via VPC Link Tool (optionnel, à tester)

Désactivé par défaut. L'outil se présente à VPC Link Tool comme le script DCS de Virpil : JSON en UDP sur
127.0.0.1:4123, 10 fois par seconde. Table de conversion : clés DCS habituelles (`aircraftType`, `mech_Gear_Status`,
`mech_Gear_Value`, `mcp_MasterWarning`, `mcp_AutopilotOn`, `plane_flare`, `plane_chaff`, `plane_RadarAltimeter`…)
et tous les champs de la télémétrie sous leur nom d'origine (`lights_enabled`, `flight.state.gear`,
`power.shield_generators`, `annunciators.hit`…), booléens en 0/1, énumérations en nombres. Les LED se règlent ensuite dans VPC Link Tool.
## Démarrage automatique et réglage des LED

- Bouton « ⚙ » : entrée « Exécuter » du compte Windows (`SCTelemetryLocal.exe -wait`). L'outil attend sans fenêtre,
  s'ouvre quand RSI Launcher ou Star Citizen démarre, se remet en attente 1 minute après leur fermeture. Une seule
  instance à la fois ; relancer l'exe réveille celle qui attend. Journal : `%APPDATA%\SCTelemetryLocal\autostart.log`.
- Page `/leds.html` : règles par LED VKB (première règle vraie = mode + couleur), conditions sur les noms d'origine
  de la télémétrie (`seated`, `flight.state.gear == DEPLOYED`…), identification d'une LED, réglage d'origine.
## Compiler

Go 1.22 ou plus récent, sous Windows :

```
go build -trimpath -o SCTelemetryLocal.exe .
```

Icône et informations de version : `go run ./tools/icon` puis `go-winres make` (fichiers `.syso` déjà fournis).

## Remerciements

La documentation de [7Hills Link](https://github.com/TheBanjolorian/7hills-link) (`docs/TELEMETRY.md`) sur la télémétrie locale du jeu.

---

## English

A **100% local, read-only** dashboard for Star Citizen, on your PC or phone. It reads the telemetry the game itself serves on `127.0.0.1` and `Game.log`, takes the API definitions from your installed `StarCitizen.exe`, and serves a page at `http://localhost:8742` (optionally on your LAN with a QR code). No data leaves your network, no game memory access, no commands sent to the game. The only change to your PC, when you click it: `GRPC_CLIENT_SERVER_ENABLED=true` for your Windows user.

Download the `.zip` from [Releases](../../releases), run `SCTelemetryLocal.exe` (unsigned: SmartScreen > "More info" > "Run anyway"), follow the "Setup" panel.

Personal project, not affiliated with Cloud Imperium Games. Star Citizen® is a trademark of Cloud Imperium Rights LLC. License: MIT.

Virpil: optional bridge to VPC Link Tool (pretends to be the DCS script, JSON over UDP 127.0.0.1:4123, original telemetry field names and DCS keys).
VKB LEDs (optional, off by default, "💡 LED" button): Gladiator grip bar = master mode / hit / missile, blue LED = decoupled;
S-TECS bars = shields, gear, quantum, ON = power plant. Nothing is stored in the device; unplug/replug to restore its LEDs.