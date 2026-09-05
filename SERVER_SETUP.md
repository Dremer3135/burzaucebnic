# Burza Učebnic - Server Deployment & Administration Guide

This guide is for the backend administrator or automation agent deploying **Burza Učebnic** on a Linux server for the domain **`burza.skrat.org`**.

---

## 1. Architecture Overview

```
                      Internet (HTTPS 443)
                               │
                               ▼
            ┌────────────────────────────────────────┐
            │   Reverse Proxy (Caddy or Nginx)       │
            │   Domain: burza.skrat.org              │
            │   Handles SSL / TLS (Let's Encrypt)    │
            └──────────────┬──────────────────┬──────┘
                           │                  │
               /api/* & /_/*                  │ All other routes (SSR / HTML)
                           ▼                  ▼
     ┌────────────────────────────┐    ┌────────────────────────────┐
     │ PocketBase Go Backend      │    │ SvelteKit Node.js Frontend │
     │ Binary: burza-server       │    │ Runner: node build/index.js│
     │ Port: 127.0.0.1:<PB_PORT>  │    │ Port: 127.0.0.1:<FE_PORT>  │
     │ Data: pb_data/ (SQLite)    │    │ Reads: PB_PORT=<PB_PORT>   │
     └────────────────────────────┘    └──────────────┬─────────────┘
                                                      │
                                                      │ Internal SSR Auth Verification
                                                      └──────────────────►
```

- **Backend (`server/`)**: Custom Go binary embedding PocketBase v0.40+. Manages SQLite database (`pb_data/`), schema migrations, Google OAuth2, book image compression via `ffmpeg` (scaled to 720p), QR code variable symbols, and cashier transactions.
- **Frontend (`web/`)**: SvelteKit 5 built with `@sveltejs/adapter-node`. Runs as a standalone Node.js server. Uses server hooks (`hooks.server.ts`) to validate sessions and guard routes.
- **Reverse Proxy**: Exposes port 443 with valid SSL certificates, routing `/api/*` and `/_/*` to PocketBase, and all other traffic to the SvelteKit frontend.

---

## 2. Port Configuration (Multi-Service Host Friendly)

Because the target server hosts multiple other applications, **ports are not hardcoded**.

1. **Check which ports are currently in use on the host:**
   ```bash
   sudo ss -tulpn | grep LISTEN
   ```
2. **Choose two available local ports:**
   - `<PB_PORT>`: For PocketBase (e.g. `8095`, `9090`, `18090`, or any unused port).
   - `<FE_PORT>`: For SvelteKit (e.g. `3005`, `5000`, `13000`, or any unused port).

*(In the examples below, replace `<PB_PORT>` and `<FE_PORT>` with your chosen port numbers).*

---

## 3. Step-by-Step Setup Instructions

### Step 1: DNS Setup
In the DNS management console for `skrat.org`:
- Create an **A Record**:
  - Host / Subdomain: `burza`
  - Target IP: `<YOUR_SERVER_PUBLIC_IP>`
  - TTL: 300 (or default)
- Confirm DNS resolution:
  ```bash
  ping -c 3 burza.skrat.org
  ```

---

### Step 2: Google Cloud Console OAuth 2.0 Setup

Authentication is restricted to **Google Sign-In with School Account**.

1. Open [Google Cloud Console](https://console.cloud.google.com/).
2. Create or select your project (e.g. `Burza Ucebnic`).
3. Go to **APIs & Services** → **OAuth consent screen**:
   - **User Type**:
     - **Internal** (Recommended if school has Google Workspace for Education): Automatically restricts sign-in exclusively to users with your school's domain. Personal `@gmail.com` accounts cannot authenticate.
     - **External**: If using a generic Google Cloud organization, set to External.
   - App Name: `Burza učebnic`
   - User support email: `<admin-email>`
   - Developer contact email: `<admin-email>`
   - Scopes: `openid`, `.../auth/userinfo.email`, `.../auth/userinfo.profile`.
4. Go to **Credentials** → **Create Credentials** → **OAuth Client ID**:
   - Application type: **Web application**
   - Name: `Burza Web Client`
   - **Authorized JavaScript origins**:
     ```
     https://burza.skrat.org
     ```
   - **Authorized redirect URIs** (CRITICAL — must match exactly):
     ```
     https://burza.skrat.org/api/oauth2-redirect
     ```
5. Copy the generated **Client ID** and **Client Secret**.

---

### Step 3: Server Prerequisites & Code Checkout

Install required packages on the Linux server (Ubuntu/Debian):
```bash
sudo apt update
sudo apt install -y git ffmpeg nodejs npm golang-go caddy
```
*(Note: `ffmpeg` is required by `server/main.go` to compress uploaded textbook photos to 720p).*

Clone the repository:
```bash
sudo mkdir -p /opt/burzaucebnic
sudo chown -R $USER:$USER /opt/burzaucebnic
git clone https://github.com/Dremer3135/burzaucebnic.git /opt/burzaucebnic
cd /opt/burzaucebnic
```

---

### Step 4: Build Backend & Frontend

1. **Build Go Backend:**
   ```bash
   cd /opt/burzaucebnic/server
   go build -o burza-server main.go
   chmod +x burza-server
   ```

2. **Build SvelteKit Frontend:**
   ```bash
   cd /opt/burzaucebnic/web
   npm ci
   npm run build
   ```
   *(This outputs the standalone Node server in `/opt/burzaucebnic/web/build`).*

---

### Step 5: Systemd Service Configuration

#### 1. Backend Service: `/etc/systemd/system/burza-backend.service`
Create the file with:
```ini
[Unit]
Description=Burza Ucebnic PocketBase Backend
After=network.target

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/opt/burzaucebnic/server
ExecStart=/opt/burzaucebnic/server/burza-server serve --http="127.0.0.1:<PB_PORT>"
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

#### 2. Frontend Service: `/etc/systemd/system/burza-frontend.service`
Create the file with:
```ini
[Unit]
Description=Burza Ucebnic SvelteKit Frontend
After=network.target burza-backend.service

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/opt/burzaucebnic/web
Environment=NODE_ENV=production
Environment=HOST=127.0.0.1
Environment=PORT=<FE_PORT>
Environment=PB_PORT=<PB_PORT>
ExecStart=/usr/bin/node /opt/burzaucebnic/web/build/index.js
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

#### 3. Set Permissions and Start Services:
```bash
sudo chown -R www-data:www-data /opt/burzaucebnic
sudo systemctl daemon-reload
sudo systemctl enable --now burza-backend burza-frontend
```

Verify services are active and listening:
```bash
sudo systemctl status burza-backend
sudo systemctl status burza-frontend
curl http://127.0.0.1:<PB_PORT>/api/health
curl -I http://127.0.0.1:<FE_PORT>/
```

---

### Step 6: Reverse Proxy Configuration

#### Option A: Caddy (Recommended)
Add to `/etc/caddy/Caddyfile`:
```caddyfile
burza.skrat.org {
    # PocketBase API and Admin Dashboard
    handle /api/* {
        reverse_proxy 127.0.0.1:<PB_PORT>
    }
    handle /_/* {
        reverse_proxy 127.0.0.1:<PB_PORT>
    }

    # SvelteKit SSR Frontend
    handle {
        reverse_proxy 127.0.0.1:<FE_PORT>
    }
}
```
Reload Caddy:
```bash
sudo systemctl reload caddy
```
*(Caddy will automatically provision and renew Let's Encrypt certificates for `burza.skrat.org`).*

---

#### Option B: Nginx + Certbot
If using Nginx, create `/etc/nginx/sites-available/burza.skrat.org`:
```nginx
server {
    listen 80;
    server_name burza.skrat.org;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name burza.skrat.org;

    ssl_certificate /etc/letsencrypt/live/burza.skrat.org/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/burza.skrat.org/privkey.pem;

    client_max_body_size 25M;

    # PocketBase API & Realtime WebSockets
    location ~ ^/(api|_)/ {
        proxy_pass http://127.0.0.1:<PB_PORT>;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # SvelteKit Frontend
    location / {
        proxy_pass http://127.0.0.1:<FE_PORT>;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```
Enable site and reload Nginx:
```bash
sudo ln -s /etc/nginx/sites-available/burza.skrat.org /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

---

### Step 7: PocketBase Initial Setup & OAuth2 Activation

1. Open **`https://burza.skrat.org/_/`** in your browser.
2. If this is a fresh install, create the **Initial Admin Account** when prompted.
3. Configure Application URL:
   - Go to **Settings** → **Application**.
   - Set **Application URL** to: `https://burza.skrat.org`.
   - Click **Save changes**.
4. Enable Google OAuth2:
   - Go to **Settings** → **Auth providers** (or **Collections** → `users` → gear icon → **OAuth2 providers**).
   - Click **Google**.
   - Check **Enabled**.
   - Enter your **Client ID** and **Client Secret** obtained from Google Cloud Console.
   - Click **Save**.

---

### Step 8: Promoting a User to Cashier

Because Google Sign-In is the sole login method:
1. Have the cashier log into `https://burza.skrat.org` with their Google account once.
2. Log into PocketBase Admin at `https://burza.skrat.org/_/`.
3. Open the **`users`** collection.
4. Click on the cashier's user row.
5. Set the **`isCashier`** toggle to **`true`**.
6. Click **Save changes**.
7. The user now has cashier authorization (`/cashier` and `/cashier/payments`).

---

## 4. Maintenance & Operational Commands

- **Check backend logs:**
  ```bash
  sudo journalctl -u burza-backend -f
  ```
- **Check frontend logs:**
  ```bash
  sudo journalctl -u burza-frontend -f
  ```
- **Restart services after git pull:**
  ```bash
  cd /opt/burzaucebnic
  git pull origin main

  # If backend changed:
  cd server && go build -o burza-server main.go && sudo systemctl restart burza-backend

  # If frontend changed:
  cd ../web && npm ci && npm run build && sudo systemctl restart burza-frontend
  ```
- **Backup database:**
  PocketBase stores everything in SQLite inside `/opt/burzaucebnic/server/pb_data`.
  To back up:
  ```bash
  tar -czvf burza_db_backup_$(date +%F).tar.gz /opt/burzaucebnic/server/pb_data
  ```

---

## 5. Developing Locally with Remote Production Backend

You can run the SvelteKit frontend locally on your machine while pointing all API requests and Google authentication directly to the remote server:

```bash
cd web
npm run dev:remote
```

- Your local browser connects to `https://127.0.0.1:5174` (with live hot reload and camera access).
- Vite's proxy automatically routes `/api/*` and `/_/*` to `https://burza.skrat.org`.
- Google Sign-In popups and real database records sync in real time against the production PocketBase instance.

