# Go with server-rendered HTML and HTMX

The app is a Go server using `html/template`, HTMX where needed, a pure-Go SQLite driver, and Chart.js for charts, compiled to a single ARM binary for the Pi. We chose this over Django, which would have been faster to a first version (free admin, migrations), because the user wants to practise Go and prefers a dependency-light binary that keeps working without upkeep. Consequence: forms, validation, CRUD screens and schema migrations are written by hand. A JS SPA was rejected as unnecessary for a single-user app.
