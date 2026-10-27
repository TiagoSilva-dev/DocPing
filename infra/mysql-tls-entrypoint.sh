#!/bin/sh
# The stock MySQL image generates a server certificate carrying no name at all,
# so a client doing full verification -- which is what the API requires outside
# development -- can never match it against the host it dialled. This mints a
# CA and a server certificate for the service's private hostname on first boot
# and keeps both on the data volume, so the trust anchor survives restarts.
#
# The certificates sit beside the data directory rather than inside it: mysqld
# refuses to initialise a data directory that holds anything else, so the image
# moves the data one level down (see --datadir in mysql.Dockerfile) and keeps
# /var/lib/mysql as the volume root shared by both.
#
# Only the CA certificate is printed, and it is public by nature: it is how the
# API learns which anchor to trust (MYSQL_CA_CERT). The private keys are
# created here and never leave the volume.
set -e

CERTS=/var/lib/mysql/certs
HOST=${MYSQL_TLS_HOST:-mysql.railway.internal}

if [ ! -f "$CERTS/server-cert.pem" ]; then
	mkdir -p "$CERTS"
	openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
		-keyout "$CERTS/ca-key.pem" -out "$CERTS/ca.pem" \
		-subj "/CN=DocPing Internal CA" \
		-addext "basicConstraints=critical,CA:TRUE" \
		-addext "keyUsage=critical,keyCertSign"
	openssl req -newkey rsa:2048 -nodes \
		-keyout "$CERTS/server-key.pem" -out "$CERTS/server.csr" \
		-subj "/CN=$HOST"
	cat > "$CERTS/ext.cnf" <<EOF
basicConstraints=CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:$HOST
EOF
	openssl x509 -req -in "$CERTS/server.csr" \
		-CA "$CERTS/ca.pem" -CAkey "$CERTS/ca-key.pem" -CAcreateserial \
		-out "$CERTS/server-cert.pem" -days 3650 -extfile "$CERTS/ext.cnf"
fi

chown -R mysql:mysql "$CERTS"
chmod 600 "$CERTS"/*-key.pem

echo DOCPING_CA_BEGIN
cat "$CERTS/ca.pem"
echo DOCPING_CA_END

exec docker-entrypoint.sh "$@" \
	--ssl-ca="$CERTS/ca.pem" \
	--ssl-cert="$CERTS/server-cert.pem" \
	--ssl-key="$CERTS/server-key.pem"
