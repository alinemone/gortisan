#!/bin/sh
# Starts layout1 with the command given to the container (serve by default).
# With RUN_MIGRATIONS=true it applies the pending migrations first; replicas
# that start together wait for each other (a Postgres advisory lock).
set -e

if [ "$RUN_MIGRATIONS" = "true" ]; then
	/usr/local/bin/layout1 migrate up
fi

exec /usr/local/bin/layout1 "$@"
