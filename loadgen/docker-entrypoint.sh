set -euo pipefail

wait-for-it --timeout=60 mysql:3306

cd /loadgen

tail -f load.py