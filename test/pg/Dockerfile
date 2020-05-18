FROM postgres:9.6

ENV POSTGRES_PASSWORD postgres

RUN mkdir -p /docker-entrypoint-initdb.d

COPY schema.sql /docker-entrypoint-initdb.d/
COPY rotate_password /usr/local/bin/
