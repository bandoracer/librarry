#!/bin/sh
set -eu
calibredb list --with-library /library >/dev/null
calibre-server --userdb /users.sqlite --manage-users -- add fixture fixture-password
exec calibre-server /library --listen-on 0.0.0.0 --port 8080 --enable-auth --auth-mode "${LIBRARRY_CALIBRE_AUTH_MODE:-auto}" --userdb /users.sqlite --disable-use-bonjour
