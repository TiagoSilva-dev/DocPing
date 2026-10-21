# MySQL for Railway, serving a certificate that carries the service's private
# hostname so the API can verify it fully. See mysql-tls-entrypoint.sh.
#
# Railway's V2 runtime executes a service's start command as argv, without a
# shell, so the certificate bootstrap cannot live in that field: it lives here,
# as a reviewable file, and the service leaves its start command empty.
FROM mysql@sha256:0f51e63e537a1f176423fa03ef559aabe6fa4372a53160a44bc7e75da7d15364
COPY mysql-tls-entrypoint.sh /usr/local/bin/mysql-tls-entrypoint.sh
ENTRYPOINT ["/usr/local/bin/mysql-tls-entrypoint.sh"]
# The data directory sits one level below the volume root so the certificates
# can live beside it: mysqld refuses to initialise a data directory that
# contains anything else. The remaining flags are the ones the Railway MySQL
# template used, kept so this image is a drop-in replacement for it.
CMD ["mysqld", "--datadir=/var/lib/mysql/data", "--innodb-use-native-aio=0", "--disable-log-bin", "--performance_schema=0", "--innodb-buffer-pool-size=1G"]
