#!/bin/sh
set -e

./create-cluster start
./create-cluster create -f

exec "$@"
