# hardlink-ui

<a href="https://www.paypal.com/donate/?hosted_button_id=5GG7HMSFJDH82" target="_blank">
  <img src="https://img.shields.io/badge/☕_Offrir_un_café-FFDD00?style=for-the-badge&logo=paypal&logoColor=003087" />
</a>

**Interface web moderne pour gérer les hardlinks sur votre NAS ou serveur de stockage.**  
Dupliquez sans perdre de place. 🚀

---

## 📚 Sommaire

- [📖 Présentation](#-présentation)
- [🚀 Démarrage rapide](#-démarrage-rapide)
  - [Prérequis](#prérequis)
  - [Configuration rapide avec docker-compose](#configuration-rapide-avec-docker-compose)
  - [Trouver PUID/PGID](#trouver-puidpgid)
  - [Générer un secret TOTP (2FA)](#-générer-un-secret-totp-2fa)
- [✨ Fonctionnalités](#-fonctionnalités)
- [📋 Prérequis détaillés](#-prérequis-détaillés)
- [🔧 Installation avancée](#-installation-avancée)
- [⚙️ Configuration](#️-configuration)
- [🔒 Notes de sécurité](#-notes-de-sécurité)
- [📱 Guide d’utilisation](#-guide-dutilisation)
- [📱 Progressive Web App (PWA)](#-progressive-web-app-pwa)
- [🛠️ Troubleshooting](#️-troubleshooting)
- [❓ FAQ](#-faq)
- [📄 Licence](#-licence)
- [🤝 Contribution](#-contribution)
- [📞 Support](#-support)

---

## 📖 Présentation

**hardlink-ui** est une interface web minimaliste et sécurisée permettant de créer, gérer et optimiser les hardlinks sur vos systèmes de fichiers Linux. L'application a été développée et testée sur **Synology DSM** - le support sur d'autres plateformes n'est pas garanti mais peut fonctionner.

Parfait pour les utilisateurs de NAS Synology qui souhaitent économiser de l'espace disque en remplaçant les fichiers dupliqués par des hardlinks.

### Pourquoi les hardlinks ?

Les hardlinks permettent à plusieurs chemins de pointer vers le même fichier physique sur le disque. Contrairement aux copies, ils n'occupent pas d'espace supplémentaire tout en conservant l'apparence de fichiers distincts dans différents dossiers.

**Cas d'usage typiques :**
- Organiser votre bibliothèque média en plusieurs catégories sans dupliquer les fichiers
- Économiser de l'espace en convertissant des doublons existants en hardlinks
- Créer des structures de dossiers alternatives sans copier les données

---

## 🚀 Démarrage rapide

La méthode la plus simple pour commencer avec **hardlink-ui** sur votre NAS Synology :

### Prérequis

- **Docker** installé sur votre Synology (via Package Center)
- Accès SSH ou Container Manager sur votre Synology

### Configuration rapide avec docker-compose

1. **Créez un dossier** pour hardlink-ui sur votre NAS (par exemple `/volume1/docker/hardlink-ui`)

2. **Créez un fichier `docker-compose.yml`** dans ce dossier :

```yaml
version: "3.9"

services:
  hardlink-ui:
    image: ghcr.io/gosiva/hardlink-ui:latest
    container_name: hardlink-ui
    restart: unless-stopped
    ports:
      - "8095:8000"
    environment:
      - TZ=Europe/Brussels
      - APP_SECRET_KEY=CHANGEZ_MOI_SECRET_ALEATOIRE_LONG
      - APP_ADMIN_USER=admin
      - APP_ADMIN_PASSWORD=VotreMotDePasseSecurise
      - APP_TOTP_SECRET=VotreSecretTOTP
      - APP_DATA_ROOT=/data
      # PUID/PGID OBLIGATOIRES - voir étape 3 ci-dessous
      - PUID=1026
      - PGID=100
    volumes:
      - /volume1/data:/data  # Changez selon votre volume
```

### Trouver PUID/PGID

   Sur votre Synology, en SSH :
   ```bash
   id votre_nom_utilisateur
   ```
   
   Exemple de sortie :
   ```
   uid=1026(john) gid=100(users)
   ```
   
   Utilisez ces valeurs dans le docker-compose :
   - `PUID=1026` (votre uid)
   - `PGID=100` (votre gid)
   
   **Pourquoi c'est obligatoire ?** Pour que Docker puisse créer/modifier vos fichiers avec les bonnes permissions.
   
   **Guide détaillé Synology :** https://mariushosting.com/synology-find-uid-userid-and-gid-groupid-in-5-seconds/

### 🔐 Générer un secret TOTP (2FA)

   Pour activer la double authentification, vous devez fournir un **secret TOTP**.  
   Ce secret permet de générer les codes à 6 chiffres utilisés lors de la connexion.

   ---

   #### 🟢 Méthode 1 : Générer un secret via un site web (recommandé)

   Utilisez un générateur simple et fiable :

   👉 https://randomkeygen.com/totp-secret

   1. Ouvrez la page  
   2. Dans **TOTP Secret Generator**, choisissez **32 bytes**  
   3. Copiez la clé Base32 générée  
   4. Collez-la dans votre `docker-compose.yml` :

   ~~~yaml
   environment:
   - APP_TOTP_SECRET=VOTRE_SECRET_TOTP
   ~~~

   ---

   #### 🔵 Méthode 2 : Générer un secret sur Windows (PowerShell)

   ~~~powershell
   $bytes = New-Object byte[] 32; (New-Object System.Security.Cryptography.RNGCryptoServiceProvider).GetBytes($bytes); $alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"; $output = ""; $buffer = 0; $bitsLeft = 0; foreach ($b in $bytes) { $buffer = ($buffer -shl 8) -bor $b; $bitsLeft += 8; while ($bitsLeft -ge 5) { $bitsLeft -= 5; $output += $alphabet[($buffer -shr $bitsLeft) -band 31]; } }; if ($bitsLeft -gt 0) { $output += $alphabet[($buffer -shl (5 - $bitsLeft)) -band 31]; }; $output
   ~~~

   1. Exécutez la commande  
   2. Copiez la valeur générée  
   3. Collez-la dans `APP_TOTP_SECRET`

   ---

   #### 📱 Ajouter le secret dans votre application d’authentification

   Compatible avec : Google Authenticator, Authy, Aegis, Bitwarden, etc.

   1. Ouvrez votre application 2FA  
   2. Appuyez sur **+**  
   3. Choisissez **Saisir une clé de configuration**  
   4. Renseignez :
      - **Nom du compte :** `hardlink-ui`
      - **Clé :** votre secret TOTP  
      - **Type :** TOTP / Time-based  

   ---

   #### 🧩 Optionnel : Ajouter via QR Code

   Si vous préférez scanner un QR code, utilisez cette URL :

   ~~~text
   otpauth://totp/username?secret=VOTRE_SECRET_TOTP&issuer=hardlink-ui
   ~~~

   Modifier "username" par le Login que vous avez configurer !

   Générez ensuite un QR code avec un outil en ligne :

   👉 https://www.qr-code-generator.com  

   ---

### ✔️ Exemple complet dans docker-compose

~~~yaml
environment:
  - APP_ADMIN_USER=admin
  - APP_ADMIN_PASSWORD=VotreMotDePasse
  - APP_TOTP_SECRET=VOTRE_SECRET_TOTP
  - APP_SECRET_KEY=VotreCleSecrete
  - APP_DATA_ROOT=/data
  - PUID=1026
  - PGID=100
~~~

5. **Démarrez l'application** :
   ```bash
   docker-compose up -d
   ```

6. **Accédez à l'interface** :
   
   Ouvrez votre navigateur : `http://votre-nas:8095`
   
   (ou `http://localhost:8095` si vous êtes sur le NAS)

**C'est tout !** Vous pouvez maintenant vous connecter avec vos identifiants admin et le code 2FA.

---

## ✨ Fonctionnalités

- 🔍 **Explorateur de hardlinks** : Parcourez vos fichiers et visualisez les liens existants
- 🔗 **Créateur de hardlinks** : Créez des hardlinks pour fichiers ou dossiers entiers
- 📊 **Détection de doublons** : Scannez et convertissez automatiquement les fichiers dupliqués
- 📱 **Interface responsive** : Fonctionne sur desktop, tablette et mobile
- 🔒 **Authentification 2FA** : Sécurité renforcée avec TOTP
- 🌓 **Thème sombre/clair** : Interface élégante adaptable
- 🇫🇷 **Interface en français** : Navigation intuitive en français

---

**Explorateur de hardlinks**

L'explorateur permet de parcourir vos fichiers et de visualiser les liens existants. Les fichiers avec plusieurs hardlinks sont indiqués par un badge montrant le nombre de liens. L'interface affiche :
- Navigation par arborescence avec breadcrumb
- Recherche en temps réel
- Détails des hardlinks avec tous les emplacements
- Mode suppression pour retirer les hardlinks en trop

**Créateur de hardlinks**

Interface intuitive pour créer des hardlinks en mode Single (fichier par fichier) ou Multi (sélection multiple). Fonctionnalités :
- Sélection source et destination côte à côte
- Création de nouveaux dossiers à la volée
- Validation des noms de fichiers compatible Synology/DSM
- Traitement par lots en mode Multi

**Détection de doublons**

Le scan de doublons identifie les fichiers identiques et calcule l'espace potentiellement économisable. Caractéristiques :
- Dashboard avec statistiques en temps réel
- Groupes de fichiers identiques triés par taille
- Conversion en masse vers hardlinks
- Indicateur de progression pendant la conversion

**Interface responsive**

L'interface s'adapte automatiquement aux différentes tailles d'écran :
- Desktop : interface complète avec panneaux côte à côte
- Tablette : adaptation des colonnes et espacement
- Mobile : navigation optimisée, tables scrollables, tooltips tactiles
- PWA : fonctionne comme une application native sur iOS et Android

---

## 📋 Prérequis détaillés

- **Docker** et **Docker Compose** installés sur votre système
- Un système de fichiers supportant les hardlinks (ext4, btrfs, xfs, etc.)
- Accès aux permissions nécessaires sur le dossier de données

---

## 🔧 Installation avancée

### Option 1 : Docker run (simple)

```bash
docker run -d \
  --name hardlink-ui \
  -p 8095:8000 \
  -e APP_ADMIN_USER=admin \
  -e APP_ADMIN_PASSWORD=VotreMotDePasseSecurise \
  -e APP_TOTP_SECRET=VotreSecretTOTP \
  -e APP_SECRET_KEY=VotreCleSecreteSession \
  -e APP_DATA_ROOT=/data \
  -e PUID=1026 \
  -e PGID=100 \
  -v /volume1/data:/data \
  ghcr.io/gosiva/hardlink-ui:latest
```

### Option 2 : Build depuis les sources

1. **Clonez le dépôt :**
   ```bash
   git clone https://github.com/gosiva/hardlink-ui.git
   cd hardlink-ui
   ```

2. **Buildez l'image :**
   ```bash
   docker build -t hardlink-ui:local .
   ```

3. **Lancez le conteneur :**
   ```bash
   docker run -d \
     --name hardlink-ui \
     -p 8095:8000 \
     -e APP_ADMIN_USER=admin \
     -e APP_ADMIN_PASSWORD=VotreMotDePasseSecurise \
     -e APP_TOTP_SECRET=VotreSecretTOTP \
     -e APP_SECRET_KEY=VotreCleSecreteSession \
     -e APP_DATA_ROOT=/data \
     -e PUID=1026 \
     -e PGID=100 \
     -v /volume1/data:/data \
     hardlink-ui:local
   ```

---

## ⚙️ Configuration

### Variables d'environnement

| Variable | Description | Défaut | Obligatoire |
|----------|-------------|--------|-------------|
| `APP_ADMIN_USER` | Nom d'utilisateur admin | - | ✅ |
| `APP_ADMIN_PASSWORD` | Mot de passe admin | - | ✅ |
| `APP_TOTP_SECRET` | Secret TOTP pour 2FA | - | ✅ |
| `APP_SECRET_KEY` | Clé secrète pour les sessions | `dev_insecure_key` | ⚠️ Recommandé |
| `APP_DATA_ROOT` | Chemin racine des données à gérer | `/data` | ✅ |
| `PUID` | User ID pour les permissions fichiers | `1000` | ✅ **Obligatoire** |
| `PGID` | Group ID pour les permissions fichiers | `1000` | ✅ **Obligatoire** |
| `PORT` | Port d'écoute interne du serveur | `8000` | ❌ |
| `HOST` | Adresse d'écoute du serveur | `0.0.0.0` | ❌ |
| `SESSION_TIMEOUT` | Durée des sessions en secondes | `3600` | ❌ |
| `LOG_LEVEL` | Niveau de journalisation (INFO, DEBUG) | `INFO` | ❌ |

### PUID et PGID : Explication et importance

**⚠️ PUID/PGID sont OBLIGATOIRES pour un fonctionnement correct sur Synology**

Lorsque vous exécutez hardlink-ui dans Docker, le processus s'exécute avec un utilisateur spécifique. Si cet utilisateur n'a pas les mêmes permissions que votre utilisateur système, vous rencontrerez des problèmes de permissions lors de la création de hardlinks.

**Comment trouver vos PUID/PGID sur Synology ?**

**Méthode 1 : Via SSH** (recommandée)

Sur votre Synology, en SSH :
```bash
id votre_utilisateur
```

Exemple de sortie :
```
uid=1026(john) gid=100(users) groups=100(users),101(administrators)
```

Dans cet exemple :
- `PUID=1026` (uid)
- `PGID=100` (gid - souvent 100 pour le groupe "users" sur Synology)

**Méthode 2 : Guide Synology en 5 secondes**

Suivez ce guide détaillé avec captures d'écran :
👉 https://mariushosting.com/synology-find-uid-userid-and-gid-groupid-in-5-seconds/

**Exemple de configuration complète :**

```env
# Utilisateur qui possède les fichiers sur le NAS
PUID=1026
PGID=100

# Autres configurations
APP_ADMIN_USER=admin
APP_ADMIN_PASSWORD=SuperSecretPassword123!
APP_TOTP_SECRET=JBSWY3DPEHPK3PXP
APP_SECRET_KEY=une-cle-secrete-longue-et-aleatoire-123456
APP_DATA_ROOT=/data
```

**Configuration docker-compose.yml :**

```yaml
version: "3.9"

services:
  hardlink-ui:
    image: ghcr.io/gosiva/hardlink-ui:latest
    container_name: hardlink-ui
    restart: unless-stopped
    ports:
      - "8095:8000"
    environment:
      - PUID=${PUID}
      - PGID=${PGID}
      - APP_ADMIN_USER=${APP_ADMIN_USER}
      - APP_ADMIN_PASSWORD=${APP_ADMIN_PASSWORD}
      - APP_TOTP_SECRET=${APP_TOTP_SECRET}
      - APP_SECRET_KEY=${APP_SECRET_KEY}
      - APP_DATA_ROOT=/data
    volumes:
      - /volume1/data:/data  # Adaptez selon votre volume Synology
```

---

## 🔒 Notes de sécurité

⚠️ **Important** : Cette application manipule directement vos fichiers. Prenez ces précautions :

1. **Authentification forte** :
   - Utilisez un mot de passe admin robuste (minimum 16 caractères)
   - Activez toujours le 2FA avec une application d'authentification
   - Ne partagez jamais votre secret TOTP

2. **Clé secrète de session** :
   - Générez une clé aléatoire longue pour `APP_SECRET_KEY`
   - Ne réutilisez jamais la valeur par défaut en production
   - Exemple de génération :
     ```bash
     python3 -c "import secrets; print(secrets.token_hex(32))"
     ```

3. **Accès réseau** :
   - Si exposé sur Internet, utilisez un reverse proxy avec HTTPS (nginx, Traefik, Caddy)
   - Configurez des règles de pare-feu strictes
   - Envisagez l'utilisation d'un VPN pour l'accès distant

4. **Permissions** :
   - Limitez l'accès au dossier `APP_DATA_ROOT` uniquement aux données nécessaires
   - N'accordez jamais l'accès à la racine système (`/`)

5. **Sauvegardes** :
   - Effectuez toujours des sauvegardes avant des opérations massives
   - La conversion de doublons en hardlinks est **irréversible**

---

## 📱 Guide d'utilisation

### 1. Connexion

1. Accédez à `http://votre-nas:8095` (ou l'adresse IP de votre serveur avec le port 8095)
2. Entrez vos identifiants admin
3. Confirmez avec le code 2FA de votre application d'authentification

### 2. Explorateur de hardlinks

- **Navigation** : Cliquez sur les dossiers pour naviguer
- **Recherche** : Utilisez la barre de recherche pour filtrer les fichiers
- **Détails** : Sélectionnez un fichier pour voir tous ses emplacements hardlink
- **Badge** : Le nombre à côté d'un fichier indique le nombre de hardlinks

### 3. Créateur de hardlinks

**Mode Single :**
1. Sélectionnez un fichier ou dossier source
2. Naviguez vers le dossier de destination
3. Cliquez sur "Créer le hardlink"

**Mode Multi :**
1. Activez le mode Multi avec le switch
2. Cochez plusieurs fichiers/dossiers
3. Sélectionnez le dossier de destination
4. Cliquez sur "Créer X hardlinks"

### 4. Détection et conversion de doublons

1. Allez dans l'onglet "Doublons"
2. Cliquez sur "🔍 Scanner les doublons"
3. Attendez la fin du scan
4. Sélectionnez les groupes à convertir
5. Cliquez sur "🔗 Convertir en hardlinks"
6. Confirmez l'opération (irréversible !)

Le scan utilise une méthode rapide (hash du début et de la fin des fichiers) pour détecter les doublons sans lire entièrement les gros fichiers.

### 5. Paramètres

- **Nom de la racine** : Personnalisez le nom affiché au lieu de "/"
- **Niveau de journalisation** : Minimal, Debug ou Trace
- **Thème** : Sombre ou Clair

---

## 📱 Progressive Web App (PWA)

**hardlink-ui** fonctionne comme une Progressive Web App, offrant une expérience d'application native sur mobile et desktop.

### Installation sur iPhone/iPad

1. **Ouvrez** hardlink-ui dans Safari
2. **Appuyez** sur le bouton de partage (icône carrée avec flèche vers le haut)
3. **Sélectionnez** "Sur l'écran d'accueil"
4. **Confirmez** l'ajout

L'application apparaîtra sur votre écran d'accueil avec une icône dédiée.

### Splash Screens iOS

L'application inclut des écrans de démarrage (splash screens) optimisés pour tous les modèles d'iPhone et iPad actuels :
- iPhone 14 Pro Max, 13 Pro Max, 12 Pro Max (430x932)
- iPhone 14 Pro, 13 Pro, 12 Pro (393x852)
- iPhone 14 Plus, 13, 12 (390x844)
- iPhone 11 Pro Max, XS Max (414x896)
- iPhone 11, XR (414x896)
- iPhone X, XS (375x812)
- iPad Pro 12.9" (1024x1366)
- iPad Pro 11", Air (834x1194, 834x1112, 810x1080)

**Note comportementale iOS** : En raison des limitations d'iOS, l'écran de démarrage peut parfois ne pas s'afficher lors du premier lancement. Fermez et rouvrez l'application pour voir le splash screen. Ce comportement est lié à la gestion du cache par iOS et non à l'application elle-même.

### Installation sur Android

1. **Ouvrez** hardlink-ui dans Chrome
2. **Appuyez** sur le menu (⋮) puis "Ajouter à l'écran d'accueil"
3. **Confirmez** l'ajout

L'application fonctionnera en mode standalone avec sa propre icône.

### Fonctionnalités PWA

- ✅ Installation sur l'écran d'accueil
- ✅ Fonctionne en mode standalone (plein écran)
- ✅ Splash screens personnalisés
- ✅ Icônes adaptées à chaque plateforme
- ✅ Thème personnalisé (barre de statut)
- ⚠️ Mode hors ligne non supporté (nécessite une connexion au serveur)

---

## 🛠️ Troubleshooting

### Problème : "Permission denied" lors de la création de hardlinks

**Cause** : Les PUID/PGID du conteneur ne correspondent pas à ceux de vos fichiers.

**Solution** :
1. Vérifiez le propriétaire de vos fichiers :
   ```bash
   ls -ln /chemin/vers/vos/donnees
   ```
2. Trouvez votre UID/GID :
   ```bash
   id votre_utilisateur
   ```
3. Mettez à jour `PUID` et `PGID` dans `.env`
4. Redémarrez le conteneur :
   ```bash
   docker-compose down && docker-compose up -d
   ```

### Problème : "Invalid cross-device link"

**Cause** : Vous essayez de créer un hardlink entre deux systèmes de fichiers différents.

**Solution** : Les hardlinks ne fonctionnent que sur le même système de fichiers. Vérifiez que source et destination sont sur la même partition.

```bash
df -h /chemin/source /chemin/destination
```

### Problème : Le scan de doublons est très lent

**Cause** : Beaucoup de fichiers ou fichiers très volumineux.

**Solution** : C'est normal. Le scan lit le début et la fin de chaque fichier. Pour des datasets de plusieurs To, cela peut prendre plusieurs minutes. Soyez patient.

### Problème : Impossible de se connecter après changement de mot de passe

**Cause** : Le fichier `users.json` contient toujours l'ancien hash.

**Solution** :
1. Arrêtez le conteneur
2. Supprimez le fichier `app/data/users.json`
3. Redémarrez le conteneur (il sera recréé avec le nouveau mot de passe)

### Problème : Les hardlinks ne s'affichent pas correctement

**Cause** : L'index d'inodes n'est pas à jour.

**Solution** : Rechargez la page ou allez dans l'onglet "Doublons" et lancez un scan (cela reconstruit l'index).

---

## ❓ FAQ

**Q : Quelle est la différence entre un hardlink et un lien symbolique ?**

R : Un hardlink pointe directement vers les données sur le disque (même inode), tandis qu'un lien symbolique (symlink) pointe vers un chemin de fichier. Si vous supprimez le fichier original, le symlink devient cassé, mais un hardlink reste valide. Les hardlinks économisent de l'espace car ils partagent les mêmes données.

**Q : Que se passe-t-il si je supprime un hardlink ?**

R : Supprimer un hardlink ne supprime que ce chemin spécifique. Le fichier physique reste sur le disque tant qu'au moins un hardlink pointe vers lui. Les données ne sont vraiment supprimées que lorsque tous les hardlinks sont supprimés.

**Q : Puis-je créer des hardlinks entre différents disques ?**

R : Non. Les hardlinks ne fonctionnent que sur le même système de fichiers (même partition). Pour des liens entre partitions, utilisez des liens symboliques (non supportés par cette application).

**Q : La conversion de doublons est-elle sûre ?**

R : Oui, mais irréversible. hardlink-ui remplace les fichiers dupliqués par des hardlinks vers un fichier "maître". Les données ne sont pas perdues, mais vous ne pourrez plus distinguer quel fichier était l'original. Faites toujours une sauvegarde avant.

**Q : Combien d'espace puis-je économiser ?**

R : Cela dépend de votre dataset. Si vous avez beaucoup de doublons (par exemple, la même série TV organisée par genre ET par année), vous pouvez économiser 50% ou plus. Lancez le scan de doublons pour une estimation.

**Q : hardlink-ui fonctionne-t-il sur Windows ?**

R : Non. Les hardlinks fonctionnent différemment sur Windows (NTFS) et ne sont pas supportés par cette application qui cible les systèmes Linux.

**Q : Puis-je utiliser hardlink-ui sur mon Synology NAS ?**

R : Oui ! C'est l'un des principaux cas d'usage. Installez Docker sur votre Synology via le Package Center, puis suivez les instructions d'installation. N'oubliez pas de configurer PUID/PGID correctement.

**Q : Comment désinstaller hardlink-ui ?**

R : Arrêtez et supprimez le conteneur :
```bash
docker-compose down
# ou
docker stop hardlink-ui && docker rm hardlink-ui
```

Les hardlinks créés resteront intacts. Seul le dossier `app/data/` contenant la configuration utilisateur peut être supprimé si nécessaire.

---

## 📄 Licence

Ce projet est sous licence MIT. Voir le fichier `LICENSE` pour plus de détails.

---

## 🤝 Contribution

Les contributions sont les bienvenues ! N'hésitez pas à ouvrir une issue ou une pull request.

---

## 📞 Support

Pour toute question ou problème :
- Ouvrez une [issue sur GitHub](https://github.com/gosiva/hardlink-ui/issues)
- Consultez la section [Troubleshooting](#-troubleshooting)

---

**hardlink-ui** - Gérez vos hardlinks en toute simplicité. 💾✨
[![Donate](https://img.shields.io/badge/PayPal-Donate-blue.svg)](https://www.paypal.com/donate/?hosted_button_id=5GG7HMSFJDH82)
