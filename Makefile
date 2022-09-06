NAME=preset
BINARY=terraform-provider-${NAME}

build:
	go build -o ${BINARY}

build-client:
	cd client && oapi-codegen -config config.yaml superset_openapi.json