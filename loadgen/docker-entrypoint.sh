#!/bin/bash
echo "pipefail"
set -euo pipefail

echo "waitforit"
wait-for-it --timeout=60 mysql:3306

echo "laodgen"
cd /loadgen

echo "tail"
tail -f load.py