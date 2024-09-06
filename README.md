# shorten

> simple url shortener

### Setup

Environmental variables:
- `SHORTEN_HOST` - hostname
- `SHORTEN_BIND` - bind address (default: `127.0.0.1:4488`)
- `SHORTEN_MAIL` - optional email for support/abuse reports
- `POSTGRES_URI` - lib/pq connection string (see [here](https://pkg.go.dev/github.com/lib/pq#section-documentation))

### Maintenance

shorten supports a domain blacklist to ban certain domains (e.g. spam, malware, etc.);
you can use it by connecting to the database and inserting rows into the `blacklist` table.
