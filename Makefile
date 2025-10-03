# valkey commands
start-valkey:
	podman compose up valkey

start-valkey-cli:
	podman compose run --rm valkey-cli

# minio commands
start-minio:
	podman compose up minio

create-minio-buckets:
	podman compose up minio-createbuckets

# BUILD commands
build:
	go build

# test upload commands
minio-test-upload:
	./valpop populate -r testapp -s impl --timeout 10 --bucket frontend --hostname 127.0.0.1 --port 9000 --username bWluaW9hZG1pbg== --password bWluaW9hZG1pbg==

valkey-test-upload:
	./valpop populate -m valkey -r testapp -s impl --timeout 10 --bucket frontend --hostname 127.0.0.1 --port 6379
