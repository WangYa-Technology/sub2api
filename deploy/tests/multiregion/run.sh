#!/usr/bin/env bash
set -Eeuo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "${script_dir}/../../.." && pwd)
compose_file="${script_dir}/docker-compose.yml"
project_name="sub2api-multiregion-$$"
temp_dir=$(mktemp -d)
keep_env=${KEEP_ENV:-0}

cleanup() {
  status=$?
  rm -rf "${temp_dir}"
  if [[ "${keep_env}" == "1" ]]; then
    printf 'Keeping Compose project for inspection: %s\n' "${project_name}"
  else
    docker compose -p "${project_name}" -f "${compose_file}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  fi
  exit "${status}"
}
trap cleanup EXIT INT TERM

fail() {
  printf 'multi-region integration test failed: %s\n' "$1" >&2
  exit 1
}

step() {
  printf '\n==> %s\n' "$1"
}

compose() {
  docker compose -p "${project_name}" -f "${compose_file}" "$@"
}

service_port() {
  compose port "$1" 8080 | awk -F: 'END {print $NF}'
}

wait_http() {
  local domain=$1
  local port=$2
  local path=${3:-/health}
  local attempt
  for attempt in $(seq 1 90); do
    if curl -fsS --connect-timeout 2 --max-time 5 \
      --resolve "${domain}:${port}:127.0.0.1" \
      "http://${domain}:${port}${path}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  fail "${domain}${path} did not become ready"
}

request() {
  local domain=$1
  local method=$2
  local path=$3
  local token=$4
  local body=$5
  local output=$6
  local timeout=${7:-60}
  local args=(
    --silent --show-error --connect-timeout 3 --max-time "${timeout}"
    --resolve "${domain}:${ingress_port}:127.0.0.1"
    -X "${method}" -o "${output}" -w '%{http_code}'
    -H 'Accept: application/json'
  )
  if [[ -n "${token}" ]]; then
    args+=(-H "Authorization: Bearer ${token}")
  fi
  if [[ -n "${body}" ]]; then
    args+=(-H 'Content-Type: application/json' --data-binary "${body}")
  fi
  curl "${args[@]}" "http://${domain}:${ingress_port}${path}"
}

expect_status() {
  local expected=$1
  shift
  local actual
  actual=$(request "$@")
  [[ "${actual}" == "${expected}" ]] || fail "$2 $3 returned HTTP ${actual}, expected ${expected}"
}

json_value() {
  jq -er "$1" "$2" 2>/dev/null || fail "missing JSON value $1 in response"
}

wait_for_key_rejection() {
  local domain=$1
  local api_key=$2
  local output="${temp_dir}/rejected-key-${domain}.json"
  local status
  local attempt
  for attempt in $(seq 1 30); do
    status=$(request "${domain}" GET /v1/models "${api_key}" '' "${output}" 10)
    if [[ "${status}" == "401" || "${status}" == "403" ]]; then
      return 0
    fi
    sleep 1
  done
  fail "${domain} continued accepting a disabled API key after 30 seconds"
}

[[ -n "${HCTOPUP_API_KEY:-}" ]] || fail 'HCTOPUP_API_KEY is required and must be injected through the environment'
command -v docker >/dev/null || fail 'docker is not installed'
command -v curl >/dev/null || fail 'curl is not installed'
command -v jq >/dev/null || fail 'jq is not installed'

cd "${repo_root}"

step "Build the current source and start shared PostgreSQL plus regional Redis servers"
compose build jp-01
compose up -d --wait --wait-timeout 90 postgres redis-japan redis-us redis-taiwan

step "Initialize the shared database through jp-01, then join the remaining nodes"
compose up -d --no-deps --wait --wait-timeout 180 jp-01
jp01_port=$(service_port jp-01)
wait_http 127.0.0.1 "${jp01_port}"
compose up -d --no-deps --wait --wait-timeout 180 jp-02 us-01 tw-01
compose up -d --no-build --wait --wait-timeout 90 ingress
ingress_port=$(service_port ingress)
printf 'Compose project: %s\nIngress port: %s\n' "${project_name}" "${ingress_port}"
wait_http jp.sub2api.test "${ingress_port}"
wait_http us.sub2api.test "${ingress_port}"
wait_http tw.sub2api.test "${ingress_port}"

step "Verify shared database, shared Japan Redis, and isolated US/Taiwan Redis"
for service in jp-01 jp-02 us-01 tw-01; do
  db_host=$(compose exec -T "${service}" sh -c 'printf %s "$DATABASE_HOST"')
  [[ "${db_host}" == "postgres" ]] || fail "${service} does not use the shared database"
done
[[ $(compose exec -T jp-01 sh -c 'printf %s "$REDIS_HOST"') == redis-japan ]] || fail 'jp-01 Redis mismatch'
[[ $(compose exec -T jp-02 sh -c 'printf %s "$REDIS_HOST"') == redis-japan ]] || fail 'jp-02 does not share Japan Redis'
[[ $(compose exec -T us-01 sh -c 'printf %s "$REDIS_HOST"') == redis-us ]] || fail 'us-01 Redis mismatch'
[[ $(compose exec -T tw-01 sh -c 'printf %s "$REDIS_HOST"') == redis-taiwan ]] || fail 'tw-01 Redis mismatch'
compose exec -T redis-japan redis-cli SET topology:region japan >/dev/null
compose exec -T redis-us redis-cli SET topology:region us >/dev/null
compose exec -T redis-taiwan redis-cli SET topology:region taiwan >/dev/null
[[ $(compose exec -T redis-japan redis-cli --raw GET topology:region) == japan ]] || fail 'Japan Redis marker mismatch'
[[ $(compose exec -T redis-us redis-cli --raw GET topology:region) == us ]] || fail 'US Redis is not isolated'
[[ $(compose exec -T redis-taiwan redis-cli --raw GET topology:region) == taiwan ]] || fail 'Taiwan Redis is not isolated'

step "Create an OpenAI group/account and a normal user through the Japan domain"
admin_login_body='{"email":"multiregion-admin@sub2api.test","password":"MultiregionAdminTest123!"}'
expect_status 200 jp.sub2api.test POST /api/v1/auth/login '' "${admin_login_body}" "${temp_dir}/admin-login.json"
admin_token=$(json_value '.data.access_token' "${temp_dir}/admin-login.json")

compliance_body='{"phrase":"I have read, understood, and agree to the Sub2API Deployment and Operation Compliance Commitment","language":"en"}'
expect_status 200 jp.sub2api.test POST /api/v1/admin/compliance/accept "${admin_token}" "${compliance_body}" "${temp_dir}/compliance.json"

group_body='{"name":"multiregion-openai","description":"Docker multi-region integration group","platform":"openai","rate_multiplier":1,"is_exclusive":false}'
expect_status 200 jp.sub2api.test POST /api/v1/admin/groups "${admin_token}" "${group_body}" "${temp_dir}/group.json"
group_id=$(json_value '.data.id' "${temp_dir}/group.json")

account_body=$(jq -cn \
  --arg key "${HCTOPUP_API_KEY}" \
  --argjson group_id "${group_id}" \
  '{name:"hctopup-multiregion",platform:"openai",type:"apikey",credentials:{api_key:$key,base_url:"https://api.hctopup.com/v1"},extra:{model_mapping:{"gpt-5.4-mini":"gpt-5.4-mini"}},concurrency:4,priority:10,group_ids:[$group_id]}')
expect_status 200 jp.sub2api.test POST /api/v1/admin/accounts "${admin_token}" "${account_body}" "${temp_dir}/account.json" 90
account_body=''

user_email="multiregion-user-${project_name}@sub2api.test"
user_body=$(jq -cn --arg email "${user_email}" --argjson group_id "${group_id}" \
  '{email:$email,password:"MultiregionUserTest123!",username:"multiregion-user",role:"user",balance:20,concurrency:4,allowed_groups:[$group_id]}')
expect_status 200 jp.sub2api.test POST /api/v1/admin/users "${admin_token}" "${user_body}" "${temp_dir}/user.json"

step "Login through the independent US domain and create a user API key"
user_login_body=$(jq -cn --arg email "${user_email}" '{email:$email,password:"MultiregionUserTest123!"}')
expect_status 200 us.sub2api.test POST /api/v1/auth/login '' "${user_login_body}" "${temp_dir}/user-login.json"
user_token=$(json_value '.data.access_token' "${temp_dir}/user-login.json")
[[ $(json_value '.data.user.email' "${temp_dir}/user-login.json") == "${user_email}" ]] || fail 'US login returned the wrong shared-database user'

key_body=$(jq -cn --argjson group_id "${group_id}" '{name:"multiregion-client-key",group_id:$group_id}')
expect_status 200 us.sub2api.test POST /api/v1/keys "${user_token}" "${key_body}" "${temp_dir}/api-key.json"
client_key=$(json_value '.data.key' "${temp_dir}/api-key.json")
client_key_id=$(json_value '.data.id' "${temp_dir}/api-key.json")

step "Use the same user key through Japan and US against the real OpenAI-compatible upstream"
responses_body='{"model":"gpt-5.4-mini","input":"Reply with exactly OK.","max_output_tokens":32,"stream":false}'
expect_status 200 jp.sub2api.test POST /v1/responses "${client_key}" "${responses_body}" "${temp_dir}/jp-response.json" 180
json_value '.id' "${temp_dir}/jp-response.json" >/dev/null
expect_status 200 us.sub2api.test POST /v1/responses "${client_key}" "${responses_body}" "${temp_dir}/us-response.json" 180
json_value '.id' "${temp_dir}/us-response.json" >/dev/null
expect_status 200 tw.sub2api.test GET /v1/models "${client_key}" '' "${temp_dir}/tw-models.json"

usage_count=$(compose exec -T postgres psql -U sub2api -d sub2api -Atqc 'SELECT COUNT(*) FROM usage_logs')
[[ "${usage_count}" -ge 2 ]] || fail "expected at least two persisted usage records, got ${usage_count}"

step "Verify all node metrics and background-task eligibility through the admin API"
node_metrics_ready=0
for _ in $(seq 1 30); do
  status=$(request jp.sub2api.test GET /api/v1/admin/ops/dashboard/overview "${admin_token}" '' "${temp_dir}/ops.json" 15)
  if [[ "${status}" == "200" ]] && [[ $(jq -r '.data.node_metrics | length' "${temp_dir}/ops.json") -eq 4 ]]; then
    node_metrics_ready=1
    break
  fi
  sleep 1
done
[[ "${node_metrics_ready}" == "1" ]] || fail 'ops dashboard did not report all four nodes'
jq -e '
  (.data.node_metrics | map(.node_id) | sort) == ["jp-01","jp-02","tw-01","us-01"] and
  ([.data.node_metrics[] | select(.node_id | startswith("jp-")) | .background_tasks_disabled] | all(. == false)) and
  ([.data.node_metrics[] | select(.node_id == "us-01" or .node_id == "tw-01") | .background_tasks_disabled] | all(. == true)) and
  ([.data.node_metrics[] | .db_ok] | all(. == true)) and
  ([.data.node_metrics[] | .redis_ok] | all(. == true))
' "${temp_dir}/ops.json" >/dev/null || fail 'node metrics or background-task roles are incorrect'

invalidation_scopes=$(compose exec -T postgres psql -U sub2api -d sub2api -Atqc \
  "SELECT string_agg(consumer_scope, ',' ORDER BY consumer_scope) FROM auth_cache_invalidation_consumers")
[[ "${invalidation_scopes}" == "japan-shared-redis,taiwan-independent-redis,us-independent-redis" ]] || \
  fail "auth invalidation consumers do not cover every Redis scope: ${invalidation_scopes}"

step "Verify PostgreSQL advisory-lock owners are limited to eligible Japan nodes"
db_network_id=$(docker network ls -q \
  --filter "label=com.docker.compose.project=${project_name}" \
  --filter 'label=com.docker.compose.network=db-plane' | head -n 1)
[[ -n "${db_network_id}" ]] || fail 'cannot locate the Compose database network'
jp01_id=$(compose ps -q jp-01)
jp02_id=$(compose ps -q jp-02)
jp01_ip=$(docker inspect -f "{{(index .NetworkSettings.Networks \"${project_name}_db-plane\").IPAddress}}" "${jp01_id}")
jp02_ip=$(docker inspect -f "{{(index .NetworkSettings.Networks \"${project_name}_db-plane\").IPAddress}}" "${jp02_id}")
lock_owners=$(compose exec -T postgres psql -U sub2api -d sub2api -Atqc \
  "SELECT DISTINCT a.client_addr::text FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE l.locktype='advisory' AND l.granted AND a.client_addr IS NOT NULL ORDER BY 1")
[[ -n "${lock_owners}" ]] || fail 'no global background advisory locks were held'
while IFS= read -r owner; do
  owner=${owner%/32}
  [[ "${owner}" == "${jp01_ip}" || "${owner}" == "${jp02_ip}" ]] || fail "ineligible node ${owner} owns a global advisory lock"
done <<<"${lock_owners}"

step "Roll jp-02 while continuously checking the Japan ingress"
rolling_failures="${temp_dir}/rolling-failures"
: >"${rolling_failures}"
(
  for _ in $(seq 1 80); do
    if ! curl -fsS --connect-timeout 1 --max-time 3 \
      --resolve "jp.sub2api.test:${ingress_port}:127.0.0.1" \
      "http://jp.sub2api.test:${ingress_port}/health" >/dev/null; then
      printf 'failed\n' >>"${rolling_failures}"
    fi
    sleep 0.1
  done
) &
health_loop_pid=$!
compose up -d --no-deps --force-recreate jp-02 >/dev/null
wait "${health_loop_pid}"
[[ ! -s "${rolling_failures}" ]] || fail 'Japan ingress returned an error during jp-02 rolling restart'
wait_http 127.0.0.1 "$(service_port jp-02)"

step "Verify cross-Redis API-key invalidation"
expect_status 200 us.sub2api.test GET /v1/models "${client_key}" '' "${temp_dir}/models-before-disable.json"
disable_body='{"status":"inactive"}'
expect_status 200 jp.sub2api.test PUT "/api/v1/keys/${client_key_id}" "${user_token}" "${disable_body}" "${temp_dir}/key-disabled.json"
wait_for_key_rejection us.sub2api.test "${client_key}"
wait_for_key_rejection tw.sub2api.test "${client_key}"

step "Verify crashed process slots are reclaimed within the heartbeat recovery window"
# Let the just-replaced jp-02 process refresh its heartbeat twice. Selecting only
# recently refreshed index members excludes the previous rolling-release process.
sleep 20
heartbeat_cutoff=$(compose exec -T redis-japan redis-cli --raw TIME | head -n 1 | tr -d '\r')
heartbeat_cutoff=$((heartbeat_cutoff - 15))
heartbeat_prefixes=()
while IFS= read -r prefix; do
  [[ -n "${prefix}" ]] && heartbeat_prefixes+=("${prefix}")
done < <(compose exec -T redis-japan redis-cli --raw ZRANGEBYSCORE concurrency:process:heartbeat_index "${heartbeat_cutoff}" +inf | tr -d '\r')
[[ "${#heartbeat_prefixes[@]}" -ge 2 ]] || fail 'expected heartbeats from both Japan nodes'
compose kill -s KILL jp-02 >/dev/null
dead_prefix=''
for _ in $(seq 1 45); do
  for prefix in "${heartbeat_prefixes[@]}"; do
    exists=$(compose exec -T redis-japan redis-cli --raw EXISTS "concurrency:process:heartbeat:${prefix}" | tr -d '\r')
    if [[ "${exists}" == "0" ]]; then
      dead_prefix=${prefix}
      break 2
    fi
  done
  sleep 1
done
[[ -n "${dead_prefix}" ]] || fail 'killed process heartbeat did not expire'
fake_account_id=900000
fake_request_id="${dead_prefix}-docker-crash-test"
redis_now=$(compose exec -T redis-japan redis-cli --raw TIME | head -n 1 | tr -d '\r')
compose exec -T redis-japan redis-cli ZADD "concurrency:account:${fake_account_id}" "${redis_now}" "${fake_request_id}" >/dev/null
compose exec -T redis-japan redis-cli EXPIRE "concurrency:account:${fake_account_id}" 900 >/dev/null
compose exec -T redis-japan redis-cli ZADD concurrency:account:active_index "$((redis_now + 900))" "${fake_account_id}" >/dev/null
compose exec -T redis-japan redis-cli SADD "concurrency:process:accounts:${dead_prefix}" "${fake_account_id}" >/dev/null
compose exec -T redis-japan redis-cli EXPIRE "concurrency:process:accounts:${dead_prefix}" 900 >/dev/null

slot_reclaimed=0
for _ in $(seq 1 50); do
  if [[ $(compose exec -T redis-japan redis-cli --raw ZSCORE "concurrency:account:${fake_account_id}" "${fake_request_id}" | tr -d '\r') == '' ]]; then
    slot_reclaimed=1
    break
  fi
  sleep 1
done
[[ "${slot_reclaimed}" == "1" ]] || fail 'dead process slot was not reclaimed within the expected window'
compose up -d --no-deps jp-02 >/dev/null
wait_http 127.0.0.1 "$(service_port jp-02)"

step "All multi-region Docker integration checks passed"
printf 'Validated domains: jp.sub2api.test, us.sub2api.test, tw.sub2api.test\n'
printf 'Validated model: gpt-5.4-mini via https://api.hctopup.com/v1\n'
