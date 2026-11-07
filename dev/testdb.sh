#!/usr/bin/env bash
# Creates, seeds, or drops the Querra test databases on database servers you
# already run (no Docker needed). Everything it creates is named querra_test*,
# so it never touches other databases or users on the server.
#
# Usage: dev/testdb.sh reset|drop postgres|mysql|mariadb
#
# Connection settings come from QUERRA_DEV_* environment variables;
# `wails3 task db:reset` loads them from dev/test.env (see dev/test.env.example).
# An engine whose HOST is empty is skipped.
set -euo pipefail

usage="usage: dev/testdb.sh reset|drop postgres|mysql|mariadb"
action=${1:?$usage}
engine=${2:?$usage}
[[ $action == reset || $action == drop ]] || { echo "$usage" >&2; exit 2; }
seed_dir="$(cd "$(dirname "$0")" && pwd)/seed"

# Fixed test database and credentials. Integration tests connect with these.
db=querra_test
user=querra_test
password=querra_test

require() {
  command -v "$1" > /dev/null || { echo "$engine: '$1' not found on PATH" >&2; exit 1; }
}

reset_postgres() {
  local host=${QUERRA_DEV_POSTGRES_HOST:-} port=${QUERRA_DEV_POSTGRES_PORT:-5432}
  if [[ -z $host ]]; then
    echo "postgres: skipped (QUERRA_DEV_POSTGRES_HOST is not set)"
    return
  fi
  require psql

  export PGPASSWORD=${QUERRA_DEV_POSTGRES_ADMIN_PASSWORD:-}
  export PGOPTIONS="-c client_min_messages=warning"
  local admin=(psql -X -q -w -v ON_ERROR_STOP=1 -h "$host" -p "$port"
    -U "${QUERRA_DEV_POSTGRES_ADMIN_USER:-postgres}" -d postgres)

  "${admin[@]}" -c "DROP DATABASE IF EXISTS $db WITH (FORCE)" -c "DROP ROLE IF EXISTS $user"
  if [[ $action == drop ]]; then
    echo "postgres: dropped database $db and role $user"
    return
  fi

  "${admin[@]}" -c "CREATE ROLE $user LOGIN PASSWORD '$password'" -c "CREATE DATABASE $db OWNER $user"
  # Seed as the test role so it owns every object it creates.
  PGPASSWORD=$password psql -X -q -w -v ON_ERROR_STOP=1 -h "$host" -p "$port" -U "$user" -d "$db" \
    -f "$seed_dir/postgres/01-seed.sql"
  echo "postgres: database $db reset and seeded"
}

# MySQL and MariaDB share the seed and the steps; $1 is the env var prefix.
reset_mysql_like() {
  local prefix=$1
  local host_var=${prefix}_HOST port_var=${prefix}_PORT user_var=${prefix}_ADMIN_USER pass_var=${prefix}_ADMIN_PASSWORD
  local host=${!host_var:-} port=${!port_var:-3306}
  if [[ -z $host ]]; then
    echo "$engine: skipped ($host_var is not set)"
    return
  fi
  require mysql

  export MYSQL_PWD=${!pass_var:-}
  local admin=(mysql --protocol=TCP -h "$host" -P "$port" -u "${!user_var:-root}"
    --default-character-set=utf8mb4)

  "${admin[@]}" -e "DROP DATABASE IF EXISTS $db; DROP DATABASE IF EXISTS ${db}_sales; DROP USER IF EXISTS '$user'@'%';"
  if [[ $action == drop ]]; then
    echo "$engine: dropped databases $db, ${db}_sales and user $user"
    return
  fi

  # Explicit utf8mb4: MariaDB 11.4 still defaults to latin1.
  "${admin[@]}" -e "CREATE USER '$user'@'%' IDENTIFIED BY '$password';
    CREATE DATABASE $db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
    GRANT ALL PRIVILEGES ON $db.* TO '$user'@'%';
    GRANT ALL PRIVILEGES ON ${db}_sales.* TO '$user'@'%';"
  # Seed as admin: MySQL's binary-logging rules require it for CREATE FUNCTION.
  "${admin[@]}" "$db" < "$seed_dir/mysql/01-seed.sql" > /dev/null
  echo "$engine: databases $db and ${db}_sales reset and seeded"
}

case $engine in
  postgres) reset_postgres ;;
  mysql) reset_mysql_like QUERRA_DEV_MYSQL ;;
  mariadb) reset_mysql_like QUERRA_DEV_MARIADB ;;
  *) echo "$usage" >&2; exit 2 ;;
esac
