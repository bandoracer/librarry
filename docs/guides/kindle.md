# Send to Kindle

This feature is implemented on the Kindle feature branch; it is not part of the
previously deployed stabilization images until a new release is built and deployed.

Open **Settings → Kindle**. Enter your Kindle's `@kindle.com` or
`@free.kindle.com` address, your sender address, and SMTP credentials. Select
**Use Resend defaults** for `smtp.resend.com`, port 465, implicit TLS and username
`resend`; enter a sending API key as the password. Your sender domain must be
verified with Resend. Other authenticated SMTP providers work with implicit TLS
or mandatory STARTTLS. Certificate verification cannot be disabled.

Add the sender to Amazon's **Approved Personal Document E-mail List**, enable
Kindle delivery, and save. **Send test document** submits a small text attachment
to the saved Kindle address. Confirm it appears on your Kindle while connected to
Wi-Fi. Saving settings alone sends nothing.

On an active ebook's detail page, use **Read on Kindle**, select an EPUB or PDF,
and click **Send to Kindle**. The maximum source file size is 25 MiB; base64 and
MIME overhead remain below Resend's 40 MB message limit. Files must be present,
completed native library files with one unambiguous book association. The server
checks the actual file under a configured ebook root, size, stored checksum when
available, and EPUB/PDF signature. Symlinks cannot escape that root. Audiobooks,
MOBI/AZW files, pending imports and Calibre-managed files are not sent. Convert
or export those through Calibre and import the resulting EPUB into a native root.
Librarry does not perform conversion or remove DRM in this flow.

## History and recovery

The latest 50 attempts for a book appear beneath its send action. Settings shows
the latest 50 test attempts. Statuses mean:

- **Sending:** a durable attempt exists and its SMTP request may be in flight.
- **Accepted by email server:** SMTP acknowledged the complete message. This is
  not proof of Amazon processing or arrival on the Kindle.
- **Failed:** SMTP failed before acceptance, or SMTP explicitly
  rejected the message. Fix the reported issue and manually send again.
- **Outcome unknown:** the connection or process was interrupted and acceptance
  cannot be established. Check the Kindle before choosing to send again.

File/settings validation errors appear immediately without creating an SMTP
attempt. There is no automatic retry or automatic send-on-import. The browser preserves
an attempt ID in session storage after an HTTP failure so retrying the same
request returns its previous result. A fresh explicit send after a terminal
result creates a new attempt. Duplicate request IDs cannot send twice, and
concurrent sends of the same file to the same recipient are blocked. Interrupted
attempts become unknown after two minutes, even after a restart. No claim of
exactly-once delivery across SMTP and database failures is made.

## Configuration and credentials

The optional environment defaults are `SMTP_HOST`, `SMTP_PORT` (465),
`SMTP_TLS_MODE` (`implicit` or `starttls`), `SMTP_USERNAME`, `SMTP_PASSWORD`,
`SMTP_FROM`, `SMTP_FROM_NAME`, `LIBRARRY_KINDLE_EMAIL` and
`LIBRARRY_KINDLE_ENABLED` (false). `RESEND_API_KEY` supplies the password only
when `SMTP_PASSWORD` is absent. The application reads process environment; it
does not load a root `.env` automatically. Compose templates forward these values
from their environment file.

Once settings are saved in the UI, that saved snapshot takes precedence over
all environment defaults, including after restart. An empty password retains
the current secret; **Clear saved password** removes it (disable delivery first).
Passwords never appear in API responses or delivery history. As with existing
integration credentials, UI-saved SMTP secrets are stored in Postgres and can
be included in database backups. Protect database access and backup files.
Use the existing application authentication or an authenticated reverse proxy;
Kindle endpoints share the rest of the application's access policy.

API routes:

- `GET/PUT /api/v1/kindle/settings`: read redacted settings or save a complete
  settings snapshot; PUT accepts `clearPassword`.
- `POST /api/v1/kindle/test`: `{ "requestId": "unique-request-id-123" }`.
- `POST /api/v1/kindle/send`: `{ "wantedId": "...", "fileId": "...",
  "requestId": "unique-request-id-123" }`. Recipient and path cannot be supplied
  by the send request; the configured destination and linked file determine them.
- `GET /api/v1/kindle/deliveries?wantedId=...`: recent book attempts; omit
  `wantedId` for test attempts. Empty results are `[]`.

Migration `0059_kindle_delivery.sql` appends settings and durable attempt tables.
It does not change existing books, imports, or notification webhook behavior.
