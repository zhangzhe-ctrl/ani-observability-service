FROM gcr.io/distroless/static-debian12@sha256:52dcfbabb7457ea47c82f6e13af8c8a4a1d9f7b0145142b3ecab20f2b888411d
COPY bin/ani-observability-service /usr/local/bin/ani-observability-service
COPY configs/config.yaml /etc/ani-observability/configs/config.yaml
COPY LICENSE THIRD_PARTY_NOTICES.go-kratos-layout.txt /usr/share/doc/ani-observability-service/
USER 65532:65532
EXPOSE 19090 19091
ENTRYPOINT ["/usr/local/bin/ani-observability-service"]
CMD ["-conf", "/etc/ani-observability/configs"]
