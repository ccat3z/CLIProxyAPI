#! /bin/sh

set -ex
cd /data
if [ ! -f config.yaml ]; then
  touch config.yaml
fi

export HOME="$PWD"
exec /app/server "$@"
