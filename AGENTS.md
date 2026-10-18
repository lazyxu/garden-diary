# Garden Diary development rules

- Product: family-shared garden, Web + PWA, Go API, SQLite, local photos.
- The app must work fully over LAN HTTP. HTTPS is optional for public domains and required for installable PWA on non-localhost origins.
- One photo diary may hold 1–8 pictures, with time and an optional note. Plants remain independently managed and can be grouped by species.
- Never expose the server's Docker socket to the API container.
- Keep SQLite database, photos and certificates on bind mounts independent of replaceable containers.
- Treat installer/update as a transaction: take a consistent stopped-database snapshot, pull exact tagged images, start and verify health before success, restore old compose/env/data on failed upgrade.
- Before making a performance change, capture baseline measurements, repeat the same workload, retain only a justified improvement and record both results.
- Do not commit secrets, data, uploads or database files.
- For each change: state current progress, explain remaining work, run Go tests, Web build, Compose validation, and document any skipped validation.
- Use a separate feature branch and PR, keep master stable, and avoid unrelated xDrive code.
