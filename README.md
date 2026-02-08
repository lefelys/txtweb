# txtweb

Serve a website directly from your domain's DNS TXT record 🤡

## How it works

1. Create a DNS `A` record for your (sub)domain pointing to `95.217.246.14`.
2. Add a TXT record at `_txtweb.{your-(sub)domain}` with the content you want to serve.
3. Optional: add a TXT record at `_txtweb_cfg.{your-(sub)domain}` to configure rendering options (semicolon-separated).

### Supported `_txtweb_cfg` keys

- `content-type`: `Content-Type` header.
- `html-wrap`: wrap output in standard HTML tags (`true`/`false`).
- `html-align`: content alignment (`top-left|top-right|bottom-left|bottom-right|center`).
- `html-max-width`: maximum content width.
- `html-bg`: background color.
- `html-fg`: text color.

Example:

`html-wrap=true;html-align=center;html-bg=#ffc300`

