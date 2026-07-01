#!/bin/sh
set -e

if [ -d /output ]; then
  rsync -a --delete /static/ /output/
fi

exec "$@"
