FROM iotaledger/hornet:2.0.0-rc.1 AS hornet
FROM iotaledger/inx-indexer:1.0-rc AS indexer
FROM iotaledger/inx-faucet:1.0-rc AS faucet
FROM iotaledger/inx-dashboard:1.0-rc AS dashboard
FROM iotaledger/inx-mqtt:1.0-rc AS mqtt
FROM iotaledger/inx-spammer:1.0-rc AS spammer
FROM iotaledger/inx-coordinator:1.0-rc AS coordinator
FROM iotaledger/inx-poi:1.0-rc AS poi

FROM debian:11

# hornet-1 API
EXPOSE 14265/tcp

# hornet-1 dashboard
EXPOSE 8081/tcp

# hornet-1 INX
EXPOSE 9029/tcp

# faucet
EXPOSE 8091/tcp

RUN mkdir /app
WORKDIR /app

# Prepare supervisor
RUN apt update && apt install -y supervisor parallel
RUN mkdir -p /var/log/supervisor
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf


# Copy the inx apps dir into HORNET image
COPY --from=hornet /app /app/hornet-1
COPY --from=hornet /app /app/hornet-2
COPY --from=indexer /app /app/indexer
COPY --from=faucet /app /app/faucet
COPY --from=dashboard /app /app/dashboard
COPY --from=mqtt /app /app/mqtt
COPY --from=spammer /app /app/spammer
COPY --from=coordinator /app /app/coordinator
COPY --from=poi /app /app/poi

# Overwrite default config
COPY config.json /app/hornet-1/config.json
COPY config.json /app/hornet-2/config.json

# Create snapshots
COPY protocol_parameters.json /app/protocol_parameters.json
RUN mkdir /app/hornet-1/snapshots

RUN /app/hornet-1/hornet tool snap-gen \
	--protocolParametersPath=/app/protocol_parameters.json \
	--mintAddress=tst1qq2kvnu9pqzptkggrpqrvltvagccsh6aj2fkdhla7p3lrsy9dwhdzu5l2ye \
	--outputPath=/app/hornet-1/snapshots/full_snapshot.bin

RUN cp -R /app/hornet-1/snapshots /app/hornet-2/

# Bootstrap network
RUN mkdir /app/coordinator/state
RUN COO_PRV_KEYS=651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c,0e324c6ff069f31890d496e9004636fd73d8e8b5bea08ec58a4178ca85462325f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c /app/hornet-1/hornet tool bootstrap-private-tangle \
	--configFile=/app/hornet-1/config.json \
	--snapshotPath=/app/hornet-1/snapshots/full_snapshot.bin \
	--databasePath=/app/hornet-1/privatedb \
	--cooStatePath=/app/coordinator/state/coordinator.state


CMD ["/usr/bin/supervisord"]
