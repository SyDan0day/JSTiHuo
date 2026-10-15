#!/bin/sh
set -eu
umask 077
mkdir -p /tmp/jst-secrets
# Copy only secrets required by this process, never the root password or SQL.
if [ "$1" = migrate ] || [ "$1" = admin ]; then
  cp /run/bootstrap-secrets/migration_dsn /tmp/jst-secrets/
else
  for name in app_dsn pii.json export.json lookup request challenge; do
    cp "/run/bootstrap-secrets/$name" /tmp/jst-secrets/
  done
fi
chown -R app:app /tmp/jst-secrets
if [ "$1" = api ] || [ "$1" = worker ]; then
  mkdir -p /var/lib/jst/files
  chown app:app /var/lib/jst/files
  su-exec app chmod 700 /var/lib/jst/files
fi
exec su-exec app "$@"
