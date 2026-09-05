#!/usr/bin/env bash

set -Eeuo pipefail

usage() {
	cat >&2 <<'EOF'
Usage: run-integration-tests.sh <minio|rustfs|garage|seaweedfs> [mc-binary]

The second argument may be omitted when running locally; a temporary host-native
mc binary will be built automatically. Docker is required for the S3 backend.
EOF
}

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
	usage
	exit 2
fi

backend="$1"
case "$backend" in
	minio | rustfs | garage | seaweedfs) ;;
	*)
		echo "Unsupported backend: $backend" >&2
		usage
		exit 2
		;;
esac

if ! command -v docker >/dev/null 2>&1; then
	echo "Docker is required to run integration tests" >&2
	exit 1
fi

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/mc-integration.XXXXXX")"
container_name="mc-integration-${backend}-$$"
host_port="${MC_INTEGRATION_PORT:-9000}"
access_key="${MC_TEST_ACCESS_KEY:-mc-integration}"
secret_key="${MC_TEST_SECRET_KEY:-mc-integration-secret}"

MINIO_IMAGE="${MINIO_IMAGE:-minio/minio:RELEASE.2025-09-07T16-13-09Z}"
RUSTFS_IMAGE="${RUSTFS_IMAGE:-rustfs/rustfs:1.0.0-rc.5}"
GARAGE_IMAGE="${GARAGE_IMAGE:-dxflrs/garage:v2.2.0}"
SEAWEEDFS_IMAGE="${SEAWEEDFS_IMAGE:-chrislusf/seaweedfs:4.45}"

service_endpoint="127.0.0.1:$host_port"
service_protocol="http"
skip_insecure=false
skip_presigned_post=false
skip_storage_class_check=false
skip_object_tagging=false
skip_alias_error_check=false
skip_storage_class_error=false
presigned_post_error="MethodNotAllowed"
skip_watch=false
skip_config_error=false

cleanup() {
	status=$?
	if [ "$status" -ne 0 ] && docker ps -a --format '{{.Names}}' | grep -Fxq "$container_name"; then
		echo "${backend} container logs:" >&2
		docker logs "$container_name" >&2 || true
	fi
	docker rm --force "$container_name" >/dev/null 2>&1 || true
	rm -rf "$work_dir"
	exit "$status"
}
trap cleanup EXIT

start_minio() {
	local cert_dir="$work_dir/minio-certs"
	mkdir -p "$cert_dir"
	cp "$repo_root/testdata/localhost.crt" "$cert_dir/public.crt"
	cp "$repo_root/testdata/localhost.key" "$cert_dir/private.key"

	docker run --detach --name "$container_name" \
		--publish "$host_port:9000" \
		--tmpfs /data \
		--volume "$cert_dir:/root/.minio/certs:ro" \
		--env MINIO_ROOT_USER="$access_key" \
		--env MINIO_ROOT_PASSWORD="$secret_key" \
		"$MINIO_IMAGE" server /data >/dev/null

	service_protocol=https
	skip_insecure=true
}

start_rustfs() {
	docker run --detach --name "$container_name" \
		--publish "$host_port:9000" \
		--tmpfs /data:uid=10001,gid=10001,mode=755 \
		--tmpfs /logs:uid=10001,gid=10001,mode=755 \
		--env RUSTFS_ACCESS_KEY="$access_key" \
		--env RUSTFS_SECRET_KEY="$secret_key" \
		--env RUSTFS_ADDRESS=":9000" \
		"$RUSTFS_IMAGE" >/dev/null
}

start_garage() {
	# Garage requires an initialized layout and explicit bucket/key permissions
	# before its S3 API can be used by the shared integration suite.
	skip_storage_class_check=true
	skip_object_tagging=true
	skip_alias_error_check=true
	skip_storage_class_error=true
	presigned_post_error="InvalidRequest"
	skip_watch=true
	skip_config_error=true
	local garage_dir="$work_dir/garage"
	mkdir -p "$garage_dir/meta" "$garage_dir/data"
	cat >"$garage_dir/garage.toml" <<'EOF'
metadata_dir = "/var/lib/garage/meta"
data_dir = "/var/lib/garage/data"
db_engine = "sqlite"
replication_factor = 1
rpc_bind_addr = "[::]:3901"
rpc_public_addr = "127.0.0.1:3901"
rpc_secret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

[s3_api]
s3_region = "us-east-1"
api_bind_addr = "[::]:3900"
root_domain = ".s3.garage.localhost"

[admin]
api_bind_addr = "[::]:3903"
admin_token = "mc-integration-admin-token"
metrics_token = "mc-integration-metrics-token"
EOF

	docker run --detach --name "$container_name" \
		--publish "$host_port:3900" \
		--volume "$garage_dir/garage.toml:/etc/garage.toml:ro" \
		--tmpfs /var/lib/garage/meta \
		--tmpfs /var/lib/garage/data \
		"$GARAGE_IMAGE" /garage server >/dev/null

	for _ in $(seq 1 30); do
		if docker exec "$container_name" /garage status >/dev/null 2>&1; then
			break
		fi
		sleep 1
	done

	node_id="$(docker exec "$container_name" /garage status \
		| awk '$1 ~ /^[0-9a-f]+$/ { print $1; exit }')"
	if [ -z "$node_id" ]; then
		echo "Unable to determine Garage node ID" >&2
		exit 1
	fi

	docker exec "$container_name" /garage layout assign -z ci -c 1G "$node_id" >/dev/null
	docker exec "$container_name" /garage layout apply --version 1 >/dev/null
	docker exec "$container_name" /garage bucket create mc-integration-bootstrap >/dev/null

	key_output="$(docker exec "$container_name" /garage key create mc-integration-key)"
	access_key="$(awk '/Key ID/ { print $NF; exit }' <<<"$key_output")"
	secret_key="$(awk '/Secret key/ { print $NF; exit }' <<<"$key_output")"
	if [ -z "$access_key" ] || [ -z "$secret_key" ]; then
		echo "Unable to create Garage integration credentials" >&2
		exit 1
	fi
	docker exec "$container_name" /garage key allow --create-bucket mc-integration-key >/dev/null
	docker exec "$container_name" /garage bucket allow \
		--read --write --owner mc-integration-bootstrap \
		--key mc-integration-key >/dev/null
}

start_seaweedfs() {
	# SeaweedFS Mini provisions a local S3 endpoint and uses the AWS credential
	# environment variables as its initial administrator credentials.
	skip_presigned_post=true
	skip_watch=true
	docker run --detach --name "$container_name" \
		--publish "$host_port:8333" \
		--tmpfs /data \
		--env AWS_ACCESS_KEY_ID="$access_key" \
		--env AWS_SECRET_ACCESS_KEY="$secret_key" \
		--env S3_BUCKET=mc-integration-bootstrap \
		"$SEAWEEDFS_IMAGE" mini -dir=/data >/dev/null
}

case "$backend" in
	minio) start_minio ;;
	rustfs) start_rustfs ;;
	garage) start_garage ;;
	seaweedfs) start_seaweedfs ;;
esac

echo "Waiting for $backend at ${service_protocol}://${service_endpoint}"
for _ in $(seq 1 90); do
	curl_args=(--silent --show-error --max-time 2 --output /dev/null)
	if [ "$skip_insecure" = true ]; then
		curl_args+=(--insecure)
	fi
	if curl "${curl_args[@]}" "${service_protocol}://${service_endpoint}/"; then
		break
	fi
	sleep 1
done

curl_args=(--silent --show-error --max-time 2 --output /dev/null)
if [ "$skip_insecure" = true ]; then
	curl_args+=(--insecure)
fi
if ! curl "${curl_args[@]}" "${service_protocol}://${service_endpoint}/"; then
	echo "Timed out waiting for $backend" >&2
	exit 1
fi

binary_path="${2:-${MC_BINARY_PATH:-}}"
if [ -z "$binary_path" ]; then
	binary_path="$work_dir/mc"
	(
		cd "$repo_root"
		GO111MODULE=on go build -trimpath -tags kqueue -o "$binary_path" .
	)
fi

if [ ! -x "$binary_path" ]; then
	echo "mc binary is missing or not executable: $binary_path" >&2
	exit 1
fi

export ACCESS_KEY="$access_key"
export SECRET_KEY="$secret_key"
export ENABLE_HTTPS=0
export SERVER_ENDPOINT="$service_endpoint"
export MC_BINARY_PATH="$binary_path"
export MC_TEST_ACCESS_KEY="$access_key"
export MC_TEST_ENABLE_HTTPS=false
export MC_TEST_RUN_FULL_SUITE=true
export MC_TEST_SERVER_ENDPOINT="$service_endpoint"
export MC_TEST_SECRET_KEY="$secret_key"
export MC_TEST_SKIP_SSEC_HTTP=true
export MC_TEST_SKIP_STORAGE_CLASS_CHECK="$skip_storage_class_check"
export MC_TEST_SKIP_OBJECT_TAGGING="$skip_object_tagging"
export MC_TEST_SKIP_ALIAS_ERROR="$skip_alias_error_check"
export MC_TEST_SKIP_STORAGE_CLASS_ERROR="$skip_storage_class_error"
export MC_TEST_PRESIGNED_POST_ERROR="$presigned_post_error"
export MC_TEST_SKIP_WATCH="$skip_watch"
export MC_TEST_SKIP_CONFIG_ERROR="$skip_config_error"
export MC_TEST_SKIP_BUILD=true
export MC_TEST_SKIP_INSECURE="$skip_insecure"

if [ "$service_protocol" = https ]; then
	export ENABLE_HTTPS=1
	export MC_TEST_ENABLE_HTTPS=true
	export MC_TEST_SKIP_SSEC_HTTP=false
fi

cd "$repo_root"
export MC_TEST_SKIP_PRESIGNED_POST="$skip_presigned_post"
go test -count=1 -race -v --timeout 20m ./... -run Test_FullSuite
./functional-tests.sh
