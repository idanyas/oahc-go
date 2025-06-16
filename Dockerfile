version: '3.8'

services:
  oahc-go:
    build: .
    env_file:
      - ./.env
    volumes:
      # Mount OCI private key. Path inside container must match OCI_PRIVATE_KEY_FILENAME in .env
      - ./oci_api_key.pem:/app/oci_api_key.pem

      # Mount local directory for JSON logs.
      - ./logs:/var/log/oahc-go