FROM postgres:17-alpine

RUN apk add --no-cache restic

COPY scripts/backup-postgres.sh /usr/local/bin/backup-postgres
COPY scripts/restore-drill.sh /usr/local/bin/restore-drill

RUN chmod 0555 /usr/local/bin/backup-postgres /usr/local/bin/restore-drill

ENTRYPOINT []
