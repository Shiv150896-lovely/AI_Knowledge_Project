# Frontend UI

This standalone React/Vite frontend implements the flows in `FrontendUIPlan.md` without changing the Go backend.

## Run

1. Copy `.env.example` to `.env` and set `VITE_API_BASE_URL` to the Go API URL.
2. Install and run:

```powershell
npm install
npm run dev
```

Open the Vite URL, normally `http://localhost:5173`.

## Production

```powershell
npm run build
```

Deploy `dist/` as a static single-page application with history fallback. The Go API remains unchanged. Configure its `APP_BASE_URL` to the deployed UI origin when its generated assistant URL should open this frontend’s `/assistant/:assistantToken` route.
