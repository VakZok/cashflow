# Self-hosted on a Raspberry Pi behind Tailscale, no app login

The app runs on the user's Raspberry Pi with a SQLite database, reachable only over Tailscale; Tailscale membership is the only access control, so the app has no login. We chose this over browser-only storage (data trapped on one device) and cloud backends (hosting cost, third-party custody of financial data). iCloud sync of the live database was rejected because syncing SQLite files corrupts them. Consequence: the database must be backed up off the Pi (nightly `sqlite3 .backup` copy), and the app must never be exposed publicly.
