set -euo pipefail

wait-for-it --timeout=60 mysql:3306

cd /loadgen

go run main.go
