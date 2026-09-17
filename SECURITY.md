# Security reporting

Please do not post credentials, database dumps, private library data or details
of an unpatched exploit in a public issue or pull request.

## Contact

Check the repository's [security page](https://github.com/bandoracer/librarry/security).
If GitHub offers **Report a vulnerability**, use that private reporting flow.
Private vulnerability reporting was disabled when checked on September 16, 2026;
this document does not imply that a private inbox is available.

When that option is unavailable and you do not already have a private maintainer
contact, open an [issue](https://github.com/bandoracer/librarry/issues/new) titled
**Request for private security contact**. Include only a request to arrange a
private channel. Wait for that channel before sharing technical details. Do not
include the vulnerable endpoint, payload, logs or affected user's information.

Once a private channel is established, include the affected commit/image,
deployment conditions, impact and a minimal reproduction using controlled data.
There is no guaranteed response or remediation timeline.

## Support scope

Librarry is early alpha. Historical images and tags are not maintained security
support branches, and backports are not guaranteed. Reports should identify the
actual image digest or commit rather than only `latest`. See
[current status](docs/status.md) for candidate and deployment boundaries.

## Deployment basics

Replace credential placeholders before first start. Use Forms authentication for
browser access and a separate API key for compatible clients/feeds where needed.
Keep the API and PostgreSQL private on the container network, configure media
permissions deliberately, and back up database and media before migrations.
Follow the [deployment guide](docs/deployment.md#security) for details.

A clean configured image scan is limited to that scanner, database and policy;
it is not a guarantee that every vulnerability is absent.
