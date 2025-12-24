
# Bible Verse REST API (Go + SQLite)

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?logo=go)](https://go.dev)
[![SQLite](https://img.shields.io/badge/SQLite-3-003B57.svg?logo=sqlite)](https://www.sqlite.org/)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A lightweight, high-performance REST API written in Go that returns a random Bible verse from a SQLite database as JSON.

Designed for:
- Low-RAM Ubuntu VPS
- Read-only SQLite databases
- Bots, widgets, and simple public APIs

---

## Features

- 🚀 Very low memory and CPU usage
- 📦 Single static Go binary
- 🗃 SQLite (read-only)
- 🌍 Multiple translations via simple map config
- 🧹 Automatic tag cleanup from verse text
- 🔌 No external dependencies or frameworks

---

## API Endpoint

### Get random verse

```

GET /get-random-verse/{TRANSLATION}/

```

**Example**
```

GET /get-random-verse/KJV/

````

**Response**
```json
{
    "translation":"KJV",
    "book_number":500,
    "book_title":"John",
    "book_title_short":"John",
    "chapter":8,
    "verse":36,
    "text":"If the Son therefore shall make you free, ye shall be free indeed."
}
````

---

## Running Locally

```bash
go run main.go
```

The server will start on the configured port (default: `:8080`).

---

## Preparing the build
```bash
go mod download
go mod download github.com/mattn/go-sqlite3
```

## Building Binary

```bash
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o bible-api -ldflags="-s -w" main.go
```

## Running Binary

```bash
./bible-api 
```